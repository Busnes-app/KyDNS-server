package web

import (
	"net/http"
	"strings"

	"github.com/Busnes-app/kydns-server/internal/adminapi"
	"github.com/Busnes-app/kydns-server/internal/store"
)

// rdnssForm is the IPv6 DNS advertisement card's contents.
type rdnssForm struct {
	Enabled   bool
	Interface string
	Address   string
}

func rdnssFormOf(v store.Settings) rdnssForm {
	return rdnssForm{Enabled: v.RDNSSEnabled, Interface: v.RDNSSInterface, Address: v.RDNSSAddress}
}

func rdnssFormFrom(r *http.Request) rdnssForm {
	return rdnssForm{
		Enabled:   r.PostFormValue("enabled") != "",
		Interface: strings.TrimSpace(r.PostFormValue("interface")),
		Address:   strings.TrimSpace(r.PostFormValue("address")),
	}
}

func (s *Server) rdnssStatus() adminapi.RDNSSStatus {
	if s.o.API == nil {
		return adminapi.RDNSSStatus{}
	}
	return s.o.API.RDNSSStatus()
}

// renderRDNSS draws the DHCP tab with the card showing form and errMsg.
func (s *Server) renderRDNSS(w http.ResponseWriter, r *http.Request, status int, form rdnssForm, errMsg string) {
	v, _ := s.liveSettings()
	data := s.dhcpData(dhcpFormOf(v), s.dhcpStatus(), "")
	data["RDNSS"], data["RDNSSError"] = form, errMsg
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	s.render(w, r, "dhcp.html", data)
}

// postRDNSSSuggest fills the address in from the interface; it saves nothing.
func (s *Server) postRDNSSSuggest(w http.ResponseWriter, r *http.Request) {
	form := rdnssFormFrom(r)
	sug, err := adminapi.RDNSSSuggest(form.Interface)
	if err != nil {
		s.renderRDNSS(w, r, http.StatusBadRequest, form, err.Error())
		return
	}
	form.Address = sug.Address
	s.renderRDNSS(w, r, http.StatusOK, form, "")
}

func (s *Server) postRDNSSSettings(w http.ResponseWriter, r *http.Request) {
	if s.o.Settings == nil {
		http.Error(w, "settings are not wired", http.StatusInternalServerError)
		return
	}
	form := rdnssFormFrom(r)
	cur, ok := s.liveSettings()
	if !ok {
		s.renderRDNSS(w, r, http.StatusBadRequest, form, errSettingsUnread.Error())
		return
	}
	cur.RDNSSEnabled, cur.RDNSSInterface, cur.RDNSSAddress = form.Enabled, form.Interface, form.Address
	if err := s.o.Settings.Set(cur, ""); err != nil {
		status, reason := s.dhcpSaveRefusal(err)
		s.renderRDNSS(w, r, status, form, reason)
		return
	}
	http.Redirect(w, r, "/dhcp", http.StatusSeeOther)
}
