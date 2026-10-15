package app

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Busnes-app/kydns-server/internal/store"
)

// Under systemd the journal is readable beyond root, so the token stays in
// its 0600 file; a container's log, with no shell to read the file, keeps it.
func TestSetupTokenStaysOutOfTheJournal(t *testing.T) {
	for _, tc := range []struct {
		name, journal string
		logged        bool
	}{
		{"systemd", "8:12345", false},
		{"container", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JOURNAL_STREAM", tc.journal)
			dir := t.TempDir()
			st, err := store.Open(filepath.Join(dir, "kydns.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			var buf bytes.Buffer
			token, err := ensureSetupToken(st, dir, slog.New(slog.NewTextHandler(&buf, nil)))
			if err != nil || token == "" {
				t.Fatalf("ensureSetupToken() = %q, %v", token, err)
			}
			if got := strings.Contains(buf.String(), token); got != tc.logged {
				t.Fatalf("token in log = %v, want %v: %s", got, tc.logged, buf.String())
			}
			if !strings.Contains(buf.String(), "setup-token") {
				t.Fatalf("log does not name the token file: %s", buf.String())
			}
		})
	}
}

func TestLoopbackListen(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8053": true, "[::1]:8053": true, "localhost:8053": true,
		"0.0.0.0:8053": false, ":8053": false, "192.168.1.5:8053": false, "nonsense": false,
	} {
		if got := loopbackListen(addr); got != want {
			t.Errorf("loopbackListen(%q) = %v, want %v", addr, got, want)
		}
	}
}
