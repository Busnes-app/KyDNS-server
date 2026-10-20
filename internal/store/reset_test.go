package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ResetAdmin is the recovery path: new password, SSO unlinked and off, and
// every earlier session void, all at once.
func TestResetAdminUnlinksSSOAndVoidsSessions(t *testing.T) {
	s := open(t)
	if err := s.SetAdminPassword("$argon2id$old"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSSOSettings(SSOSettings{Enabled: true, IssuerURL: "https://idp.example", ClientID: "kydns"}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkAdminSSO("sub-1", "bob", "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutToken(Token{Label: "old-admin", Hash: "old-token-hash"}); err != nil {
		t.Fatal(err)
	}
	at := time.UnixMilli(1_700_000_000_123)
	if err := s.ResetAdmin("$argon2id$new", at); err != nil {
		t.Fatal(err)
	}
	if tokens, err := s.Tokens(); err != nil || len(tokens) != 0 {
		t.Fatalf("tokens after reset = %v, %v; want none", tokens, err)
	}
	ident, err := s.AdminIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if ident.PasswordHash != "$argon2id$new" || ident.SSOSub != "" {
		t.Errorf("after reset: hash=%q sub=%q", ident.PasswordHash, ident.SSOSub)
	}
	if sso, _ := s.SSOSettings(); sso.Enabled || sso.IssuerURL != "https://idp.example" {
		t.Errorf("after reset: SSO = %+v, want off with its config kept", sso)
	}
	if nb, err := s.SessionsNotBefore(); err != nil || !nb.Equal(at) {
		t.Errorf("SessionsNotBefore() = %v, %v; want %v", nb, err, at)
	}
}

func TestResetAdminCreatesAMissingAdmin(t *testing.T) {
	s := open(t)
	if err := s.ResetAdmin("$argon2id$new", time.Now()); err != nil {
		t.Fatal(err)
	}
	if h, err := s.AdminHash(); err != nil || h != "$argon2id$new" {
		t.Fatalf("AdminHash() = %q, %v", h, err)
	}
}

// A database from before sessions_not_before gains the column on Open.
func TestOpenAddsSessionsNotBefore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kydns.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdminPassword("$argon2id$x"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE admin DROP COLUMN sessions_not_before`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, migrationIndex(t, "sessions_not_before"))); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatalf("Open() on a pre-migration database: %v", err)
	}
	defer s.Close()
	if nb, err := s.SessionsNotBefore(); err != nil || nb.UnixMilli() != 0 {
		t.Fatalf("SessionsNotBefore() = %v, %v; want the zero instant", nb, err)
	}
}

func TestResetGenerationAlwaysAdvances(t *testing.T) {
	s := open(t)
	at := time.UnixMilli(1700000000123)
	for i, when := range []time.Time{at, at, at.Add(-time.Hour)} {
		if err := s.ResetAdmin("hash", when); err != nil {
			t.Fatal(err)
		}
		generation, err := s.SessionsNotBefore()
		if err != nil || generation.UnixMilli() != at.UnixMilli()+int64(i) {
			t.Fatalf("reset %d generation = %v, %v", i, generation, err)
		}
	}
}

// migrationIndex is the position of the one migration mentioning column, so a
// test can roll user_version back to just before it.
func migrationIndex(t *testing.T, column string) int {
	t.Helper()
	for i, m := range migrations {
		if strings.Contains(m, column) {
			return i
		}
	}
	t.Fatalf("no migration mentions %s", column)
	return 0
}

// Agent tokens live in the tokens table, so the admin reset revokes them with
// the admin tokens.
func TestResetAdminRevokesAgentTokens(t *testing.T) {
	s := open(t)
	if _, err := s.PutToken(Token{Label: "agent", Hash: "h1", Scope: ScopeMCPWrite, QueryLog: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetAdmin("$argon2id$x", time.Now()); err != nil {
		t.Fatal(err)
	}
	if toks, err := s.Tokens(); err != nil || len(toks) != 0 {
		t.Fatalf("after reset: %v %v", toks, err)
	}
}

// A database from before token scopes gains them, and its tokens stay admin tokens.
func TestOpenAddsTokenScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kydns.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutToken(Token{Label: "cli", Hash: "h1"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`ALTER TABLE tokens DROP COLUMN scope`, `ALTER TABLE tokens DROP COLUMN query_log`,
		fmt.Sprintf(`PRAGMA user_version = %d`, migrationIndex(t, "query_log"))} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatalf("Open() on a pre-scope database: %v", err)
	}
	defer s.Close()
	toks, err := s.Tokens()
	if err != nil || len(toks) != 1 || toks[0].Scope != ScopeAdmin || toks[0].QueryLog {
		t.Fatalf("tokens after migration: %+v %v", toks, err)
	}
	if err := s.SetTokenAccess(toks[0].ID, ScopeMCPRead, false); err == nil {
		t.Error("an admin token was turned into an agent's")
	}
}
