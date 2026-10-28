# KyDNS Council round 5: opus raw review

Reviewer: opus. Requested `claude-opus-5-5`; actual `claude-opus-5-5` (Opus 5.5, self-reported). Rubric `ky-suite-v2`, pinned at ky-ui `639e588`; all five authority hashes match. Product source `f30749d`; evidence HEAD `f494cdc`. Raw JSON: `opus-round5.json`.

## Result

- **Overall quality (progression value): 7.9.** Mean of the 36 rounded per-file totals: sum 284 / 36 = 71/9 = 7.8889…
- Mean of the unrounded vectors (shown for transparency): 1417/180 = 7.8722…, half-up 7.9.
- **Progression line 8.0: not met.** An exact mean of at least 7.95 is needed; the gap is 11/180 = 0.0611.
- Ky brand fit (separate, never pooled with quality): 241/30 = 8.0333, half-up 8.0. Shell is N/A on the 4 signed-out logins (n=32 for shell).
- **Independently satisfied: no.** 9/10 is not achieved. Gemini unused (advisory only; quota known exhausted; not a blocker).
- **Freeze.** The whole 36-row files array (identities, vectors, observations, artifacts, caveats, totals and ky_fit) was written to `opus-round5.json` at 2026-10-28T01:04:24Z, before any prior, response or historical pixel was read. Canonical serialization: 91562 bytes, sha256 `47ec3de1b92b9d1305f11eac4ba169cbfe4cf311618aecbf904b53e762bdbd0c`. The published array reproduces those bytes, and no vector or observation was revised.

## Cohorts (Ky only; no competitors, so gap N/A; not a ranking)

| Cohort | n | Sum | Mean (exact) | Half-up |
|---|---|---|---|---|
| admin-console | 24 | 932/5 | 233/30 = 7.7667 | 7.8 |
| auth | 4 | 34 | 17/2 = 8.5000 | 8.5 |
| settings | 8 | 318/5 | 159/20 = 7.9500 | 8.0 |

The settings cohort is exactly 7.95, which rounds to 8.0. No cohort vetoes anything and none drives progression.

## Surfaces (coverage, not a ranking)

| Surface | D-light | D-dark | M-light | M-dark | Mean | My R4 | Ky fit mean |
|---|---|---|---|---|---|---|---|
| appearance | 8.0 | 8.0 | 7.8 | 7.8 | 7.9000 | 8.300 | 8.50 |
| blacklists | 7.5 | 7.5 | 7.5 | 7.5 | 7.5000 | 7.300 | 7.90 |
| dashboard | 7.5 | 7.5 | 7.5 | 7.5 | 7.5000 | 7.700 | 7.50 |
| dhcp | 8.0 | 8.0 | 7.8 | 7.8 | 7.9000 | 7.800 | 8.10 |
| discovered | 7.9 | 7.9 | 7.9 | 7.9 | 7.9000 | 7.700 | 7.70 |
| login | 8.5 | 8.5 | 8.5 | 8.5 | 8.5000 | 8.700 | 8.50 |
| records | 8.0 | 7.9 | 7.8 | 7.7 | 7.8500 | 7.500 | 8.00 |
| services | 8.0 | 7.9 | 8.0 | 7.9 | 7.9500 | 7.600 | 8.00 |
| settings | 8.0 | 8.0 | 8.0 | 8.0 | 8.0000 | 7.900 | 8.10 |

Dimension means: hierarchy 7.9444, typography 8.0000, density_layout 7.6667, color 7.8889, visible_affordances 7.7778, coherence 8.0000. Device: desktop 7.9222, mobile 7.8556. Theme: dark 7.8778, light 7.9000.
Brand per dimension: palette_tokens 8.1111 (n=36), type_geometry 8.0000 (n=36), shell 7.5000 (n=32), accent_discipline 8.3333 (n=36), suite_cohesion 8.1111 (n=36). Suite cohesion is judged against the 4 KyIdentity brand-only references. Those references were viewed only for fit and never scored for quality.

## Strongest findings (current pixels)

