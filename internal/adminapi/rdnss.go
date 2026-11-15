package adminapi

import (
	"errors"
	"net/http"
	"net/netip"

	"github.com/Busnes-app/kydns-server/internal/rdnss"
)

// RDNSSRunner is the IPv6 DNS advertisement's runner as the API reads it.
type RDNSSRunner interface {
	Status() (running bool, addr netip.Addr, err error)
}

// WithRDNSS attaches the runner. Optional: without one, nothing is running.
func (a *API) WithRDNSS(run RDNSSRunner) *API {
	a.rdnss = run
	return a
}

// RDNSSStatus is what the wizard card and GET /api/v1/rdnss/status show.
type RDNSSStatus struct {
	Running bool   `json:"running"`
	Address string `json:"address,omitempty"`
	Error   string `json:"error,omitempty"`
	// Warning is set when allow_query would refuse clients numbered from the
	// advertised address's /64.
	Warning string `json:"warning,omitempty"`
}

// RDNSSStatus reads the runner. Exported for the DHCP tab, so the screen and
// the JSON endpoint are one computation.
func (a *API) RDNSSStatus() RDNSSStatus {
	var out RDNSSStatus
	if a.rdnss == nil {
		return out
	}
	running, addr, err := a.rdnss.Status()
	out.Running, out.Address = running, addrText(addr)
	if err != nil {
		out.Error = err.Error()
	}
	if running && a.settings != nil {
		if v, err := a.settings.Get(); err == nil {
			out.Warning = aclWarning(v.AllowQuery, addr)
		}
	}
	return out
}

// aclWarning names the /64 clients would query from when allow_query does
// not cover it. Clients pick a source address of the advertised address's
// kind (RFC 6724), so a global address usually brings global sources.
func aclWarning(allow []string, addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	lan := netip.PrefixFrom(addr, 64).Masked()
	for _, c := range allow {
		if p, err := netip.ParsePrefix(c); err == nil && p.Bits() <= 64 && p.Contains(lan.Addr()) {
			return ""
		}
	}
	return "allow_query does not include " + lan.String() + ", so clients numbered from it are refused. " +
		"Add it under Settings, Server, or advertise a ULA (fd00::/8) address instead."
}

func (a *API) getRDNSSStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.RDNSSStatus())
}

// RDNSSSuggestion is the wizard's prefill: the address KyDNS would advertise.
type RDNSSSuggestion struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
}

// RDNSSSuggest proposes the address for one interface, saving nothing.
func RDNSSSuggest(name string) (RDNSSSuggestion, error) {
	if name == "" {
		return RDNSSSuggestion{}, errors.New("interface is required")
	}
	if err := rdnss.Qualifies(name); err != nil {
		return RDNSSSuggestion{}, err
	}
	p, err := rdnss.Pick(name, netip.Addr{})
	if err != nil {
		return RDNSSSuggestion{}, err
	}
	return RDNSSSuggestion{Interface: name, Address: p.Addr().String()}, nil
}

func (a *API) rdnssSuggest(w http.ResponseWriter, r *http.Request) {
	sug, err := RDNSSSuggest(r.URL.Query().Get("interface"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid", "interface", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sug)
}
