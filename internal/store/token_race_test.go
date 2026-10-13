package store_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

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
