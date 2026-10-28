# KyDNS Council round 4: opus raw review

Reviewer: opus. Requested `claude-opus-5-5`; actual `claude-opus-5-5` (Opus 5.5). Rubric `ky-suite-v2` pinned at ky-ui `639e588`; all five authority hashes match. Product source `2eb750d`; evidence HEAD `480d289` (from the brief; git not read). Raw JSON: `opus-round4.json`.

## Result

- **Overall quality (progression value): 7.8.** Mean of the 36 rounded per-file totals: sum 282 / 36 = 47/6 = 7.8(3).
- Unrounded-vector mean (transparency): 703/90 = 7.8(1), half-up 7.8.
- **Progression line 8.0: not met.** Needs an exact mean ≥ 7.95; gap 7/60 = 0.11(6).
- Ky brand fit (separate, never pooled): 76/9 = 8.(4), half-up 8.4. Shell N/A on the 4 signed-out logins (n=32 for shell).
- **Independently satisfied: no.** 9/10 not achieved. Gemini unused (quota exhausted; not a blocker).
- **Freeze.** The full 36-row files array (identities, dimensions, integer vectors, observations, ky_fit, exact and rounded totals) was written to `opus-round4.json` at 2026-10-28T00:37:43Z before any prior, response or historical pixel was read. Canonical serialization 78603 bytes, sha256 `47e91926034c524181d5fb078f645ffbfe5ba5077f4db90ad7d1672960546577` (`json.dumps(files,ensure_ascii=False,sort_keys=True,separators=(',',':'),allow_nan=False).encode('utf-8')`, no newline). The published array reproduces that hash; no vector or observation was revised.

## Cohorts (Ky only; no competitors, gap N/A; not a ranking)

| Cohort | n | Sum | Mean (exact) | Half-up |
|---|---|---|---|---|
| admin-console | 24 | 912/5 | 7.6 | 7.6 |
| auth | 4 | 174/5 | 8.7 | 8.7 |
| settings | 8 | 324/5 | 8.1 | 8.1 |

## Surfaces (coverage, not a ranking)

| Surface | D-light | D-dark | M-light | M-dark | Mean | My R3 mean | Ky fit |
|---|---|---|---|---|---|---|---|
| appearance | 8.3 | 8.3 | 8.3 | 8.3 | 8.3 | 8 | 8.8 |
| blacklists | 7.3 | 7.3 | 7.3 | 7.3 | 7.3 | 6.5 | 8.4 |
| dashboard | 7.7 | 7.7 | 7.7 | 7.7 | 7.7 | 7 | 8.2 |
| dhcp | 7.8 | 7.8 | 7.8 | 7.8 | 7.8 | 7.25 | 8.4 |
| discovered | 7.7 | 7.7 | 7.7 | 7.7 | 7.7 | 7.2 | 8.2 |
| login | 8.7 | 8.7 | 8.7 | 8.7 | 8.7 | 8 | 8.8 |
| records | 7.5 | 7.5 | 7.5 | 7.5 | 7.5 | 6.8 | 8.4 |
| services | 7.5 | 7.5 | 7.7 | 7.7 | 7.6 | 6.7 | 8.4 |
| settings | 7.8 | 7.8 | 8.0 | 8.0 | 7.9 | 7.9 | 8.4 |

Dimension means: hierarchy 7.6(1), typography 8, density_layout 7.6(1), color 8, visible_affordances 7.(8), coherence 7.(8). Device: desktop 7.8(1), mobile 7.8(5). Theme: light and dark both 7.8(3).
Brand per dimension: palette_tokens 9 (n=36), type_geometry 8.(2) (n=36), shell 8 (n=32), accent_discipline 8.(7) (n=36), suite_cohesion 8.(1) (n=36).

## Strongest findings (current pixels)

