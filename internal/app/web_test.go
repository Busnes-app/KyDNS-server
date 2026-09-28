package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func noRedirect() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// serve must mount the web UI on the admin listener alongside the JSON API.
func TestServeMountsWebUI(t *testing.T) {
	dir := t.TempDir()
	cfg, _, adminPort := writeConfig(t, dir, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Serve(ctx, cfg, nil)

	base := fmt.Sprintf("http://127.0.0.1:%d", adminPort)
	waitForHTTP(t, base+"/api/v1/healthz")
	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		resp, err := noRedirect().Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		var got struct{ Schema, Service, Status string }
		err = json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || got.Schema != "ky.health/1" || got.Service != "kydns" || got.Status != "ok" {
			t.Fatalf("GET %s = %d %+v", path, resp.StatusCode, got)
		}
	}

	client := noRedirect()
	resp, err := client.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET / = %d, want a redirect to /setup", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/setup" {
		t.Errorf("GET / redirected to %q, want /setup", loc)
	}

	// The JSON API must still work: the two transports share one listener.
	resp2, err := client.Get(base + "/api/v1/services")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/services = %d, want 401", resp2.StatusCode)
	}
}

func TestServeServesStaticAssets(t *testing.T) {
	dir := t.TempDir()
	cfg, _, adminPort := writeConfig(t, dir, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Serve(ctx, cfg, nil)

	base := fmt.Sprintf("http://127.0.0.1:%d", adminPort)
	waitForHTTP(t, base+"/api/v1/healthz")

	resp, err := noRedirect().Get(base + "/static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /static/app.css = %d, want 200", resp.StatusCode)
	}
}

// The setup token is written where the operator can read it from a terminal,
// because no UI is reachable until they have it.
func TestSetupTokenIsWritten(t *testing.T) {
	dir := t.TempDir()
	cfg, _, adminPort := writeConfig(t, dir, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Serve(ctx, cfg, nil)
	waitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%d/api/v1/healthz", adminPort))

	if tok := waitForFile(t, filepath.Join(dir, "setup-token")); len(tok) < 16 {
		t.Errorf("setup token %q is too short", tok)
	}
}
