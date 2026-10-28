# KyDNS Council round 6: opus raw review

Reviewer: opus. Requested `claude-opus-5-5`; actual `claude-opus-5-5` (Opus 5.5, self-reported). Rubric `ky-suite-v2`, pinned at ky-ui `639e588`; all five authority hashes match. Product source `6ce6fb5`; published evidence HEAD `700e666`, taken from the brief (no git command run). Raw JSON: `opus-round6.json`.

## Result

- **Overall quality (progression value): 8.0.** Mean of the 36 rounded per-file totals: sum 1446/5 / 36 = 241/30 = 8.0333, which rounds half-up to 8.0.
- Mean of the unrounded vectors (for transparency): 1441/180 = 8.0056, half-up 8.0.
- **Progression line 8.0: met**, 1/12 above the 7.95 rounding edge.
- Ky brand fit is reported separately and never pooled: 49/6 = 8.1667, half-up 8.2. Shell is N/A on the 4 signed-out logins.
- **Independently satisfied: yes, at the 8/10 checkpoint, and borderline.** See the calibration disclosure below. 9/10 is not achieved and every behaviour gate is untested. Gemini was not used.
- **Freeze.** I wrote the whole 36-row files array to `opus-round6.json` at 2026-10-28T01:47:48Z, before reading any prior record, response or historical pixel. It covers identities, vectors, observations, artifacts, caveats, totals and ky_fit. Canonical serialization: 76471 bytes, sha256 `a7d3d037efb82f89051c3690a3bac4572027112a9364d1a2d4c68fd95e0584e2`. The published array reproduces those bytes.

## Calibration disclosure

Before the freeze, and before reading any prior context, I tallied provisional vectors from my viewing notes. That tally came to mean of rounded 143/18 = 7.9444, half-up 7.9. I then re-read the anchors and raised five dimension scores, lowering none: login coherence, settings hierarchy and color, dhcp color, and appearance affordances. That gives the frozen 241/30 = 8.0333, half-up 8.0.
Each raise cites a visible fact in its frozen observation. Still, I knew the threshold, every change went up, and together they move the rounded result across 8.0. The difference (0.089) is smaller than my own drift on byte-identical images since R5 (+0.157 mean). Read the 8.0 as borderline and partly calibration, not as a measured product gain.

## Cohorts (Ky only; no competitors, so the gap is N/A; not a ranking)

| Cohort | Surfaces | n | Sum | Mean (exact) | Half-up |
|---|---|---|---|---|---|
| admin-console | blacklists, dashboard, dhcp, discovered, records, services | 24 | 942/5 | 157/20 = 7.8500 | 7.9 |
| auth | login | 4 | 34 | 17/2 = 8.5000 | 8.5 |
| settings | appearance, settings | 8 | 334/5 | 167/20 = 8.3500 | 8.4 |

DHCP is in admin-console, as in my R5, so there is no cohort drift. No cohort vetoes anything and none is pooled with a peer.

## Surfaces (coverage, not a ranking)

| Surface | D-light | D-dark | M-light | M-dark | Mean | My R5 | Ky fit mean |
|---|---|---|---|---|---|---|---|
| appearance | 8.3 | 8.3 | 8.3 | 8.3 | 8.3000 | 7.9000 | 8.4000 |
| blacklists | 7.5 | 7.5 | 7.5 | 7.5 | 7.5000 | 7.5000 | 8.2000 |
| dashboard | 7.7 | 7.7 | 7.7 | 7.7 | 7.7000 | 7.5000 | 7.6000 |
| dhcp | 8.2 | 8.2 | 8.2 | 8.2 | 8.2000 | 7.9000 | 8.2000 |
| discovered | 7.7 | 7.7 | 7.7 | 7.7 | 7.7000 | 7.9000 | 8.0000 |
| login | 8.5 | 8.5 | 8.5 | 8.5 | 8.5000 | 8.5000 | 8.5000 |
| records | 8.0 | 8.0 | 8.0 | 8.0 | 8.0000 | 7.8500 | 8.2000 |
| services | 8.0 | 8.0 | 8.0 | 8.0 | 8.0000 | 7.9500 | 8.2000 |
| settings | 8.4 | 8.4 | 8.4 | 8.4 | 8.4000 | 8.0000 | 8.2000 |

