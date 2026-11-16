package dnssec

import (
	"encoding/base64"
	"io"
	"log/slog"

	"github.com/miekg/dns"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func encode(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func mustRR(s string) dns.RR {
	rr, err := dns.NewRR(s)
	if err != nil {
		panic(err)
	}
	return rr
}
