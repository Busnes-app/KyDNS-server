package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func static(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.StaticHandler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestStaticServesStylesheet(t *testing.T) {
	_, srv := newWeb(t)
	rec := static(t, srv, "/static/app.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/app.css = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--accent") {
		t.Error("app.css does not build on the existing design tokens")
	}
	styles := static(t, srv, "/static/styles.css")
	if styles.Code != http.StatusOK {
		t.Fatalf("GET /static/styles.css = %d", styles.Code)
	}
	for _, want := range []string{
		`.nav-links a[aria-current="page"]`,
		`border-bottom-color: var(--accent)`,
		`border-left-color: var(--accent)`,
	} {
		if !strings.Contains(styles.Body.String(), want) {
			t.Errorf("styles.css missing selected navigation treatment %q", want)
		}
	}
}

// The theme script runs before first paint from <head>, so every screen,
// including the ones before a session exists, gets the stored palette.
func TestThemeScriptIsLinkedAndEmbedded(t *testing.T) {
	_, srv := newWeb(t)
	rec := static(t, srv, "/static/theme.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/theme.js = %d, want the script to be embedded", rec.Code)
	}
	for _, want := range []string{`"kydns-theme"`, `window.kyThemes`, `"DOMContentLoaded"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("theme.js missing %s", want)
		}
	}
	if catalog := static(t, srv, "/static/ky-ui/themes-browser.js").Body.String(); !strings.Contains(catalog, `'Polished Ky'`) {
		t.Error("the shared theme catalog is not embedded")
	}
	h, _ := newWeb(t)
	body := page(t, h, "/setup", nil)
	// Classic, in order, in <head>: a module would be deferred past first paint.
	head, _, _ := strings.Cut(body, "</head>")
	if !strings.Contains(head, "<script src=\"/static/ky-ui/themes-browser.js\"></script>\n<script src=\"/static/theme.js\"></script>") {
		t.Error("base.html must load themes-browser.js then theme.js as classic scripts in <head>")
	}
	if strings.Contains(head, `type="module"`) {
		t.Error("base.html defers the theme script; saved themes would apply after first paint")
	}
	setupAndLogin(t, h)
	if anon := get(t, h, "/appearance", nil); anon.Code != http.StatusSeeOther {
		t.Errorf("GET /appearance without a session = %d, want a redirect to sign in", anon.Code)
	}
	body = get(t, h, "/appearance", loginCookie(t, h)).Body.String()
	for _, want := range []string{`<h1 class="ky-page-title">Appearance</h1>`, `data-theme-picker`, `href="/static/ky-ui/appearance.css"`, `href="/static/ky-ui/appearance-themes.css"`, `href="/appearance" aria-current="page"`} {
		if !strings.Contains(body, want) {
			t.Errorf("appearance page missing %s", want)
		}
	}
	if strings.Contains(rec.Body.String(), "innerHTML") {
		t.Error("theme.js builds the picker from markup strings; use DOM calls")
	}
}

// A page that links a favicon the binary does not embed shows a broken icon in
// every tab, so the link and the file are checked together.
func TestFaviconIsLinkedAndEmbedded(t *testing.T) {
	_, srv := newWeb(t)

	rec := static(t, srv, "/static/favicon.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/favicon.svg = %d, want the icon to be embedded", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data:image/png;base64,") {
		t.Error("favicon does not embed the shared product stamp")
	}

	// /setup renders through base.html with no session, which is where the
	// link lives, so every screen inherits it.
	h, _ := newWeb(t)
	if body := page(t, h, "/setup", nil); !strings.Contains(body, `href="/static/favicon.svg"`) {
		t.Error("base.html does not link the favicon")
	}
}

// The marketing stylesheet shipped with @font-face paths pointing at
// ../assets/fonts, but the fonts live elsewhere. The embedded copy must be
// fixed, and the referenced files must actually be present.
func TestEmbeddedFontPathsResolve(t *testing.T) {
	_, srv := newWeb(t)

	rec := static(t, srv, "/static/styles.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/styles.css = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "../assets/fonts") {
		t.Error("embedded styles.css still points at ../assets/fonts")
	}
	for _, want := range []string{
		"/static/fonts/Space_Grotesk/SpaceGrotesk-VariableFont_wght.ttf",
		"/static/fonts/IBM_Plex_Mono/IBMPlexMono-Regular.ttf",
		"/static/fonts/IBM_Plex_Mono/IBMPlexMono-Medium.ttf",
		"/static/fonts/IBM_Plex_Mono/IBMPlexMono-SemiBold.ttf",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("styles.css does not reference %s", want)
		}
		if got := static(t, srv, want); got.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want the font to be embedded", want, got.Code)
		}
	}
}

func TestRenderIncludesNavAndAssets(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)

	rec := get(t, h, "/", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="/services"`, `href="/records"`, `href="/settings"`,
		"/static/app.css", "/static/styles.css",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}

// The operator should always be able to see what they are running and on what
// terms, so the sidebar licence line, the About dialog and the embedded licence
// text are checked together.
func TestNavFooterOpensLicense(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)

	year := strconv.Itoa(time.Now().Year())
	body := get(t, h, "/", c).Body.String()
	for _, want := range []string{
		// The shared licence line opens the native About dialog without script.
		`commandfor="ky-about" command="show-modal">&copy; ` + year + ` &middot; MIT License</button>`,
		`<dialog class="ky-about" id="ky-about" aria-label="About KyDNS"`,
		`KyDNS <span class="ky-about-version">v` + Version + `</span>`,
		"Developed by Busnes.app",
		"&copy; " + year + " &middot; Licensed under the MIT License",
		// The licence itself, verbatim, rather than a frame of another page.
		`aria-label="MIT License">` + template.HTMLEscapeString(licenseText) + `</textarea>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
	// The version lives in the About pill only, as in the other Ky apps.
	if n := strings.Count(body, "v"+Version); n != 1 {
		t.Errorf("version appears %d times, want once (the About pill)", n)
	}

	rec := static(t, srv, "/static/mit.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/mit.txt = %d, want the license to be embedded", rec.Code)
	}
	// The served copy is what the operator reads, so it must stay identical to
	// the license the project ships under.
	want, err := os.ReadFile("../../LICENSE.txt")
	if err != nil {
		t.Fatalf("read LICENSE.txt: %v", err)
	}
	if rec.Body.String() != string(want) || licenseText != string(want) {
		t.Error("embedded mit.txt has drifted from LICENSE.txt")
	}
}

// Every full page carries the session's CSRF token, so its forms can post.
func TestRenderIncludesCSRFToken(t *testing.T) {
	h, srv := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	sess, ok := srv.o.Sessions.Get(c.Value)
	if !ok {
		t.Fatal("no session")
	}
	if !strings.Contains(get(t, h, "/", c).Body.String(), sess.CSRF) {
		t.Error("rendered page does not carry the session CSRF token")
	}
}

// The active nav item is marked, so the operator can tell where they are.
func TestRenderMarksCurrentNav(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	body := get(t, h, "/services", c).Body.String()
	if !strings.Contains(body, `href="/services" aria-current="page"`) {
		t.Errorf("current nav item not marked:\n%s", body)
	}
}

// Templates must escape interpolated values. Operator input reaches the page
// unfiltered otherwise.
func TestTemplatesEscapeHTML(t *testing.T) {
	_, srv := newWeb(t)
	rec := httptest.NewRecorder()
	srv.renderBare(rec, "login.html", map[string]any{
		"Title": "Sign in", "Error": `<script>alert(1)</script>`,
	})
	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Errorf("template emitted unescaped HTML:\n%s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "&lt;script&gt;") {
		t.Errorf("expected escaped output:\n%s", rec.Body)
	}
}

// An unknown page name is a 500, not a blank 200 that looks like success.
func TestRenderUnknownPageFails(t *testing.T) {
	h, _ := newWeb(t)
	setupAndLogin(t, h)
	c := loginCookie(t, h)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(c)
	_, srv := newWeb(t)
	srv.render(rec, req, "nope.html", map[string]any{"Title": "x"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("render of an unknown page = %d, want 500", rec.Code)
	}
}
