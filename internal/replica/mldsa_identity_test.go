package replica

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewIdentityUsesMLDSA65AndInteroperatesWithLegacyEd25519(t *testing.T) {
	// Fresh node generates ML-DSA-65 PKCS #8 PEM node_key.
	mldsaDir := t.TempDir()
	mldsaID, err := LoadOrCreateIdentity(mldsaDir)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity(fresh): %v", err)
	}
	pub, ok := mldsaID.PublicKey.(*mldsa.PublicKey)
	if !ok {
		t.Fatalf("fresh identity public key = %T, want *mldsa.PublicKey", mldsaID.PublicKey)
	}
	if pub.Parameters() != mldsa.MLDSA65() {
		t.Fatalf("fresh identity ML-DSA params = %v, want ML-DSA-65", pub.Parameters())
	}
	raw, err := os.ReadFile(filepath.Join(mldsaDir, keyFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateNodeKeyBytes(raw); err != nil {
		t.Fatalf("ValidateNodeKeyBytes(ML-DSA-65): %v", err)
	}

	// Legacy node has 32-byte raw Ed25519 seed on disk.
	legacyDir := t.TempDir()
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, keyFile), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNodeKeyBytes(seed); err != nil {
		t.Fatalf("ValidateNodeKeyBytes(legacy Ed25519): %v", err)
	}
	legacyID, err := LoadOrCreateIdentity(legacyDir)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity(legacy): %v", err)
	}
	if _, ok := legacyID.PublicKey.(ed25519.PublicKey); !ok {
		t.Fatalf("legacy identity public key = %T, want ed25519.PublicKey", legacyID.PublicKey)
	}

	// Mixed ML-DSA-65 server <-> legacy Ed25519 client and reverse both complete
	// pinned X25519MLKEM768 mTLS handshakes.
	for _, tc := range []struct {
		name   string
		server *Identity
		client *Identity
	}{
		{"mldsa65_server_and_mldsa65_client", mldsaID, mldsaID},
		{"mldsa65_server_and_legacy_ed25519_client", mldsaID, legacyID},
		{"legacy_ed25519_server_and_mldsa65_client", legacyID, mldsaID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srvCfg, err := pinnedTLSConfig(tc.server, func(fp string) bool { return fp == tc.client.NodeID })
			if err != nil {
				t.Fatalf("server pinnedTLSConfig: %v", err)
			}
			cliCfg, err := pinnedTLSConfig(tc.client, func(fp string) bool { return fp == tc.server.NodeID })
			if err != nil {
				t.Fatalf("client pinnedTLSConfig: %v", err)
			}

			ln, err := tls.Listen("tcp", "127.0.0.1:0", srvCfg)
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()

			srvErr := make(chan error, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					srvErr <- err
					return
				}
				defer conn.Close()
				srvErr <- conn.(*tls.Conn).Handshake()
			}()

			conn, err := tls.Dial("tcp", ln.Addr().String(), cliCfg)
			if err != nil {
				t.Fatalf("tls.Dial: %v", err)
			}
			if conn.ConnectionState().CurveID != tls.X25519MLKEM768 {
				t.Fatalf("CurveID = %v, want X25519MLKEM768", conn.ConnectionState().CurveID)
			}
			_ = conn.Close()
			if err := <-srvErr; err != nil {
				t.Fatalf("server handshake: %v", err)
			}
		})
	}
}

func TestRejectsUnsupportedNodeKeysAndPeerCertificates(t *testing.T) {
	// ML-DSA-44 key is rejected on load and in leafFingerprint.
	ml44, err := mldsa.GenerateKey(mldsa.MLDSA44())
	if err != nil {
		t.Fatal(err)
	}
	ml44DER, err := x509.MarshalPKCS8PrivateKey(ml44)
	if err != nil {
		t.Fatal(err)
	}
	ml44PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ml44DER})
	if err := ValidateNodeKeyBytes(ml44PEM); err == nil {
		t.Fatal("ValidateNodeKeyBytes accepted ML-DSA-44 key, want error")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, keyFile), ml44PEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateIdentity(dir); err == nil {
		t.Fatal("LoadOrCreateIdentity accepted ML-DSA-44 key, want error")
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "bad-peer"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	ml44Cert, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, ml44.Public(), ml44)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leafFingerprint([][]byte{ml44Cert}); err == nil {
		t.Fatal("leafFingerprint accepted ML-DSA-44 certificate, want error")
	}

	// ECDSA P-256 certificate is rejected.
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecCert, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, ecKey.Public(), ecKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leafFingerprint([][]byte{ecCert}); err == nil {
		t.Fatal("leafFingerprint accepted ECDSA certificate, want error")
	}
}
