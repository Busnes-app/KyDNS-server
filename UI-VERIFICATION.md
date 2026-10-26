# Shared UI verification

> Evidence scope: this document records the revision, environment and capture conditions below. Screenshots and generated fixtures demonstrate appearance; they do not qualify live email delivery, push delivery, recovery custody or every product feature. Test counts describe that recorded run, not the current suite.

## Change

Generate Busnes palette previews from shared tokens and verify vendored assets in CI. Shared assets are pinned to ky-ui 0.2.0 with content hashes. Product layouts, saved theme keys and named presets remain local.

## Capture conditions

Captured 2026-09-25 from this branch, using the application UI (not a design mockup). Dashboard in a real scratch server, with no configured DNS records or query traffic.

OS-following Busnes Light and Dark were captured at 1280×900 and 390×844 CSS pixels. Browser device scaling may make PNG dimensions larger. Document width stayed within the viewport in these captured states; local navigation/table scrolling is intentional. Screenshots show the selected-page accent, not a complete accessibility audit.

No complete end-to-end product workflow or exhaustive named-theme audit is claimed.

## Checks

go test ./internal/web passed; vendor integrity check passed. Central ky-ui sync --check verified all ten consumers. Screenshot coverage is Busnes Light/Dark; existing named choices are retained, but not every named palette/page combination was visually exercised.

## Screenshots

| Light | Dark |
| --- | --- |
| ![Desktop light](docs/ky-ui-light-desktop.png) | ![Desktop dark](docs/ky-ui-dark-desktop.png) |
| ![Mobile light](docs/ky-ui-light-mobile.png) | ![Mobile dark](docs/ky-ui-dark-mobile.png) |

## Reproduce

Run the Go server with isolated preview data following its README; run the checks above from the repository root. Use System theme, emulate OS light/dark, and inspect both viewport sizes. Do not point preview instances at production data. For KyVault, use a configured development KyIdentity or explicitly labeled read-only browser fixtures; never bypass backend authentication.

## Services health UI removal — 2026-10-26

Verified the actual Services page on a scratch server with three services, one
proxy route and one alias, in Busnes Dark at 1280×800 and 390×844 CSS pixels.
The health column, check URL, self-signed certificate control and health polling
are absent. Document width matched both viewports; the service table retains
its existing local horizontal scrolling on mobile. Screenshots were captured
with the T3 collaborative browser as session artifacts, not committed fixtures.
Other themes and production deployment were not exercised for this change.

`make test`, `make build`, `go vet ./...`, vendor integrity, JavaScript syntax,
and the AGENTS.md installer-boundary race checks passed. The targeted Services
template detector reported no findings. A regression test verifies that the
Services page neither renders health controls nor reads the health provider;
existing session/API monitoring endpoints remain available.
