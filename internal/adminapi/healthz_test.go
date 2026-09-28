package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestHealthzPublicAliasesShareCachedCheck(t *testing.T) {
	calls := 0
	h := NewAPI(nil, nil, nil).WithHealthCheck(func(context.Context) error {
		calls++
		return nil
	}).Handler()
	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		rec := do(t, h, http.MethodGet, path, "", "")
		var got struct{ Schema, Service, Status string }
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusOK || got.Schema != "ky.health/1" || got.Service != "kydns" || got.Status != "ok" {
			t.Fatalf("GET %s = %d %+v", path, rec.Code, got)
		}
	}
	if calls != 1 {
		t.Fatalf("health check ran %d times across aliases, want 1", calls)
	}
}

func TestHealthzHidesDatabaseFailure(t *testing.T) {
	const secret = "sqlite path /private/kydns.db"
	h := NewAPI(nil, nil, nil).WithHealthCheck(func(context.Context) error {
		return errors.New(secret)
	}).Handler()
	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		rec := do(t, h, http.MethodGet, path, "", "")
		var got struct{ Schema, Service, Status string }
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusServiceUnavailable || got.Schema != "ky.health/1" || got.Service != "kydns" || got.Status != "down" {
			t.Fatalf("GET %s = %d %+v", path, rec.Code, got)
		}
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("GET %s exposed database error: %s", path, rec.Body)
		}
	}
}