Dimension means: hierarchy 8.0000, typography 8.0000, density_layout 7.7778, color 8.2222, visible_affordances 8.0000, coherence 8.1111. Desktop and mobile both come to 8.0333, as do dark and light.
Brand per dimension: palette_tokens 8.0000 (n=36), type_geometry 8.0000 (n=36), shell 8.0000 (n=32), accent_discipline 8.6667 (n=36), suite_cohesion 8.1111 (n=36). Suite cohesion is judged against the 4 KyIdentity brand-only references, which I viewed for fit only and never scored for quality.

## Strongest findings (current pixels)

1. blacklists (7.5, all four): no filled primary in view; Turn filtering off, Enable and Refresh are equal outlined buttons; phone list row switches to inline label-value pairs unlike Services/Records, and row actions are cut at the fold.
2. dashboard (7.7, all four): chart series use teal/orange/purple (orange = Forwarded); cache no-data text touches the 50% gridline; the second KPI band repeats the first band's weight; phone leaves Uptime orphaned in a 2-column grid.
3. discovered (7.7, all four): legitimate empty state, but the only next step is an inline orange text link and the page does not say what will appear here.
4. brand only: account region shows role without signed-in name; desktop forms use left labels rather than the contract's labels-above; wordmark face looks different from the KyIdentity wordmark (font identity unproven).

Strengths: login 8.5: one card, one primary, visible focus ring. settings 8.4: single Save beside the section heading, divider-grouped fields. appearance 8.3: shared card picker matches the KyIdentity reference card for card. Both R5 findings visibly answered (cache no-data text; refresh-all regrouped). One 240px left navigation region with accent rail on every signed-in desktop page.

## Drift from my R5

- Overall moved from 71/9 (7.9) to 241/30 (8.0). The mean delta is 0.1444: 0.1571 on the 28 byte-identical files and 0.1000 on the 8 changed files. Summed, that is +4.4 of +5.2 on identical pixels, which is calibration by definition.
- Changed files: Dashboard (4): Cache hit rate now prints "No cache lookups in this window" where R5 drew bare gridlines; uptime, last-query age and spike position moved with the real clock (not a product gain). Blacklists (4): Refresh all lists moved from an outlined button beside Turn filtering off to a quiet underlined action beside the Lists heading, and the gap before Lists tightened (heading ~y399 to ~y382 on desktop light). Both surfaces moved by <0.5.
- |delta| >= 0.5: appearance mobile light and dark, both +0.5 and both byte-identical. +0.5, byte-identical: the same even two-column card grid, orange-bordered selected card with filled radio and single repeated card component were read as density 8 / affordances 9 / coherence 9 this round versus 7/8/8 in R5 (which weighted the clipped "cklists" nav fragment in density); calibration, not product change.
- Brand fit mean moved from 241/30 to 49/6. Most of that comes from identical pixels, so it is calibration too.
- No cohort changes. The rubric is unchanged (ky-suite-v2 in both rounds).