1. **records, services (desktop, both themes)**: A column of three danger-outline Remove buttons is the strongest colour mass on each list page, above and competing with the single orange submit; the phone layouts already use quiet text Remove.
2. **blacklists (all)**: Four pill treatments (on, built-in, off, amber never loaded) plus two equal outline buttons and a quiet header link: no single emphasis in the viewport; mono on/off pills read toggle-like next to an Enable button.
3. **records (mobile)**: Type/Value pair then View alone under Type leaves the right track empty; each 3-field record spends ~180px.
4. **records (desktop)**: Precedence note sits under the Add record button at the page edge rather than in the form column near its fields (unchanged from R3).
5. **dashboard (all)**: Second KPI strip and Metric table restate counts; orange Forwarded series echoes the accent (owner declined recolour/deletion; recorded, not re-argued).
6. **all signed-in**: Brand-fit only: account region shows role but no signed-in name and a full-width Sign out; phone wordmark subtitle reads "Scroll navigation". Differs from KyIdentity reference; navigation redesign is a retained limit.

Strengths: Sign-in (8.7) and Appearance (8.3) are the strongest; the picker matches the KyIdentity reference card for card. R3 findings were answered in visible pixels: neutral list-header jump links, 13px row meta, slash-break source URL, label-over-value phone tracks with Remove beside the name. One 240px left navigation region with accent-rail selection on every signed-in page; one flat surface; orange limited to submit, selection and focus except the Discovered link and Forwarded series.

## Why I am not satisfied

- Current overall quality 47/6 = 7.8(3), half-up 7.8; 8.0 needs an exact mean >= 7.95 (gap 0.11(6)).
- Not satisfied on the owner rule. The product did improve visibly on list pages since R3, and my R4 calibration is more lenient than R3 on 20 identical images; under neither reading does the mean reach 7.95.
- Sensitivity on my recorded coherence misread still leaves the mean below 7.95.
- Gates untested; 9/10 not achieved.

## Drift from my own round 3

Prior overall 7.26(1) (half-up 7.3); current 7.8(3) (7.8). Mean delta 103/180 = +0.57(2). Byte-identical 20: +0.41. Changed 16: +0.775. 26 files have |delta| ≥ 0.5.

Twenty byte-identical images moved by mean +0.41: that is pure reviewer calibration and shows this round reads more leniently than my R3. The 16 changed images moved more (mean +0.775) and show real, visible product changes on Blacklists/Records/Services that answer R3 findings, but the rubric gives no way to split their deltas into product versus calibration parts; no causal numbers are claimed. Dashboard (desktop light compared) changed only by uptime/chart-time/age drift. Brand fit rose on every file, including identical pixels, which is also calibration.

