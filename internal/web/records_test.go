package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestRecordCreateAndList(t *testing.T) {
	h, _, c, csrf := loggedIn(t)
	rec := postForm(t, h, "/records/new", url.Values{
		"name": {"printer.home.arpa."}, "type": {"A"}, "value": {"192.168.1.50"},
		"csrf_token": {csrf},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body)
	}
	body := page(t, h, "/records", c)
	for _, want := range []string{"printer.home.arpa.", "192.168.1.50", "A"} {
		if !strings.Contains(body, want) {
			t.Errorf("records page missing %q", want)
		}
	}
}

func TestRecordTypeDropdownIsLimitedToV1Types(t *testing.T) {
	h, _, c, _ := loggedIn(t)
	body := page(t, h, "/records", c)
	for _, want := range []string{`value="A"`, `value="AAAA"`, `value="CNAME"`, `value="PTR"`} {
		if !strings.Contains(body, want) {
			t.Errorf("type dropdown missing %s", want)
		}
	}
	for _, unwanted := range []string{`value="TXT"`, `value="MX"`, `value="SRV"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("type dropdown offers unsupported %s", unwanted)
		}
	}
}

func TestRecordRejectsUnsupportedType(t *testing.T) {
	h, _, c, csrf := loggedIn(t)
	rec := postForm(t, h, "/records/new", url.Values{
		"name": {"x.home.arpa."}, "type": {"TXT"}, "value": {"hello"}, "csrf_token": {csrf},
	}, c)
	if rec.Code == http.StatusSeeOther {
		t.Error("TXT record accepted, want rejection")
	}
}

func TestRecordRejectsOutOfZoneName(t *testing.T) {
	h, _, c, csrf := loggedIn(t)
	rec := postForm(t, h, "/records/new", url.Values{
		"name": {"evil.example.com."}, "type": {"A"}, "value": {"192.168.1.1"}, "csrf_token": {csrf},
	}, c)
	if rec.Code == http.StatusSeeOther {
		t.Error("out-of-zone record accepted, want rejection")
	}
}

// A rejected record reopens the add dialog with what was typed and the reason
// inside it, so the operator fixes one field instead of retyping the form.
func TestRecordRejectionReopensTheAddDialog(t *testing.T) {
	h, _, c, csrf := loggedIn(t)
	rec := postForm(t, h, "/records/new", url.Values{
		"name": {"typed.home.arpa."}, "type": {"AAAA"}, "value": {"not-an-address"}, "csrf_token": {csrf},
	}, c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid record = %d, want 400", rec.Code)
	}
	body := rec.Body.String()
	_, dialog, _ := strings.Cut(body, `<dialog class="form-dialog" id="add-record"`)
	for _, want := range []string{` data-open>`, `<p class="error" role="alert">`, `value="typed.home.arpa."`, `value="not-an-address"`, `<option value="AAAA" selected>`} {
		if !strings.Contains(dialog, want) {
			t.Errorf("add dialog missing %s", want)
		}
	}
	if clean := page(t, h, "/records", c); strings.Contains(clean, "data-open") {
		t.Error("the add dialog opens on a plain visit")
	}
}

// A PTR must live under in-addr.arpa, not merely under .arpa — the private
// domain home.arpa is itself under .arpa.
func TestRecordRejectsPTROutsideReverseNamespace(t *testing.T) {
	h, _, c, csrf := loggedIn(t)
	rec := postForm(t, h, "/records/new", url.Values{
		"name": {"x.home.arpa."}, "type": {"PTR"}, "value": {"y.home.arpa."}, "csrf_token": {csrf},
	}, c)
	if rec.Code == http.StatusSeeOther {
		t.Error("PTR in the forward zone accepted, want rejection")
	}
}

func TestRecordWithViewTag(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	addView(t, srv, "tailnet", "100.64.0.0/10")
	rec := postForm(t, h, "/records/new", url.Values{
		"name": {"printer.home.arpa."}, "type": {"A"}, "value": {"100.64.0.9"},
		"view": {"tailnet"}, "csrf_token": {csrf},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(page(t, h, "/records", c), "tailnet") {
		t.Error("view tag not shown on the records page")
	}
}

func TestRecordDelete(t *testing.T) {
	h, srv, c, csrf := loggedIn(t)
	postForm(t, h, "/records/new", url.Values{
		"name": {"temp.home.arpa."}, "type": {"A"}, "value": {"192.168.1.60"}, "csrf_token": {csrf},
	}, c)
	recs, err := srv.o.Registry.Records()
	if err != nil || len(recs) != 1 {
		t.Fatalf("Records() = %v, %v", recs, err)
	}
	if got := postForm(t, h, "/records/delete", url.Values{
		"id": {itoa(recs[0].ID)}, "csrf_token": {csrf},
	}, c); got.Code != http.StatusSeeOther {
		t.Fatalf("delete = %d: %s", got.Code, got.Body)
	}
	if strings.Contains(page(t, h, "/records", c), "temp.home.arpa.") {
		t.Error("deleted record still listed")
	}
}

// Services and Blacklists reopen their add dialogs the same way Records does.
func TestAddDialogsReopenWithTypedValues(t *testing.T) {
	h, _, c, csrf := loggedIn(t)
	for _, tc := range []struct {
		path, dialog string
		form         url.Values
		want         []string
	}{
		{"/services/new", "add-service", url.Values{"name": {"typed"}, "address": {"not-an-address"}, "aliases": {"web"}},
			[]string{`value="typed"`, `value="not-an-address"`, `value="web"`}},
		{"/blacklists/lists/new", "add-list", url.Values{"name": {"typed-list"}, "url": {"http://plain.example/hosts"}, "format": {"hosts"}, "interval": {"3600"}},
			[]string{`value="typed-list"`, `value="http://plain.example/hosts"`, `<option value="hosts" selected>`, `value="3600"`}},
	} {
		tc.form.Set("csrf_token", csrf)
		rec := postForm(t, h, tc.path, tc.form, c)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", tc.path, rec.Code)
		}
		_, dialog, _ := strings.Cut(rec.Body.String(), `<dialog class="form-dialog" id="`+tc.dialog+`"`)
		for _, want := range append(tc.want, ` data-open>`, `<p class="error" role="alert">`) {
			if !strings.Contains(dialog, want) {
				t.Errorf("%s dialog missing %s", tc.dialog, want)
			}
		}
	}
}

var elementID = regexp.MustCompile(` id="([^"]+)"`)

// An add form moved into a dialog must leave no copy behind on the page: a
// leftover half-form duplicated the Services field IDs once.
func TestAddPagesHaveUniqueIDs(t *testing.T) {
	h, _, c, _ := loggedIn(t)
	for _, path := range []string{"/records", "/services", "/blacklists"} {
		seen := map[string]bool{}
		for _, m := range elementID.FindAllStringSubmatch(page(t, h, path, c), -1) {
			if seen[m[1]] {
				t.Errorf("%s repeats id %q", path, m[1])
			}
			seen[m[1]] = true
		}
	}
}