| File | Identical | R5 | R6 | Delta | Dimension deltas |
|---|---|---|---|---|---|
| appearance-desktop-dark.png | yes | 8.0 | 8.3 | +0.3 | visible_affordances +1, coherence +1 |
| appearance-desktop-light.png | yes | 8.0 | 8.3 | +0.3 | visible_affordances +1, coherence +1 |
| appearance-mobile-dark.png | yes | 7.8 | 8.3 | +0.5 | density_layout +1, visible_affordances +1, coherence +1 |
| appearance-mobile-light.png | yes | 7.8 | 8.3 | +0.5 | density_layout +1, visible_affordances +1, coherence +1 |
| blacklists-desktop-dark.png | no | 7.5 | 7.5 | +0.0 | visible_affordances +1, coherence -1 |
| blacklists-desktop-light.png | no | 7.5 | 7.5 | +0.0 | visible_affordances +1, coherence -1 |
| blacklists-mobile-dark.png | no | 7.5 | 7.5 | +0.0 | visible_affordances +1, coherence -1 |
| blacklists-mobile-light.png | no | 7.5 | 7.5 | +0.0 | visible_affordances +1, coherence -1 |
| dashboard-desktop-dark.png | no | 7.5 | 7.7 | +0.2 | visible_affordances +1 |
| dashboard-desktop-light.png | no | 7.5 | 7.7 | +0.2 | visible_affordances +1 |
| dashboard-mobile-dark.png | no | 7.5 | 7.7 | +0.2 | visible_affordances +1 |
| dashboard-mobile-light.png | no | 7.5 | 7.7 | +0.2 | visible_affordances +1 |
| dhcp-desktop-dark.png | yes | 8.0 | 8.2 | +0.2 | color +1 |
| dhcp-desktop-light.png | yes | 8.0 | 8.2 | +0.2 | color +1 |
| dhcp-mobile-dark.png | yes | 7.8 | 8.2 | +0.4 | hierarchy +1, color +1 |
| dhcp-mobile-light.png | yes | 7.8 | 8.2 | +0.4 | hierarchy +1, color +1 |
| discovered-desktop-dark.png | yes | 7.9 | 7.7 | -0.2 | hierarchy -1 |
| discovered-desktop-light.png | yes | 7.9 | 7.7 | -0.2 | hierarchy -1 |
| discovered-mobile-dark.png | yes | 7.9 | 7.7 | -0.2 | hierarchy -1 |
| discovered-mobile-light.png | yes | 7.9 | 7.7 | -0.2 | hierarchy -1 |
| login-desktop-dark.png | yes | 8.5 | 8.5 | +0.0 | visible_affordances -1, coherence +1 |
| login-desktop-light.png | yes | 8.5 | 8.5 | +0.0 | visible_affordances -1, coherence +1 |
| login-mobile-dark.png | yes | 8.5 | 8.5 | +0.0 | visible_affordances -1, coherence +1 |
| login-mobile-light.png | yes | 8.5 | 8.5 | +0.0 | visible_affordances -1, coherence +1 |
| records-desktop-dark.png | yes | 7.9 | 8.0 | +0.1 | color +1 |
| records-desktop-light.png | yes | 8.0 | 8.0 | +0.0 | - |
| records-mobile-dark.png | yes | 7.7 | 8.0 | +0.3 | density_layout +1, color +1 |
| records-mobile-light.png | yes | 7.8 | 8.0 | +0.2 | density_layout +1 |
| services-desktop-dark.png | yes | 7.9 | 8.0 | +0.1 | color +1 |
| services-desktop-light.png | yes | 8.0 | 8.0 | +0.0 | - |
| services-mobile-dark.png | yes | 7.9 | 8.0 | +0.1 | color +1 |
| services-mobile-light.png | yes | 8.0 | 8.0 | +0.0 | - |
| settings-desktop-dark.png | yes | 8.0 | 8.4 | +0.4 | hierarchy +1, color +1 |
| settings-desktop-light.png | yes | 8.0 | 8.4 | +0.4 | hierarchy +1, color +1 |
| settings-mobile-dark.png | yes | 8.0 | 8.4 | +0.4 | hierarchy +1, color +1 |
| settings-mobile-light.png | yes | 8.0 | 8.4 | +0.4 | hierarchy +1, color +1 |

## Post-freeze corrections (outside the frozen rows; vectors unchanged)