| File (kydns-03-after-) | Same bytes | R3 | R4 | Δ | Dim Δ | Cause | Explanation |
|---|---|---|---|---|---|---|---|
| blacklists-desktop-dark.png | no | 6.5 | 7.3 | +0.8 | hierarchy +1, typography +2, density_layout +1, color +1 | mixed | Visible product changes: header "Add list" orange->neutral ink and the source URL now breaks after "githubusercontent.com/" instead of mid "StevenBla|ck" (typography +2, color +1 partly reflect these); hierarchy +1 and density +1 fall on unchanged section structure (calibration). |
| blacklists-desktop-light.png | no | 6.5 | 7.3 | +0.8 | hierarchy +1, typography +2, density_layout +1, color +1 | mixed | Visible product changes: header "Add list" orange->neutral ink and the source URL now breaks after "githubusercontent.com/" instead of mid "StevenBla|ck" (typography +2, color +1 partly reflect these); hierarchy +1 and density +1 fall on unchanged section structure (calibration). |
| blacklists-mobile-dark.png | no | 6.5 | 7.3 | +0.8 | hierarchy +1, typography +2, density_layout +1, color +1 | mixed | Visible product changes: header "Add list" orange->neutral ink and the source URL now breaks after "githubusercontent.com/" instead of mid "StevenBla|ck" (typography +2, color +1 partly reflect these); hierarchy +1 and density +1 fall on unchanged section structure (calibration). |
| blacklists-mobile-light.png | no | 6.5 | 7.3 | +0.8 | hierarchy +1, typography +2, density_layout +1, color +1 | mixed | Visible product changes: header "Add list" orange->neutral ink and the source URL now breaks after "githubusercontent.com/" instead of mid "StevenBla|ck" (typography +2, color +1 partly reflect these); hierarchy +1 and density +1 fall on unchanged section structure (calibration). |
| dashboard-desktop-dark.png | no | 7.0 | 7.7 | +0.7 | hierarchy +1, typography +1, visible_affordances +1, coherence +1 | reviewer_calibration | Compared with the R3 desktop-light original: only natural drift is visible (uptime 2m->4m, spike x-position, last-query age); hierarchy, typography, affordances and coherence +1 each are calibration on unchanged layout, not product change. |
| dashboard-desktop-light.png | no | 7.0 | 7.7 | +0.7 | hierarchy +1, typography +1, visible_affordances +1, coherence +1 | reviewer_calibration | Compared with the R3 desktop-light original: only natural drift is visible (uptime 2m->4m, spike x-position, last-query age); hierarchy, typography, affordances and coherence +1 each are calibration on unchanged layout, not product change. |
| dashboard-mobile-dark.png | no | 7.0 | 7.7 | +0.7 | hierarchy +1, typography +1, visible_affordances +1, coherence +1 | reviewer_calibration | Not byte-identical; R3 mobile original not reopened, so any non-drift pixel change is unverified. The same +1 steps as desktop on the same layout are treated as calibration; no product improvement is claimed. |
| dashboard-mobile-light.png | no | 7.0 | 7.7 | +0.7 | hierarchy +1, typography +1, visible_affordances +1, coherence +1 | reviewer_calibration | Not byte-identical; R3 mobile original not reopened, so any non-drift pixel change is unverified. The same +1 steps as desktop on the same layout are treated as calibration; no product improvement is claimed. |
| dhcp-desktop-dark.png | yes | 7.2 | 7.8 | +0.6 | hierarchy +1, typography +1, visible_affordances +1, coherence +1 | reviewer_calibration | Byte-identical; +1 steps on unchanged form pixels are calibration. |
| dhcp-desktop-light.png | yes | 7.2 | 7.8 | +0.6 | hierarchy +1, typography +1, visible_affordances +1, coherence +1 | reviewer_calibration | Byte-identical; +1 steps on unchanged form pixels are calibration. |
| dhcp-mobile-dark.png | yes | 7.3 | 7.8 | +0.5 | typography +1, density_layout +1, visible_affordances +1 | reviewer_calibration | Byte-identical; +1 steps on unchanged form pixels are calibration. |
| dhcp-mobile-light.png | yes | 7.3 | 7.8 | +0.5 | typography +1, density_layout +1, visible_affordances +1 | reviewer_calibration | Byte-identical; +1 steps on unchanged form pixels are calibration. |
| discovered-desktop-dark.png | yes | 7.1 | 7.7 | +0.6 | typography +1, density_layout +2 | reviewer_calibration | Byte-identical; desktop density 6->8 applies the rubric legitimate-empty-state rule (honest blank space is not penalised) where R3 penalised the empty canvas; calibration. |
| discovered-desktop-light.png | yes | 7.1 | 7.7 | +0.6 | typography +1, density_layout +2 | reviewer_calibration | Byte-identical; desktop density 6->8 applies the rubric legitimate-empty-state rule (honest blank space is not penalised) where R3 penalised the empty canvas; calibration. |
| login-desktop-dark.png | yes | 8.0 | 8.7 | +0.7 | hierarchy +1, color +1, visible_affordances +1, coherence +1 | reviewer_calibration | Byte-identical; hierarchy/color/affordances/coherence 8->9 are calibration (R3 itself moved login 8.2->8.0 on identical pixels). |
| login-desktop-light.png | yes | 8.0 | 8.7 | +0.7 | hierarchy +1, color +1, visible_affordances +1, coherence +1 | reviewer_calibration | Byte-identical; hierarchy/color/affordances/coherence 8->9 are calibration (R3 itself moved login 8.2->8.0 on identical pixels). |
| login-mobile-dark.png | yes | 8.0 | 8.7 | +0.7 | hierarchy +1, color +1, visible_affordances +1, coherence +1 | reviewer_calibration | Byte-identical; hierarchy/color/affordances/coherence 8->9 are calibration (R3 itself moved login 8.2->8.0 on identical pixels). |
| login-mobile-light.png | yes | 8.0 | 8.7 | +0.7 | hierarchy +1, color +1, visible_affordances +1, coherence +1 | reviewer_calibration | Byte-identical; hierarchy/color/affordances/coherence 8->9 are calibration (R3 itself moved login 8.2->8.0 on identical pixels). |
| records-desktop-dark.png | no | 6.7 | 7.5 | +0.8 | typography +2, density_layout +1, color +1, visible_affordances +1 | mixed | Visible product changes: header link orange->neutral and "all views" cells now 13px meta (typography +2, dark color +1 partly reflect these); density and affordances +1 fall on unchanged table/form pixels (calibration). |
| records-desktop-light.png | no | 6.9 | 7.5 | +0.6 | typography +2, density_layout +1, visible_affordances +1 | mixed | Visible product changes: header link orange->neutral and "all views" cells now 13px meta (typography +2, dark color +1 partly reflect these); density and affordances +1 fall on unchanged table/form pixels (calibration). |
| records-mobile-dark.png | no | 6.7 | 7.5 | +0.8 | typography +2, color +2, coherence +1 | mixed | Visible product changes: labels now sit above values on shared tracks, Remove moved to the name row, header link neutral (typography +2, color, coherence reflect these); magnitude also includes calibration. View remains orphaned under Type. |
| records-mobile-light.png | no | 6.9 | 7.5 | +0.6 | typography +2, color +1, coherence +1 | mixed | Visible product changes: labels now sit above values on shared tracks, Remove moved to the name row, header link neutral (typography +2, color, coherence reflect these); magnitude also includes calibration. View remains orphaned under Type. |
| services-desktop-dark.png | no | 6.7 | 7.5 | +0.8 | typography +2, density_layout +1, color +1, visible_affordances +1 | mixed | Same visible changes as Records desktop (neutral header link, 13px meta cells); density and affordances +1 on unchanged pixels are calibration. |
| services-desktop-light.png | no | 6.9 | 7.5 | +0.6 | typography +2, density_layout +1, visible_affordances +1 | mixed | Same visible changes as Records desktop (neutral header link, 13px meta cells); density and affordances +1 on unchanged pixels are calibration. |
| services-mobile-dark.png | no | 6.5 | 7.7 | +1.2 | typography +2, density_layout +2, color +2, coherence +1 | mixed | Visible product changes: rows rebuilt into balanced 2x2 label-over-value tracks, Remove beside the name, neutral header link (typography, density, color, coherence reflect these); magnitude also includes calibration. |
| services-mobile-light.png | no | 6.7 | 7.7 | +1.0 | typography +2, density_layout +2, color +1, coherence +1 | mixed | Visible product changes: rows rebuilt into balanced 2x2 label-over-value tracks, Remove beside the name, neutral header link (typography, density, color, coherence reflect these); magnitude also includes calibration. |

