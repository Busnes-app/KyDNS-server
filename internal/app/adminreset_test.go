package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/auth"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// seeded writes a config and a database with an existing admin password.
func seeded(t *testing.T, existing string) (cfgPath, dir string) {
	t.Helper()
	dir = t.TempDir()
	cfgPath = filepath.Join(dir, "kydns.yaml")
	if err := os.WriteFile(cfgPath, []byte("data_dir: "+dir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if existing != "" {
		hash, err := auth.HashPassword(existing)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetAdminPassword(hash); err != nil {
			t.Fatal(err)
		}
	}
	return cfgPath, dir
}

// answers returns a PasswordReader that replies with each value in turn.
func answers(vals ...string) PasswordReader {
	i := 0
	return func(string) (string, error) {
		v := vals[i]
		if i < len(vals)-1 {
			i++
		}
		return v, nil
	}
}

func adminHash(t *testing.T, dir string) string {
	t.Helper()
	st, err := store.Open(filepath.Join(dir, "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h, err := st.AdminHash()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// The whole point: a forgotten password is recoverable.
func TestResetAdminPasswordReplacesIt(t *testing.T) {
	cfg, dir := seeded(t, "the-old-password")
	var out bytes.Buffer
	if err := ResetAdminPassword(cfg, answers("a-brand-new-password", "a-brand-new-password"), &out); err != nil {
		t.Fatal(err)
	}
	hash := adminHash(t, dir)
	if !verified(hash, "a-brand-new-password") {
		t.Error("the new password does not verify")
	}
	if verified(hash, "the-old-password") {
		t.Error("the old password still works")
	}
	if !strings.Contains(out.String(), "session has ended") {
		t.Error("output does not say that signed-in sessions ended")
	}
}

// A reset is how an operator recovers from a compromise, so it must also cut
// off a linked SSO identity and every live session.
func TestResetUnlinksSSOAndVoidsSessions(t *testing.T) {
	cfg, dir := seeded(t, "the-old-password")
	st, err := store.Open(filepath.Join(dir, "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetSSOSettings(store.SSOSettings{Enabled: true, IssuerURL: "https://idp.example", ClientID: "kydns"})
	if err := st.LinkAdminSSO("attacker", "mallory", ""); err != nil {
		t.Fatal(err)
	}
	st.Close()
	before := time.Now()

	if err := ResetAdminPassword(cfg, answers("a-brand-new-password", "a-brand-new-password"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(filepath.Join(dir, "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ident, _ := st.AdminIdentity()
	sso, _ := st.SSOSettings()
	nb, _ := st.SessionsNotBefore()
	if ident.SSOSub != "" || sso.Enabled || nb.Before(before) {
		t.Fatalf("after reset: sub=%q sso=%v notBefore=%v (reset began %v)", ident.SSOSub, sso.Enabled, nb, before)
	}
}

// The database stays closed while the prompt waits: Ctrl-C there skips every
// defer, and an open store would leave WAL and SHM files owned by whoever ran
// the command, typically root.
func TestResetPromptsBeforeOpeningTheDatabase(t *testing.T) {
	cfg, dir := seeded(t, "the-old-password")
	var open []string
	abandoned := func(string) (string, error) {
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, err := os.Stat(filepath.Join(dir, "kydns.db"+suffix)); err == nil {
				open = append(open, suffix)
			}
		}
		return "", errors.New("interrupted")
	}
	if err := ResetAdminPassword(cfg, abandoned, &bytes.Buffer{}); err == nil {
		t.Fatal("an interrupted prompt reported success")
	}
	if len(open) > 0 {
		t.Fatalf("kydns.db%v existed while the prompt was waiting", open)
	}
}

// Recovery must also work when setup was never completed.
func TestResetWorksWithNoExistingAdmin(t *testing.T) {
	cfg, dir := seeded(t, "")
	if err := ResetAdminPassword(cfg, answers("a-brand-new-password", "a-brand-new-password"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !verified(adminHash(t, dir), "a-brand-new-password") {
		t.Error("password was not set")
	}
}

func TestResetRejectsMismatch(t *testing.T) {
	cfg, dir := seeded(t, "the-old-password")
	err := ResetAdminPassword(cfg, answers("a-brand-new-password", "a-different-password"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("mismatched passwords were accepted")
	}
	if !verified(adminHash(t, dir), "the-old-password") {
		t.Error("a failed reset changed the stored password")
	}
}

func TestResetRejectsShortPassword(t *testing.T) {
	cfg, dir := seeded(t, "the-old-password")
	if err := ResetAdminPassword(cfg, answers("short", "short"), &bytes.Buffer{}); err == nil {
		t.Fatal("a short password was accepted")
	}
	if !verified(adminHash(t, dir), "the-old-password") {
		t.Error("a failed reset changed the stored password")
	}
}

// A missing database is a clear message, not a confusing SQLite error or a
// silently created empty one.
func TestResetWithoutDatabaseExplains(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "kydns.yaml")
	if err := os.WriteFile(cfg, []byte("data_dir: "+dir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := ResetAdminPassword(cfg, answers("a-brand-new-password", "a-brand-new-password"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("reset succeeded with no database")
	}
	if !strings.Contains(err.Error(), "no database") {
		t.Errorf("error = %v, want it to name the missing database", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "kydns.db")); statErr == nil {
		t.Error("a database was created by a failed reset")
	}
}

func TestResetRejectsBadConfigPath(t *testing.T) {
	if err := ResetAdminPassword(filepath.Join(t.TempDir(), "absent.yaml"),
		answers("a-brand-new-password"), &bytes.Buffer{}); err == nil {
		t.Error("reset succeeded with no config file")
	}
}

// After a reset the operator can sign in through the web UI with the new
// password: the recovery path actually restores access.
func TestResetRestoresWebLogin(t *testing.T) {
	cfg, dir := seeded(t, "the-old-password")
	if err := ResetAdminPassword(cfg, answers("a-brand-new-password", "a-brand-new-password"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	hash, err := st.AdminHash()
	if err != nil {
		t.Fatal(err)
	}
	if !verified(hash, "a-brand-new-password") {
		t.Error("the stored hash does not accept the new password")
	}
}

// verified is VerifyPassword for tests, where the checker is never busy.
func verified(encoded, plaintext string) bool {
	ok, err := auth.VerifyPassword(encoded, plaintext)
	return err == nil && ok
}