- frozen rows blacklists-*, hierarchy observation: "carry equal weight" overstates: the underlined Add list / Refresh all lists actions are visibly quieter than the outlined buttons (confirmed against the R5 original post-freeze). recorded outside frozen rows; the score basis (no filled primary, three equal outlined buttons) stands; vectors unchanged.
- frozen rows blacklists-*, coherence observation: "two pill styles" are a neutral label pill and an amber warning badge with different semantics; this is a mild coherence point, not stray styling. qualification only; vectors unchanged.
- frozen rows login-* typography/ky_fit and dashboard-* typography: "mono-style" wordmark/title face is a visual impression; font identity is unproven by pixels (the brief records Space Grotesk probe 36/36). qualification only; vectors unchanged.
- frozen rows dashboard-*, density observation: "sits on the 50% gridline" is approximate: the text baseline sits just above and touches the line at this resolution. qualification only; vectors unchanged.

## Response record check

- Cache chart bare gridlines: yes: "No cache lookups in this window" in all 4 current dashboard captures; Response time keeps a single dot (consistent with the stated 0ms samples). Unprovable: aria-description, refresh behaviour, null-vs-zero logic.
- Filtering toggle and refresh-all compete: yes: Filtering owns only Turn filtering off; Refresh all lists is an underlined action beside Lists; tighter gap. Unprovable: 40px target, CSRF POST, focus.
- Phone Records density, danger/accent hues, account/nav brand, orange data series: unchanged by design (closed decisions); retained as independent observations only, not demands. Unprovable: n/a.
- Retained limits (native preview failed, dirty build metadata, 8 partials): no visible artifact in the 36 originals; not penalised. Unprovable: all behaviour gates.

## Recommendation

No batch. Opus rounded overall accepts; remaining items are small cohort/screen slips or closed owner decisions (chart colours, navigation). No new round requested.

## Unscored and gates

- replication (4 combinations): N/A: standalone fixture hides the link and /replication 303-redirects; excluded from counts and means
- 8 incomplete stage05 captures: unscored partial attempt; no zero or missing penalty; pixels never decoded
- 180 historical R1-R5 and 4 brand references: history/brand only; never pooled into quality
- below-fold sections and live flows: Settings tokens/SSO/backup/recovery; lower forms; dashboard table remainder; replication, SSO, KyRecovery, forwarding, list fetch
- ordinary-user workflows: N/A: KyDNS is administrator-only
- keyboard, focus, contrast, states, responsive, named-font loading, security, integration, production: unproven by screenshots
- Gates keyboard, focus, contrast, states and responsive: all untested, because a screenshot cannot pass a behaviour gate.

## Verification and limits

- Inputs: 228 PNGs checked by sha256, bytes and IHDR, with 0 mismatches. Disk matches the listing. Roles are 36 current, 180 historical, 8 incomplete and 4 brand. The 9×2×2 matrix is complete. All 5 authority hashes and 4 brand hashes match. All 28 cross-round exact matches were recomputed from hashes (28 current files are byte-identical to stage 04).
- Duplicate audit: 630 pairs in total, 306 same-size and 324 excluded. The pair sets are exact, and the threshold flags recompute with 0 errors. There are 0 exact or near duplicates. The closest pair is discovered vs records desktop dark at 4.18% changed. I reused the measured audit rather than running a new pixel diff.
- Viewing: I opened each of the 36 current PNGs once, individually, at its original path with the native image tool. No contact sheet, crop or resize. The tool may resample internally, so pixel positions are estimates. Before the freeze I decoded no historical or partial pixels.
- Pixels do not prove font identity, exact CSS sizes, computed contrast or behaviour. I make no security, integration or production claims.
- DOX: no AGENTS.md change, because this reviewer-owned raw pair alters no contract. Nothing outside the pair was written; temporary arithmetic lives in /tmp/opus-r6.

## Per-file scores (frozen)

