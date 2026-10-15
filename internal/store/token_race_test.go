package store_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Busnes-app/kydns-server/internal/registry"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// Offboarding that lands while a bound token is being created must not leave a token
// that a later reactivation brings back. The hook starts offboarding just before the
// insert and returns once it has either committed or is queued behind the creation.
func TestBoundTokenCreationOverlappingOffboardingStaysRevoked(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	b := store.InstallerBundleForTest(t)
	ctx := context.Background()
	if _, err := st.ApplyInstaller(ctx, b, false); err != nil {
		t.Fatal(err)
	}
	issuer := b.SSO.IssuerURL
	reg := registry.New(st, "home.arpa.", func() error { return nil })
	done := make(chan error, 1)
	store.SetBeforeTokenInsert(func() {
		store.SetBeforeTokenInsert(nil)
		waits := st.WaitCount()
		go func() {
			done <- st.ApplyInstallerDirectory(ctx, issuer, "user.deactivated", []byte(`{"id":"owner","active":false,"meta":{"version":"W/\"2\""}}`))
		}()
		for st.WaitCount() == waits {
			select {
			case err := <-done:
				done <- err
				return
			default:
				runtime.Gosched()
			}
		}
	})
	t.Cleanup(func() { store.SetBeforeTokenInsert(nil) })
	token, createErr := reg.CreateIdentityToken("owner", issuer, "owner")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	on := []byte(`{"id":"owner","active":true,"roles":[{"value":"kydns.admin"}],"meta":{"version":"W/\"3\""}}`)
	if err := st.ApplyInstallerDirectory(ctx, issuer, "user.updated", on); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := reg.AuthenticateTokenPrincipal(token); createErr == nil && ok {
		t.Fatal("token created during offboarding authenticates after reactivation")
	}
}

func TestTokenIssuanceAuthenticatedBeforeRecoveryIsRejected(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := registry.New(st, "home.arpa.", func() error { return nil })
	parent, err := reg.CreateToken("parent")
	if err != nil {
		t.Fatal(err)
	}
	generation, err := reg.RecoveryGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if !reg.AuthenticateToken(parent) {
		t.Fatal("parent must authenticate before recovery")
	}
	// Models a browser or API request paused after authentication.
	if err := st.ResetAdmin("new-password-hash", time.Now()); err != nil {
		t.Fatal(err)
	}
	token, err := reg.CreateIdentityTokenAtGeneration("child", "", "", generation)
	if err == nil || reg.AuthenticateToken(token) {
		t.Fatal("pre-recovery request issued a usable token")
	}
	tokens, err := st.Tokens()
	if err != nil || len(tokens) != 0 {
		t.Fatalf("tokens after recovery = %v, %v", tokens, err)
	}
	current, err := reg.RecoveryGeneration()
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := reg.CreateIdentityTokenAtGeneration("fresh", "", "", current)
	if err != nil || !reg.AuthenticateToken(fresh) {
		t.Fatalf("post-recovery issuance failed: %v", err)
	}
}

func TestTokenInsertTransactionOverlappingRecoveryLeavesNoToken(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "kydns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := registry.New(st, "home.arpa.", func() error { return nil })
	generation, err := reg.RecoveryGeneration()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	store.SetBeforeTokenInsert(func() {
		store.SetBeforeTokenInsert(nil)
		waits := st.WaitCount()
		go func() { done <- st.ResetAdmin("new-password-hash", time.Now()) }()
		for st.WaitCount() == waits {
			runtime.Gosched()
		}
	})
	defer store.SetBeforeTokenInsert(nil)
	token, createErr := reg.CreateIdentityTokenAtGeneration("overlap", "", "", generation)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if createErr == nil && reg.AuthenticateToken(token) {
		t.Fatal("token survived overlapping recovery")
	}
}