1. blacklists (7.5, all four): Turn filtering off, Refresh all lists, Enable and Refresh are four equal outline buttons; the protection-reducing action is not distinguished from refreshes and the viewport has no single emphasis. Desktop adds a ~60px gap before Lists and a form edge at x~1016 vs table at 1240; phone cuts row actions at the fold.
2. dashboard (7.5, all four): Cache hit rate chart draws only gridlines with no series and no no-data text; Response time shows a single dot; orange Forwarded series sits near the accent (prior owner decision on recolour recorded in R4, not re-argued).
3. records/services dark: coral danger Remove text reads close in hue to the orange accent. Shared-token pairing (--ky-danger vs --ky-accent dark); outside a product-local batch.
4. records mobile (7.7/7.8): three short records fill the initial viewport; Type label, pill and Value stack less evenly than Services 2x2 pairs.
5. discovered (7.9): legitimate empty state is clear, but the remedy is only an inline orange link (brand-fit accent slip).
6. brand only, all signed-in: account region shows role without signed-in name and a full-width Sign out; phone wordmark subtitle "Scroll navigation" sits in KyIdentity's tagline slot. Navigation redesign is a retained limit.

Strengths: Sign-in 8.5 and Appearance 8.0/7.8 strongest; picker matches the KyIdentity reference card for card. All four R4 findings visibly answered on Records/Services/Blacklists. One 240px left navigation region with accent rail on every signed-in desktop page; flat surfaces; Settings and DHCP each have one orange Save.

## Why I am not satisfied

- Overall quality 71/9 exact (7.8889) half-up 7.9; 8.0 needs exact >= 7.95; gap 0.0611.
- Rule applied exactly; scores were frozen before any prior or response context and were not adjusted toward the threshold.
- Independent satisfaction aligns: list/admin pages are solid (7.5-8.0) with minor slips, not yet 8 across the set. 9/10 not achieved. Gates untested.

## Drift from my own round 4

Prior overall 47/6 (half-up 7.8); current 71/9 (half-up 7.9). Mean delta 1/18 = 0.0556. Byte-identical 23: -1/46 = -0.0217. Changed 13: 5/26 = 0.1923. 4 files have |delta| ≥ 0.5.

The 23 byte-identical images moved only through reviewer calibration (net −0.02, with individual moves of ±0.5 or less). This round reads about as strictly as R4 overall but differs per dimension. The 13 changed images show visible fixes for R4 findings 1–4, which I confirmed against 4 reopened R4 originals. The rubric gives no way to split their deltas into product and calibration parts, so I claim no causal numbers. Dashboard pixel differences are natural uptime and chart-minute drift. Brand fit fell on identical pixels too, which is calibration.

| File (kydns-04-after-) | Same bytes | R4 | R5 | Δ | Dim Δ | Cause | Explanation |
|---|---|---|---|---|---|---|---|
| appearance-mobile-dark.png | yes | 8.3 | 7.8 | -0.5 | density_layout -1, color -1, visible_affordances -1 | reviewer_calibration | Byte-identical; density 8->7 (clipped "cklists" scroll row counted against rhythm this round), color and affordances 9->8: all calibration on unchanged pixels, not a regression. |
| appearance-mobile-light.png | yes | 8.3 | 7.8 | -0.5 | density_layout -1, color -1, visible_affordances -1 | reviewer_calibration | Byte-identical; density 8->7 (clipped "cklists" scroll row counted against rhythm this round), color and affordances 9->8: all calibration on unchanged pixels, not a regression. |
| records-desktop-light.png | no | 7.5 | 8.0 | +0.5 | hierarchy +1, color +1, coherence +1 | mixed (visible product change + calibration) | Changed pixels: outline danger Remove buttons became quiet danger text and the precedence note moved into the form column above Add record (both visible vs the R4 original). Hierarchy and coherence +1 align with these; light color +1 because Remove no longer forms a red column; dark color stays 7 for coral-vs-accent hue. Magnitude also contains calibration; no exact split claimed. |
| services-desktop-light.png | no | 7.5 | 8.0 | +0.5 | hierarchy +1, color +1, coherence +1 | mixed (visible product change + calibration) | Changed pixels: outline danger Remove buttons became quiet danger text (visible vs R4 original). Hierarchy and coherence +1 align; light color +1 because Remove no longer forms a red column. Magnitude also contains calibration; no exact split claimed. |

