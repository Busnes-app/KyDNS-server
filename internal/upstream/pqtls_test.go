package upstream

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/Busnes-app/ky-primitives/pqtls"
	"github.com/miekg/dns"
)

// Outbound DoT and DoH connections leave CurvePreferences unset so Go's default
// TLS 1.3 handshake offers and negotiates X25519MLKEM768 with capable peers,
// both normally and under the strong-crypto profile (GODEBUG=fips140=on).
func TestOutboundTLSNegotiatesHybrid(t *testing.T) {
	t.Run("tlsConfig", func(t *testing.T) {
		cert, pool := testCert(t)
		ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		done := make(chan struct{})
		go func() {
			defer close(done)
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			_ = c.(*tls.Conn).Handshake()
		}()

		conn, err := tls.Dial("tcp", ln.Addr().String(), tlsConfig(dotSpec(ln.Addr().String(), pool)))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		pqtls.RequireHybrid(t, conn.ConnectionState())
		<-done
	})

	t.Run("DoT", func(t *testing.T) {
		cert, pool := testCert(t)
		states := make(chan tls.ConnectionState, 1)
		addr, _ := dotServer(t, cert, 0, func(w dns.ResponseWriter, r *dns.Msg) {
			if cs, ok := w.(dns.ConnectionStater); ok {
				if st := cs.ConnectionState(); st != nil {
					states <- *st
				}
			}
			_ = w.WriteMsg(answer(r))
		})

		u, err := New(dotSpec(addr, pool), 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer u.Close()

		resp, err := u.Exchange(context.Background(), query())
		wantAnswer(t, resp, err)

		select {
		case st := <-states:
			pqtls.RequireHybrid(t, st)
		default:
			t.Fatal("DoT server observed no TLS connection state")
		}
	})

	t.Run("DoH", func(t *testing.T) {
		cert, pool := testCert(t)
		states := make(chan tls.ConnectionState, 1)
		seen := new(dns.Msg)
		echo := dohEcho(t, seen)
		srv := dohServer(t, cert, func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil {
				states <- *r.TLS
			}
			echo(w, r)
		})

		u, err := New(dohSpec(t, srv, pool), 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer u.Close()

		resp, err := u.Exchange(context.Background(), query())
		wantAnswer(t, resp, err)

		select {
		case st := <-states:
			pqtls.RequireHybrid(t, st)
		default:
			t.Fatal("DoH server observed no TLS connection state")
		}
	})
}
