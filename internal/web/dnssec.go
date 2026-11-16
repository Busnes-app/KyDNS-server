package web

import "net/http"

// validating reports whether "Validate DNSSEC locally" is on.
func (s *Server) validating() bool {
	v, ok := s.liveSettings()
	return ok && v.DNSSECValidate
}

// dnssecClockBanner fires while validation runs without signature time
// checks, because the clock cannot be trusted yet.
func (s *Server) dnssecClockBanner() *Banner {
	if s.o.DNSSECClock == nil || !s.validating() {
		return nil
	}
	skip, why := s.o.DNSSECClock()
	if !skip {
		return nil
	}
	return &Banner{
		Title: "DNSSEC signature dates are not being checked.",
		Body: "KyDNS still validates every signature, but accepts expired ones, because " + why +
			". Without that, a clock that is wrong would fail every signed answer, and NTP could never resolve its servers to fix the clock.",
		Fix: "This clears by itself once NTP has set the clock. If it stays, check that this host runs an NTP client (chrony or systemd-timesyncd) and can reach it.",
	}
}

// dnssecNotice reports whether the one-time upgrade notice should show.
func (s *Server) dnssecNotice() bool {
	if s.o.Store == nil || s.validating() {
		return false
	}
	on, err := s.o.Store.DNSSECNotice()
	if err != nil {
		s.o.Logger.Error("read dnssec notice", "error", err)
	}
	return on
}

// postDNSSECNotice answers the notice: "enable" turns validation on, both
// actions retire the notice.
func (s *Server) postDNSSECNotice(w http.ResponseWriter, r *http.Request) {
	action := r.PostFormValue("action")
	if action != "enable" && action != "dismiss" {
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if s.o.Store == nil || s.o.Settings == nil {
		http.Error(w, "settings are not wired", http.StatusInternalServerError)
		return
	}
	if action == "enable" {
		v, ok := s.liveSettings()
		if !ok {
			http.Error(w, "settings are not loaded yet", http.StatusServiceUnavailable)
			return
		}
		v.DNSSECValidate = true
		if err := s.o.Settings.Set(v, ""); err != nil {
			s.o.Logger.Error("turn on dnssec validation", "error", err)
			http.Error(w, "could not save the setting", http.StatusInternalServerError)
			return
		}
	}
	if err := s.o.Store.ClearDNSSECNotice(); err != nil {
		s.o.Logger.Error("clear dnssec notice", "error", err)
		http.Error(w, "could not save the choice", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