The other 32 rows are below 0.5. Each has its own explanation and both raw vectors (prior_scores, scores) in the JSON `drift.files`.

## Response record check (post-freeze)

- **Desktop danger-outline Remove competes**: visible: yes: quiet danger text in R5 records/services desktop vs outline in R4 originals. Not provable from pixels: 40px target, POST/CSRF/replica behaviour, focus.
- **Blacklist pills**: visible: yes: "Filtering is on" prose with bold on; per-source "off" plain text; built-in neutral and amber never-loaded pills remain. Not provable from pixels: other states.
- **Phone Records View orphaned**: visible: yes: Type|View paired, Value full width. Not provable from pixels: empty-state colspan correction (not captured).
- **Precedence note outside form column**: visible: yes: note at x~470 inside form above Add record on desktop. Not provable from pixels: phone placement below fold.
- **R4 font misread**: visible: R5 frozen observations make no font-mixing claim. Not provable from pixels: font identity.

## Corrections (outside frozen rows)

- **frozen rows services-desktop-dark/light, typography observation**: Says the "Aliases (comma separated)" label is wrapped. The current original (viewed pre-freeze) and the R4 original reopened post-freeze both show it on one line ending ~22px before the field; "tight to its field" stands. factual misread recorded outside frozen rows; vectors and observations unchanged; typography 8 not driven by the wrap claim.
- **frozen rows login-*-dark, coherence observation**: "Stamp mark renders without its dark tile" is a visibility claim: pixels cannot distinguish an absent tile from one matching the dark page colour. qualification recorded; frozen text unchanged.

## One bounded batch (warranted)

Progression unmet by a small exact margin; two concrete, product-local findings on the two lowest surfaces. Scope: One bounded batch; no shared token change, no navigation/Settings redesign, no new features, no chart recolour, routes/copy/behaviour preserved.

- Dashboard: when a chart window has no samples (Cache hit rate, Response time), state it in place in text instead of drawing bare gridlines; keep charts, data and colours.
- Blacklists: give the Filtering section one action (Turn filtering off) and move Refresh all lists to the Lists header as a quiet action like Add list; tighten the gap before Lists to the page section rhythm. Keep Enable/Refresh per row and all states.

Not predicted. Unknown until recaptured and blind-scored. Not a demand for further rounds on individual screens or tiny gaps once the Opus rounded overall accepts.

## Gates, unscored and limits

- Keyboard, focus, contrast, states and responsive: all **untested**. Screenshots cannot pass them.
- Unscored: replication (4 combinations). N/A: standalone fixture hides the link and /replication 303-redirects to Dashboard; excluded from counts and means.
- Unscored: below-fold sections. Settings API tokens/SSO/backup/recovery/upstream/config; lower Services/Records/Blacklists/DHCP forms; Dashboard table remainder.
- Unscored: live integrations. replication, SSO, KyRecovery, external forwarding, blacklist fetch.
- Unscored: ordinary-user workflows. N/A: KyDNS is administrator-only.
- Unscored: unseen states. errors, loading, populated leases/discovery, filled forms.
- Unscored: behaviour, keyboard, focus, all-theme contrast, accessibility, security, production. outside static image review.
- Inputs verified by script: 184/184 artifacts match sha256, byte size and PNG IHDR (36 current, 144 historical decoded only for headers, 4 brand). The inventory equals the disk glob and the 9×2×2 matrix is complete. All 23 cross-round exact matches were recomputed from hashes. The duplicate audit covers exactly the 306 same-size pairs (324 different-size pairs excluded, 630 total). Its threshold arithmetic recomputes with 0 mismatches and finds 0 exact or near duplicates (closest pair 4.18%). The measured audit was reused, not rerun. `brand-references.json` purpose strings still say KyDrive (cosmetic).
- Blind access: before the freeze, only the allowlist, directory listings and `git rev-parse HEAD`. After the freeze, only my R4 pair, `round5-responses.json` (read in full) and 4 matched R4 originals. Never opened: peer pairs, audits, COUNCIL, CSV/XLSX, provenance, capture checks, build-report.py, source and git log.
- Viewing: each of the 36 current PNGs and the 4 brand references was opened once, individually, at its original path with the native image tool. No montage or crops were used. The tool may resample internally, so pixel positions are estimates.
- Font identity, exact CSS sizes, computed contrast and functionality are not proven by pixels. Observations describe visible facts only.
- No security, integration, all-theme or production claims. DOX: no AGENTS.md change; this reviewer-owned raw pair alters no contract.

