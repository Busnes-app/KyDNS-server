# Visual refinement evidence: KyDNS (2026-10-26)

Before/after captures and real-browser checks for the fleet visual-refinement batch
(baseline review gap 5: phone navigation in three rows with the version/licence inside
the header, ~7px chart axis labels, misaligned stats). Ratings belong to the independent
reviewer; this record claims no score.

## Stages

| Stage | Source | What it shows |
| --- | --- | --- |
| `00-before` | clean `5ac6796` (origin/main) | baseline; 4/6 checks (queries-chart labels 6px, stat edges 33/49/211px) |
| `01-after` | clean `bd9efce` | first pass; 5/6 checks. **Defective**: the new `.nav-links` wrapper collided with the marketing `styles.css` mobile menu class, so the phone row rendered at the viewport bottom and the page overflowed to 406px; the selected desktop row gained a bottom rule |
| `02-after` | clean `97dd59b` | the one confirmation recapture after renaming it `app-nav-links`; 6/6 checks |

Each stage is light and dark (OS `prefers-color-scheme`, no saved choice) × 1280×900 and
390×844 CSS px, device scale 1, full-page PNGs: `dashboard` and `appearance` (the last
navigation row, so the phone capture shows whether the current row is in view). Manifests
record the source commit, the uncommitted-source hash (null: clean commits; this directory
excluded), OS, Playwright/Chromium, font load state, server, fixture, time, PNG SHA-256s
and checks. `results.mjs` writes `captures.csv` and `behaviour-checks.csv`.

Fixture: kydns built from the tree; admin UI on 127.0.0.1:5391 and DNS on 127.0.0.1:5393;
admin created through `/setup`; local A records nas/printer.home.arpa; twelve loopback
queries. Uptime and last-query ages are live and differ between runs.

## Changes

- Below 860px: brand and Sign out on the top row; the navigation is one sideways-scrolling
  row (`.app-nav-links`). `static/nav.js` scrolls only that row so the current page is in
  view (again after the web font loads); the page does not move and focus stays on the body.
- The version and licence button moves below the page on phones (`.page-legal`); the
  sidebar copy stays on desktop. Both open the same About popover.
- Charts draw in CSS pixels: the viewBox width follows the rendered width and the height
  comes from the template (queries 180px, the pair 130px), so axis labels are 12px at every
  width. Desktop charts are therefore flatter than before (214 → 180px and 150 → 130px).
- Phone stat rows are a two-column grid; every value starts on one of two edges.

Phone header (top of page to the bottom of the navigation): 203px → 113px.

## Checks

Per stage in `behaviour-checks.csv`: header height, axis label box ≥ 11px (stage 02:
15px boxes for 12px text), stat edges per row (33/207px), current row in view on both
pages without page scroll or focus change, the visible licence button opens About, and no
console or CSP errors. Required repository checks are listed in the pull request.

## Known limits

- Full-page captures end the sticky desktop sidebar at 900px; real scrolling keeps it pinned.
- The phone dashboard metric table still wraps labels in IBM Plex Mono.
- The driver gained a port-busy guard after stage 00 (see the KyRecovery record: another
  process briefly held 5391). No stage was affected.

## Reproduce

From the repository root, with a directory holding `@playwright/test` and Chromium and
`dig` on PATH: `CAPTURE_PLAYWRIGHT=/path/to/web node docs/ui-visual-refinement-2026-10-26/capture.mjs 02 after`,
then `node docs/ui-visual-refinement-2026-10-26/results.mjs`. Synthetic data only.
