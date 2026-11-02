package web

import "testing"

// Every page names the scripts and stylesheets of the running build, so browsers revalidate
// it: after an upgrade a page shows the new build's markup. Static files keep their headers.
func TestPagesRevalidateAndStaticFilesKeepCaching(t *testing.T) {
	h, srv := newWeb(t)
	if got := get(t, h, "/setup", nil).Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("GET /setup: Cache-Control %q, want no-cache", got)
	}
	setupAndLogin(t, h)
	for _, path := range []string{"/login", "/settings/appearance"} {
		cookie := loginCookie(t, h)
		if path == "/login" {
			cookie = nil
		}
		if got := get(t, h, path, cookie).Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("GET %s: Cache-Control %q, want no-cache", path, got)
		}
	}
	if got := static(t, srv, "/static/app.css").Header().Get("Cache-Control"); got != "" {
		t.Errorf("GET /static/app.css: Cache-Control %q, want none (unchanged)", got)
	}
}
