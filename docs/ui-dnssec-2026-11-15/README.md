# "Validate DNSSEC locally" UI evidence (2026-11-15)

Before (`origin/main` 14c36fc) and after (branch `feat/dnssec-validate`) captures of
the two changed screens, Dashboard and Settings > Server, in OS light and dark at
1280×900 and 390×844 CSS px, full page. Made by `capture.mjs` against a disposable
loopback server with synthetic data; no query left the machine.

- Dashboard (after) shows the one-time upgrade notice, armed the way the migration
  arms it on an upgraded database.
- Settings > Server (after) has "Validate DNSSEC locally" under Upstreams.
- Not captured: the clock-safety banner. A real server cannot be put in that state
  without changing the system clock; `TestDNSSECClockBanner` covers its rendering.