The other 10 rows (Appearance +0.3, Discovered mobile +0.4, Settings ±0.1) are below 0.5; each has its explanation in the JSON. After the freeze I reopened 7 round-3 originals: blacklists desktop and mobile light, dashboard desktop light, records desktop and mobile dark, services mobile dark, services desktop light.

## Corrections (outside frozen rows)

- **current R4 frozen rows: coherence observations on records-desktop (2), services-desktop (2) and blacklists-desktop (2)**: Observation says select values render mono while text inputs are grotesk. Post-freeze 2x nearest-neighbour crops (/tmp, Records and Services forms, light) show text-input placeholders (printer.home.arpa., 192.168.1.50, kypost, webmail) and select values in the same mono face. Blacklists form placeholders (my-list, https://lists.example/hosts) are visibly mono in the original too. factual misread recorded; frozen vectors and observations retained unchanged. The other coherence evidence (circular type pill on Records; four pill treatments on Blacklists; second form heading at the fold on Services) stands. Arithmetic only: if those six coherence values were 8 instead of 7, the six rounded totals would each be 0.2 higher, the overall sum +6/5, and the mean 7.8(6) (half-up 7.9), still below the 7.95 exact line. This is not a rescoring.
- **my R3 finding 3 (Records placeholders switch face)**: The same crops show both Records text-input placeholders in one mono face; the parent static audit is consistent with this. My R3 claim was a misread. R3 array retained unedited.

## One bounded batch (warranted)

Progression line unmet (7.8(3) exact vs 7.95 needed) and findings 1-4 are concrete, product-local and do not touch shared tokens, routes, copy or behaviour. Scope: One bounded list-page consistency batch; no new features, no shared danger/accent override, no navigation/Settings redesign, no empty-state filler, no chart recolour.

- Records/Services desktop: render row Remove as the same quiet danger text button the phone layout uses (keep 40px target, danger hue and existing POST/confirmation).
- Blacklists: one status vocabulary - plain text state with at most one neutral pill per row; keep the amber warning only for the never-loaded/error state.
- Records phone: place View beside Type (or Value full-width with Type|View paired) so both tracks fill.
- Records desktop: move the precedence note into the form column above the submit.

Not predicted. Outcome unknown until recaptured and blind-scored. No demand for further rounds on individual screens.

## Gates, unscored and limits

- Keyboard, focus, contrast, states and responsive: all **untested**. Screenshots cannot pass them.
- Unscored: replication (4 combinations). N/A: standalone fixture hides the link and /replication 303-redirects to Dashboard; excluded from counts and means.
- Unscored: below-fold sections. Settings API tokens/SSO/backup/recovery/upstream/config; lower Services/Records/Blacklists/DHCP forms; Dashboard table remainder.
- Unscored: live integrations. replication, SSO, KyRecovery, external forwarding, blacklist fetch.
- Unscored: ordinary-user workflows. N/A: KyDNS is administrator-only.
- Unscored: behaviour, keyboard, focus, all-theme contrast, accessibility, security, production. outside static image review.
- Inputs verified by script: 148/148 artifacts match sha256, bytes and PNG IHDR (36 current, 108 historical, 4 brand); inventory equals disk glob; 9×2×2 matrix complete; 20 cross-round exact matches confirmed by hash; duplicate audit covers exactly the 306 same-size pairs (324 different-size excluded), threshold arithmetic recomputes with 0 mismatches, 0 exact/near duplicates (closest 4.30%); measured audit preserved, not rerun. Historical PNGs were hash/IHDR-read only before the freeze. `brand-references.json` purpose strings still say KyDrive (cosmetic).
- Blind access: before freeze only the allowlist plus directory listings. After freeze only my round-3 pair, round4-responses.json, 7 matched round-3 originals and 2 magnified crops of current originals. Never opened: peer pairs (including sol-round4), audits, COUNCIL, CSV/XLSX, provenance, capture checks, build-report.py, source, git log.
- Viewing: every one of the 36 current PNGs and 4 brand references opened individually at its original path with the native image tool; no contact sheets. The tool may resample internally; pixel positions are estimates.
- Schema note for the exporter: frozen rows store `total_unrounded` as a JSON number and `viewport` as a string (R3 stored `total_unrounded` as a string and `viewport` as an object); `ky_fit` has `scores`, `observations`, `total`, plus `total_unrounded`, `dimensions_used` and `not_applicable` on logins. Rows are frozen and were not reshaped.
- No security, integration, all-theme or production claims.

## Per-file scores (frozen)

| File | H | T | D | C | A | Co | Exact | Total | Ky fit |
|---|---|---|---|---|---|---|---|---|---|
| kydns-03-after-appearance-desktop-dark.png | 8 | 8 | 8 | 9 | 9 | 8 | 8.30 | 8.3 | 8.8 |
| kydns-03-after-appearance-desktop-light.png | 8 | 8 | 8 | 9 | 9 | 8 | 8.30 | 8.3 | 8.8 |
| kydns-03-after-appearance-mobile-dark.png | 8 | 8 | 8 | 9 | 9 | 8 | 8.30 | 8.3 | 8.8 |
| kydns-03-after-appearance-mobile-light.png | 8 | 8 | 8 | 9 | 9 | 8 | 8.30 | 8.3 | 8.8 |
| kydns-03-after-blacklists-desktop-dark.png | 7 | 8 | 7 | 8 | 7 | 7 | 7.30 | 7.3 | 8.4 |
| kydns-03-after-blacklists-desktop-light.png | 7 | 8 | 7 | 8 | 7 | 7 | 7.30 | 7.3 | 8.4 |
| kydns-03-after-blacklists-mobile-dark.png | 7 | 8 | 7 | 8 | 7 | 7 | 7.30 | 7.3 | 8.4 |
| kydns-03-after-blacklists-mobile-light.png | 7 | 8 | 7 | 8 | 7 | 7 | 7.30 | 7.3 | 8.4 |
| kydns-03-after-dashboard-desktop-dark.png | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-dashboard-desktop-light.png | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-dashboard-mobile-dark.png | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-dashboard-mobile-light.png | 8 | 8 | 7 | 7 | 8 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-dhcp-desktop-dark.png | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-03-after-dhcp-desktop-light.png | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-03-after-dhcp-mobile-dark.png | 7 | 8 | 8 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-03-after-dhcp-mobile-light.png | 7 | 8 | 8 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-03-after-discovered-desktop-dark.png | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-discovered-desktop-light.png | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-discovered-mobile-dark.png | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-discovered-mobile-light.png | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-03-after-login-desktop-dark.png | 9 | 8 | 8 | 9 | 9 | 9 | 8.65 | 8.7 | 8.8 |
| kydns-03-after-login-desktop-light.png | 9 | 8 | 8 | 9 | 9 | 9 | 8.65 | 8.7 | 8.8 |
| kydns-03-after-login-mobile-dark.png | 9 | 8 | 8 | 9 | 9 | 9 | 8.65 | 8.7 | 8.8 |
| kydns-03-after-login-mobile-light.png | 9 | 8 | 8 | 9 | 9 | 9 | 8.65 | 8.7 | 8.8 |
| kydns-03-after-records-desktop-dark.png | 7 | 8 | 8 | 7 | 8 | 7 | 7.50 | 7.5 | 8.4 |
| kydns-03-after-records-desktop-light.png | 7 | 8 | 8 | 7 | 8 | 7 | 7.50 | 7.5 | 8.4 |
| kydns-03-after-records-mobile-dark.png | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.4 |
| kydns-03-after-records-mobile-light.png | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.4 |
| kydns-03-after-services-desktop-dark.png | 7 | 8 | 8 | 7 | 8 | 7 | 7.50 | 7.5 | 8.4 |
| kydns-03-after-services-desktop-light.png | 7 | 8 | 8 | 7 | 8 | 7 | 7.50 | 7.5 | 8.4 |
| kydns-03-after-services-mobile-dark.png | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.4 |
| kydns-03-after-services-mobile-light.png | 7 | 8 | 8 | 8 | 7 | 8 | 7.65 | 7.7 | 8.4 |
| kydns-03-after-settings-desktop-dark.png | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-03-after-settings-desktop-light.png | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-03-after-settings-mobile-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.4 |
| kydns-03-after-settings-mobile-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.4 |