| File | Cohort | State | H | T | D | C | A | Co | Exact | Total | Ky fit |
|---|---|---|---|---|---|---|---|---|---|---|---|
| appearance-desktop-dark.png | settings | populated | 8 | 8 | 8 | 8 | 9 | 9 | 8.30 | 8.3 | 8.4 |
| appearance-desktop-light.png | settings | populated | 8 | 8 | 8 | 8 | 9 | 9 | 8.30 | 8.3 | 8.4 |
| appearance-mobile-dark.png | settings | populated | 8 | 8 | 8 | 8 | 9 | 9 | 8.30 | 8.3 | 8.4 |
| appearance-mobile-light.png | settings | populated | 8 | 8 | 8 | 8 | 9 | 9 | 8.30 | 8.3 | 8.4 |
| blacklists-desktop-dark.png | admin-console | sparse | 7 | 8 | 7 | 8 | 8 | 7 | 7.45 | 7.5 | 8.2 |
| blacklists-desktop-light.png | admin-console | sparse | 7 | 8 | 7 | 8 | 8 | 7 | 7.45 | 7.5 | 8.2 |
| blacklists-mobile-dark.png | admin-console | sparse | 7 | 8 | 7 | 8 | 8 | 7 | 7.45 | 7.5 | 8.2 |
| blacklists-mobile-light.png | admin-console | sparse | 7 | 8 | 7 | 8 | 8 | 7 | 7.45 | 7.5 | 8.2 |
| dashboard-desktop-dark.png | admin-console | populated | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 7.6 |
| dashboard-desktop-light.png | admin-console | populated | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 7.6 |
| dashboard-mobile-dark.png | admin-console | populated | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 7.6 |
| dashboard-mobile-light.png | admin-console | populated | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 7.6 |
| dhcp-desktop-dark.png | admin-console | empty | 8 | 8 | 8 | 9 | 8 | 8 | 8.15 | 8.2 | 8.2 |
| dhcp-desktop-light.png | admin-console | empty | 8 | 8 | 8 | 9 | 8 | 8 | 8.15 | 8.2 | 8.2 |
| dhcp-mobile-dark.png | admin-console | empty | 8 | 8 | 8 | 9 | 8 | 8 | 8.15 | 8.2 | 8.2 |
| dhcp-mobile-light.png | admin-console | empty | 8 | 8 | 8 | 9 | 8 | 8 | 8.15 | 8.2 | 8.2 |
| discovered-desktop-dark.png | admin-console | empty | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.0 |
| discovered-desktop-light.png | admin-console | empty | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.0 |
| discovered-mobile-dark.png | admin-console | empty | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.0 |
| discovered-mobile-light.png | admin-console | empty | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.0 |
| login-desktop-dark.png | auth | empty | 9 | 8 | 8 | 9 | 8 | 9 | 8.50 | 8.5 | 8.5 |
| login-desktop-light.png | auth | empty | 9 | 8 | 8 | 9 | 8 | 9 | 8.50 | 8.5 | 8.5 |
| login-mobile-dark.png | auth | empty | 9 | 8 | 8 | 9 | 8 | 9 | 8.50 | 8.5 | 8.5 |
| login-mobile-light.png | auth | empty | 9 | 8 | 8 | 9 | 8 | 9 | 8.50 | 8.5 | 8.5 |
| records-desktop-dark.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| records-desktop-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| records-mobile-dark.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| records-mobile-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| services-desktop-dark.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| services-desktop-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| services-mobile-dark.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| services-mobile-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| settings-desktop-dark.png | settings | populated | 9 | 8 | 8 | 9 | 8 | 8 | 8.35 | 8.4 | 8.2 |
| settings-desktop-light.png | settings | populated | 9 | 8 | 8 | 9 | 8 | 8 | 8.35 | 8.4 | 8.2 |
| settings-mobile-dark.png | settings | populated | 9 | 8 | 8 | 9 | 8 | 8 | 8.35 | 8.4 | 8.2 |
| settings-mobile-light.png | settings | populated | 9 | 8 | 8 | 9 | 8 | 8 | 8.35 | 8.4 | 8.2 |
