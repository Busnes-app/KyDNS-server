# DHCP tab: IPv6 DNS advertisement card (2026-11-15)

Before (`14c36fc`, origin/main) and after (branch `feat/rdnss`) of the one changed
screen, `/dhcp`, on a fresh install with synthetic data. Light and dark (OS
`prefers-color-scheme`) at 1280×900 and 390×844 CSS px, full page.

Reproduce from the repository root after `npm ci` in `browser/`:

    node docs/ui-rdnss-2026-11-15/capture.mjs before <kydns built from 14c36fc>
    node docs/ui-rdnss-2026-11-15/capture.mjs after <kydns built from this branch>

The card reuses the DHCP form's markup and ky-ui status badge; no new navigation.
