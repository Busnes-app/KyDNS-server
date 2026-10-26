package upstream

import (
	"bufio"
	"context"
	"crypto/fips140"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type tlsPeerFixture struct {
	Chain [][]byte
	Key   []byte
}

// The peer runs outside the selected process profile. Otherwise its own TLS
// signing policy could refuse a weak key before the client policy is exercised.
func TestStrongTLSPeerHelper(t *testing.T) {
	file := os.Getenv("KYDNS_TEST_TLS_PEER")
	if file == "" {
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var fixture tlsPeerFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	key, err := x509.ParsePKCS8PrivateKey(fixture.Key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: fixture.Chain, PrivateKey: key}
	addr, _ := dotServer(t, cert, 0, func(w dns.ResponseWriter, r *dns.Msg) { _ = w.WriteMsg(answer(r)) })
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wire, err := io.ReadAll(io.LimitReader(r.Body, 65536))
		if err != nil {
			http.Error(w, "read", 400)
			return
		}
		var q dns.Msg
		if err := q.Unpack(wire); err != nil {
			http.Error(w, "query", 400)
			return
		}
		body, err := answer(&q).Pack()
		if err != nil {
			http.Error(w, "answer", 500)
			return
		}
		w.Header().Set("Content-Type", dohMediaType)
		_, _ = w.Write(body)
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	defer srv.Close()
	fmt.Println(addr)
	fmt.Println(srv.URL)
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestStrongTLSUpstreams(t *testing.T) {
	for _, tc := range []struct {
		name               string
		leafBits, rootBits int
	}{
		{"strong", 2048, 2048}, {"weak-leaf", 1024, 2048}, {"below-leaf-floor", 2047, 2048}, {"weak-root", 2048, 1024}, {"below-root-floor", 2048, 2047},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootKey, err := rsa.GenerateKey(rand.Reader, tc.rootBits)
			if err != nil {
				t.Fatal(err)
			}
			leafKey, err := rsa.GenerateKey(rand.Reader, tc.leafBits)
			if err != nil {
				t.Fatal(err)
			}
			root := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
			rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
			if err != nil {
				t.Fatal(err)
			}
			root, err = x509.ParseCertificate(rootDER)
			if err != nil {
				t.Fatal(err)
			}
			leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
			leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, &leafKey.PublicKey, rootKey)
			if err != nil {
				t.Fatal(err)
			}
			keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(tlsPeerFixture{Chain: [][]byte{leafDER, rootDER}, Key: keyDER})
			if err != nil {
				t.Fatal(err)
			}
			file := t.TempDir() + "/peer.json"
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestStrongTLSPeerHelper$")
			for _, v := range os.Environ() {
				if !strings.HasPrefix(v, "GODEBUG=") && !strings.HasPrefix(v, "KYDNS_TEST_TLS_PEER=") {
					cmd.Env = append(cmd.Env, v)
				}
			}
			cmd.Env = append(cmd.Env, "GODEBUG=fips140=off", "KYDNS_TEST_TLS_PEER="+file)
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = input.Close()
				if err := cmd.Wait(); err != nil {
					t.Errorf("TLS peer: %v", err)
				}
			})
			scanner := bufio.NewScanner(output)
			if !scanner.Scan() {
				t.Fatal("no DoT address")
			}
			dotAddr := scanner.Text()
			if !scanner.Scan() {
				t.Fatal("no DoH address")
			}
			dohURL := scanner.Text()
			go io.Copy(io.Discard, output)
			roots := x509.NewCertPool()
			roots.AddCert(root)
			dspec := dotSpec(dotAddr, roots)
			hspec, err := Parse(dohURL + "/dns-query")
			if err != nil {
				t.Fatal(err)
			}
			hspec.RootCAs = roots
			for _, spec := range []Spec{dspec, hspec} {
				u, err := New(spec, 2*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer u.Close()
				resp, err := u.Exchange(context.Background(), query())
				weak := tc.leafBits < 2048 || tc.rootBits < 2048
				if fips140.Enabled() && weak {
					if err == nil {
						t.Fatalf("accepted %s peer with weak %s", spec.Raw, tc.name)
					}
				} else {
					wantAnswer(t, resp, err)
				}
			}
		})
	}
}