## Per-file scores (frozen)

| File | Cohort | State | H | T | D | C | A | Co | Exact | Total | Ky fit |
|---|---|---|---|---|---|---|---|---|---|---|---|
| appearance-desktop-dark.png | settings | populated | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.6 |
| appearance-desktop-light.png | settings | populated | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.6 |
| appearance-mobile-dark.png | settings | populated | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| appearance-mobile-light.png | settings | populated | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| blacklists-desktop-dark.png | admin-console | sparse | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.0 |
| blacklists-desktop-light.png | admin-console | sparse | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.0 |
| blacklists-mobile-dark.png | admin-console | sparse | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 7.8 |
| blacklists-mobile-light.png | admin-console | sparse | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 7.8 |
| dashboard-desktop-dark.png | admin-console | populated | 8 | 8 | 7 | 7 | 7 | 8 | 7.50 | 7.5 | 7.6 |
| dashboard-desktop-light.png | admin-console | populated | 8 | 8 | 7 | 7 | 7 | 8 | 7.50 | 7.5 | 7.6 |
| dashboard-mobile-dark.png | admin-console | populated | 8 | 8 | 7 | 7 | 7 | 8 | 7.50 | 7.5 | 7.4 |
| dashboard-mobile-light.png | admin-console | populated | 8 | 8 | 7 | 7 | 7 | 8 | 7.50 | 7.5 | 7.4 |
| dhcp-desktop-dark.png | admin-console | empty | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| dhcp-desktop-light.png | admin-console | empty | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| dhcp-mobile-dark.png | admin-console | empty | 7 | 8 | 8 | 8 | 8 | 8 | 7.80 | 7.8 | 8.0 |
| dhcp-mobile-light.png | admin-console | empty | 7 | 8 | 8 | 8 | 8 | 8 | 7.80 | 7.8 | 8.0 |
| discovered-desktop-dark.png | admin-console | empty | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 7.8 |
| discovered-desktop-light.png | admin-console | empty | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 7.8 |
| discovered-mobile-dark.png | admin-console | empty | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 7.6 |
| discovered-mobile-light.png | admin-console | empty | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 7.6 |
| login-desktop-dark.png | auth | sparse | 9 | 8 | 8 | 9 | 9 | 8 | 8.50 | 8.5 | 8.5 |
| login-desktop-light.png | auth | sparse | 9 | 8 | 8 | 9 | 9 | 8 | 8.50 | 8.5 | 8.5 |
| login-mobile-dark.png | auth | sparse | 9 | 8 | 8 | 9 | 9 | 8 | 8.50 | 8.5 | 8.5 |
| login-mobile-light.png | auth | sparse | 9 | 8 | 8 | 9 | 9 | 8 | 8.50 | 8.5 | 8.5 |
| records-desktop-dark.png | admin-console | sparse | 8 | 8 | 8 | 7 | 8 | 8 | 7.85 | 7.9 | 8.0 |
| records-desktop-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| records-mobile-dark.png | admin-console | sparse | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 7.8 |
| records-mobile-light.png | admin-console | sparse | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.0 |
| services-desktop-dark.png | admin-console | sparse | 8 | 8 | 8 | 7 | 8 | 8 | 7.85 | 7.9 | 8.0 |
| services-desktop-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| services-mobile-dark.png | admin-console | sparse | 8 | 8 | 8 | 7 | 8 | 8 | 7.85 | 7.9 | 7.8 |
| services-mobile-light.png | admin-console | sparse | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| settings-desktop-dark.png | settings | populated | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| settings-desktop-light.png | settings | populated | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.2 |
| settings-mobile-dark.png | settings | populated | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| settings-mobile-light.png | settings | populated | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
