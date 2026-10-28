# KyDNS Council round 3: opus raw review

Reviewer: opus. Requested model `claude-opus-5-5`; actual model `claude-opus-5-5` (Opus 5.5). Rubric `ky-suite-v2`, pinned at ky-ui `639e588`, and all five authority hashes match. Evidence HEAD `49bec51`; captured product source `fde2314` (x/net 0.62.0 from main). The later commit changes evidence only. Raw JSON: `opus-round3.json`.

## Result

- **Overall quality (progression value): 7.3.** Mean of the 36 rounded per-file totals: sum 1307/5 / 36 = 1307/180 = 7.26(1).
- Unrounded-vector mean (for transparency): 869/120 = 7.241(6), which rounds half-up to 7.2.
- **Progression line 8.0: not met.** It needs an exact mean of at least 7.95, and the gap is 31/45 = 0.6(8).
- Ky brand fit (separate, never pooled with quality): 67/9 = 7.(4), which rounds half-up to 7.4. Shell is N/A on the 4 signed-out logins and excluded from their denominators.
- **Independently satisfied: no** (reasons below). 9/10 is not achieved. Gemini was not used; its quota is exhausted, and that does not block progression.
- **Freeze.** I wrote the entire files array to disk at 2026-10-27T23:57:38+00:00 before reading any prior or response file. It holds 36 rows: identities, dimensions, integer vectors, observations, ky_fit vectors and reasons, and exact and rounded totals. The canonical serialization is 71914 bytes, sha256 `65507450c3b6023eba3eb8fa47a1c8261623b853411ef9f0bcfc23a24078ae52`, using `json.dumps(files, ensure_ascii=False, sort_keys=True, separators=(',',':'), allow_nan=False).encode('utf-8')` with no trailing newline. The stage-1 file sha256 was `52646dc9cb5d4a22309d6cb609169a02696836c8ffae097033600ccfe592dcbd`. The published array reproduces the hash unchanged, and no vector or observation was revised after the freeze.

## Cohorts (Ky only; no competitors, so the gap is N/A; not a ranking)

| Cohort | n | Sum | Mean (exact) | Half-up |
|---|---|---|---|---|
| admin-console | 24 | 829/5 | 6.908(3) | 6.9 |
| auth | 4 | 32 | 8 | 8.0 |
| settings | 8 | 318/5 | 7.95 | 8.0 |

## Canonical surfaces (coverage, not a ranking)

| Surface | D-light | D-dark | M-light | M-dark | Mean (exact) | Half-up | My R2 mean |
|---|---|---|---|---|---|---|---|
| appearance | 8.0 | 8.0 | 8.0 | 8.0 | 8 | 8.0 | 7.95 |
| blacklists | 6.5 | 6.5 | 6.5 | 6.5 | 6.5 | 6.5 | 7.35 |
| dashboard | 7.0 | 7.0 | 7.0 | 7.0 | 7 | 7.0 | 7.15 |
| dhcp | 7.2 | 7.2 | 7.3 | 7.3 | 7.25 | 7.3 | 7.6 |
| discovered | 7.1 | 7.1 | 7.3 | 7.3 | 7.2 | 7.2 | 7.5 |
| login | 8.0 | 8.0 | 8.0 | 8.0 | 8 | 8.0 | 8.2 |
| records | 6.9 | 6.7 | 6.9 | 6.7 | 6.8 | 6.8 | 7.6 |
| services | 6.9 | 6.7 | 6.7 | 6.5 | 6.7 | 6.7 | 7.1 |
| settings | 7.9 | 7.9 | 7.9 | 7.9 | 7.9 | 7.9 | 7.85 |

Device means: desktop 7.2(5), mobile 7.2(6). Theme means: light 7.28(3), dark 7.23(8) (dark is lower only because of the danger/accent hue reading on Services and Records). Dimension means: hierarchy 7.(2), typography 6.(8), density_layout 7.(1), color 7.(4), visible_affordances 7.(3), coherence 7.5.

Brand per dimension: palette_tokens 7.(8) (n=36), type_geometry 7.(6) (n=36), shell 7 (n=32), accent_discipline 7.(1) (n=36), suite_cohesion 7.(4) (n=36).

## Strongest remaining findings (current pixels)

1. **services, records, blacklists (all)**: Header jump links are orange underlined text, so each list page shows two accent marks (link + filled submit); on Blacklists the link is the only orange above the fold.
2. **services, records (dark)**: Danger salmon Remove (outline on desktop, text on phone) sits beside the orange link/button at nearly the same hue; destructive and primary blur at a glance.
3. **services, records**: Rows mix 14px mono values with 15px sans 'all views' on a lower baseline; Records placeholders switch face (Name mono, Value sans); the Records precedence note sits at x=280 outside the form column.
4. **services, records (mobile)**: Two-column label/value rows sit on staggered baselines and Remove floats below its row.
5. **blacklists**: Source URL still breaks mid-token (StevenBla|ck); one row mixes four type styles; four equal outlined buttons with no visible primary above the fold.
6. **dashboard**: Counts repeat across KPI strip, legend and table; orange doubles as the Forwarded series hue (declined and retained by owner; recorded, not re-argued).
7. **signed-in mobile (16)**: The 'Scroll navigation' caption reads like a product tagline in the wordmark slot; it answers the cue finding but differs from the KyIdentity header (subtitle/account).
8. **settings**: Three stacked headings (Settings / Server settings / Zone) precede the first field.

Strengths: Sign-in and the Appearance picker are clean (8.0); the picker matches the KyIdentity reference card for card. Every signed-in page uses one 240px left navigation region with accent-rail selection on one flat surface. Settings now has one clear Save that aligns to the field edge (7.9).

## Why I am not satisfied

- Current overall quality 1307/180 = 7.26(1), half-up 7.3; the 8.0 line needs an exact mean of at least 7.95.
- The fde2314 changes visibly answer most of my round-2 findings (submits now primary, quiet phone Remove, chevron disclosure, checkbox/help in the input column, DHCP quote glyph, Settings Save edge, scroll caption). My lower scores fall mainly on pixels that did not change, so round-over-round movement (-0.3) is reviewer variation, not regression. Under either round's calibration the gate is unmet (round 2 7.6, round 3 7.3).
- Remaining defects are concrete and repeated across list pages: orange header jump links outside the accent roles, mixed mono/sans cell sizes and placeholders, staggered phone label grids, mid-token URL breaks, and dark-theme danger/accent hues that read nearly alike.
- 9/10 is not achieved; behaviour gates untested.

## Drift from my own round 2 (reviewer variation vs changed pixels)

My prior overall was 7.5(8) (half-up 7.6). The current overall is 7.26(1) (half-up 7.3). The mean delta of rounded totals is -59/180 = -0.32(7). On the six byte-identical images, the mean delta is -0.1(3): each login moved -0.2 (hierarchy 9 to 8) and each desktop Appearance moved 0. That movement is pure reviewer variation. The rubric did not change, and there is no competitor drift. I did not recalibrate after reading round 2.

Most negative movement falls on dimensions whose pixels did not change. My round-3 scoring is stricter than my round-2 scoring; the product did not regress. The visible changes moved in the direction I asked for in round 2, and I did not reward several of them, such as the shorter Blacklists row and the aligned DHCP checkboxes. Cause labels are judgment bookkeeping, not isolated causal measurements.

Every |delta| >= 0.5 (light and dark listed separately where they differ):

| File | R2 | R3 | Delta | Dim deltas | Cause | Explanation |
|---|---|---|---|---|---|---|
| blacklists-desktop-dark.png | 7.3 | 6.5 | -0.8 | hierarchy -2, typography -1, color -1, visible_affordances -1 | mixed | Changed pixels: filled header 'Add list' -> orange link, so no filled primary remains above the fold (hierarchy 8->6 is partly this change), and Name/description/URL merged into one column (row ~220px -> ~150px, not rewarded in density). Typography 7->6, colour 8->7 and affordances 8->7 fell on the unchanged URL break, pills and outlined buttons (reviewer variation). |
| blacklists-desktop-light.png | 7.3 | 6.5 | -0.8 | hierarchy -2, typography -1, color -1, visible_affordances -1 | mixed | Changed pixels: filled header 'Add list' -> orange link, so no filled primary remains above the fold (hierarchy 8->6 is partly this change), and Name/description/URL merged into one column (row ~220px -> ~150px, not rewarded in density). Typography 7->6, colour 8->7 and affordances 8->7 fell on the unchanged URL break, pills and outlined buttons (reviewer variation). |
| blacklists-mobile-dark.png | 7.4 | 6.5 | -0.9 | hierarchy -2, typography -1, density_layout -1, color -1 | mixed | Changed pixels: header button -> link and the 'Source' label dropped with the URL under the description. Hierarchy 8->6 is partly the lost filled primary; typography, density and colour each -1 on unchanged URL-break, label-grid and pill pixels (reviewer variation). |
| blacklists-mobile-light.png | 7.4 | 6.5 | -0.9 | hierarchy -2, typography -1, density_layout -1, color -1 | mixed | Changed pixels: header button -> link and the 'Source' label dropped with the URL under the description. Hierarchy 8->6 is partly the lost filled primary; typography, density and colour each -1 on unchanged URL-break, label-grid and pill pixels (reviewer variation). |
| dhcp-desktop-dark.png | 7.7 | 7.2 | -0.5 | hierarchy -1, visible_affordances -1, coherence -1 | reviewer_variation | Changed pixels: checkboxes moved into the input column, the opening quote glyph fixed, Save moved down a row. These are what I asked for in round 2; hierarchy, affordances and coherence each 8->7 fell on unchanged form pixels, and my round-3 density note even cites the requested checkbox placement. Reviewer variation. |
| dhcp-desktop-light.png | 7.7 | 7.2 | -0.5 | hierarchy -1, visible_affordances -1, coherence -1 | reviewer_variation | Changed pixels: checkboxes moved into the input column, the opening quote glyph fixed, Save moved down a row. These are what I asked for in round 2; hierarchy, affordances and coherence each 8->7 fell on unchanged form pixels, and my round-3 density note even cites the requested checkbox placement. Reviewer variation. |
| records-desktop-dark.png | 7.7 | 6.7 | -1.0 | typography -2, density_layout -1, color -1, visible_affordances -1, coherence -1 | reviewer_variation | Only changed pixels: filled header 'Add record' -> orange underlined link and the outlined submit -> filled (fixes my round-2 finding 1). Typography 8->6, density 8->7, affordances 8->7 and coherence 8->7 fell on unchanged table, placeholder and note pixels; dark colour 7->6 is a new reading of unchanged hues. Reviewer variation, not a regression. |
| records-desktop-light.png | 7.7 | 6.9 | -0.8 | typography -2, density_layout -1, visible_affordances -1, coherence -1 | reviewer_variation | Only changed pixels: filled header 'Add record' -> orange underlined link and the outlined submit -> filled (fixes my round-2 finding 1). Typography 8->6, density 8->7, affordances 8->7 and coherence 8->7 fell on unchanged table, placeholder and note pixels; dark colour 7->6 is a new reading of unchanged hues. Reviewer variation, not a regression. |
| records-mobile-dark.png | 7.5 | 6.7 | -0.8 | hierarchy -1, typography -2, color -1, coherence -1 | reviewer_variation | Changed pixels: header button -> link, outlined Remove -> red text, caption. Hierarchy 8->7, typography 8->6 and coherence 8->7 fell on unchanged row/label pixels; dark colour 7->6 as on desktop. Reviewer variation. |
| records-mobile-light.png | 7.5 | 6.9 | -0.6 | hierarchy -1, typography -2, coherence -1 | reviewer_variation | Changed pixels: header button -> link, outlined Remove -> red text, caption. Hierarchy 8->7, typography 8->6 and coherence 8->7 fell on unchanged row/label pixels; dark colour 7->6 as on desktop. Reviewer variation. |
| services-mobile-dark.png | 7.2 | 6.5 | -0.7 | hierarchy -1, typography -1, density_layout -1, color -1 | reviewer_variation | Changed pixels: filled header button -> orange underlined link, outlined Remove -> red text, play marker -> chevron, 'Scroll navigation' caption. These answer my round-2 findings 1, 4 and 6, yet hierarchy 8->7, typography 7->6 and density 7->6 fell on the unchanged staggered two-column rows I already flagged in round 2; dark colour 7->6 is a new reading of unchanged danger/accent hues. The drop is reviewer variation, not a regression. |
| services-mobile-light.png | 7.2 | 6.7 | -0.5 | hierarchy -1, typography -1, density_layout -1 | reviewer_variation | Changed pixels: filled header button -> orange underlined link, outlined Remove -> red text, play marker -> chevron, 'Scroll navigation' caption. These answer my round-2 findings 1, 4 and 6, yet hierarchy 8->7, typography 7->6 and density 7->6 fell on the unchanged staggered two-column rows I already flagged in round 2; dark colour 7->6 is a new reading of unchanged danger/accent hues. The drop is reviewer variation, not a regression. |

The other 24 rows have |delta| < 0.5; each row has its explanation in the JSON. After the freeze I reopened 8 round-2 originals: blacklists desktop and mobile light, records desktop light and dark, records mobile light, services mobile light and dark, and dhcp desktop light.

## One bounded batch (warranted: gate unmet, defects concrete)

- Header jump links: neutral ink, underlined tertiary links, so orange marks only the submit, selection and focus.
- Row typography: render 'all views'/View cells at the same size and baseline as the mono values (or both meta 13px); give Records placeholders one face; move the precedence note into the form column.
- Phone label grids (Services/Records/Blacklists): one row track per label pair so left/right baselines align; Remove on the last row's baseline.
- Blacklists URL: allow breaks only after '/' (e.g. <wbr> after slashes) instead of mid-token.
- Dark danger vs accent: raise upstream in ky-ui (shared token), not a product override; out of this product batch.

Even a clean execution is unlikely to add the +0.69 mean needed for 7.95 under this round's scoring; the outcome is unknown until recaptured and blind-scored. I am not demanding new rounds for individual screens.

## Gates, unscored and limits

- Keyboard, focus, contrast, states and responsive are all **untested**. Static and partial captures cannot pass them.
- Unscored: replication (4 combinations). N/A: standalone fixture hides the link and /replication 303-redirects to Dashboard; no duplicate Dashboard scored.
- Unscored: below-fold sections. Settings API tokens/SSO/backup/upstream/config; lower forms on Services/Records/Blacklists/DHCP; Dashboard table remainder; phone Settings token scroller.
- Unscored: live integrations. replication, SSO, KyRecovery, forwarding, blacklist fetch.
- Unscored: ordinary-user workflows. N/A: KyDNS is administrator-only.
- Unscored: behaviour, keyboard, all-theme contrast, accessibility, security, production. outside static review.
- Input verification: all 112 artifacts (36 current, 72 historical, 4 brand) match on sha256, bytes and PNG IHDR. Inventory and the disk glob agree, the 9x2x2 matrix is complete, and there are 36 distinct current hashes. The six round-2 identities were confirmed by hash; round 1 has none. The duplicate audit covers exactly the 306 same-size pairs (324 different-size pairs excluded). The threshold logic recomputes with 0 mismatches. There are no exact or near duplicates (closest pair 4.32%), and I did not repeat the pixel diff. Historical images were only hashed and IHDR-read before the freeze. `brand-references.json` purpose strings still say KyDrive, which is cosmetic.
- Blind access: before the freeze I read only the allowlist, plus the worktree HEAD and status and a directory listing. After the freeze I read only round3-responses.json, my own round-2 pair and 8 matched round-2 originals. I never opened peer pairs, round 1, audits, COUNCIL, CSV/XLSX, provenance, capture checks or the git log.
- Viewing: I opened all 36 current PNGs and the 4 brand references individually at their original paths with the native image tool, with no contact sheets. I also made two magnified crops (in /tmp) of the current title and wordmark to check the typeface; they are all Space Grotesk, so I raised no finding. I cannot prove from inside the tool whether it resamples, and pixel positions are estimates.
- No security, integration, all-theme or production claims.

## Per-file scores (frozen)

| File | H | T | D | C | A | Co | Exact | Total | Ky fit |
|---|---|---|---|---|---|---|---|---|---|
| kydns-02-after-appearance-desktop-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-appearance-desktop-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-appearance-mobile-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-appearance-mobile-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-blacklists-desktop-dark.png | 6 | 6 | 6 | 7 | 7 | 7 | 6.45 | 6.5 | 7.2 |
| kydns-02-after-blacklists-desktop-light.png | 6 | 6 | 6 | 7 | 7 | 7 | 6.45 | 6.5 | 7.2 |
| kydns-02-after-blacklists-mobile-dark.png | 6 | 6 | 6 | 7 | 7 | 7 | 6.45 | 6.5 | 7.2 |
| kydns-02-after-blacklists-mobile-light.png | 6 | 6 | 6 | 7 | 7 | 7 | 6.45 | 6.5 | 7.2 |
| kydns-02-after-dashboard-desktop-dark.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 7.0 |
| kydns-02-after-dashboard-desktop-light.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 7.0 |
| kydns-02-after-dashboard-mobile-dark.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 7.0 |
| kydns-02-after-dashboard-mobile-light.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 7.0 |
| kydns-02-after-dhcp-desktop-dark.png | 7 | 7 | 7 | 8 | 7 | 7 | 7.15 | 7.2 | 7.6 |
| kydns-02-after-dhcp-desktop-light.png | 7 | 7 | 7 | 8 | 7 | 7 | 7.15 | 7.2 | 7.6 |
| kydns-02-after-dhcp-mobile-dark.png | 7 | 7 | 7 | 8 | 7 | 8 | 7.30 | 7.3 | 7.6 |
| kydns-02-after-dhcp-mobile-light.png | 7 | 7 | 7 | 8 | 7 | 8 | 7.30 | 7.3 | 7.6 |
| kydns-02-after-discovered-desktop-dark.png | 7 | 7 | 6 | 8 | 7 | 8 | 7.10 | 7.1 | 7.4 |
| kydns-02-after-discovered-desktop-light.png | 7 | 7 | 6 | 8 | 7 | 8 | 7.10 | 7.1 | 7.4 |
| kydns-02-after-discovered-mobile-dark.png | 7 | 7 | 7 | 8 | 7 | 8 | 7.30 | 7.3 | 7.4 |
| kydns-02-after-discovered-mobile-light.png | 7 | 7 | 7 | 8 | 7 | 8 | 7.30 | 7.3 | 7.4 |
| kydns-02-after-login-desktop-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-login-desktop-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-login-mobile-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-login-mobile-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-02-after-records-desktop-dark.png | 7 | 6 | 7 | 6 | 7 | 7 | 6.70 | 6.7 | 7.0 |
| kydns-02-after-records-desktop-light.png | 7 | 6 | 7 | 7 | 7 | 7 | 6.85 | 6.9 | 7.0 |
| kydns-02-after-records-mobile-dark.png | 7 | 6 | 7 | 6 | 7 | 7 | 6.70 | 6.7 | 7.0 |
| kydns-02-after-records-mobile-light.png | 7 | 6 | 7 | 7 | 7 | 7 | 6.85 | 6.9 | 7.0 |
| kydns-02-after-services-desktop-dark.png | 7 | 6 | 7 | 6 | 7 | 7 | 6.70 | 6.7 | 7.0 |
| kydns-02-after-services-desktop-light.png | 7 | 6 | 7 | 7 | 7 | 7 | 6.85 | 6.9 | 7.0 |
| kydns-02-after-services-mobile-dark.png | 7 | 6 | 6 | 6 | 7 | 7 | 6.50 | 6.5 | 7.0 |
| kydns-02-after-services-mobile-light.png | 7 | 6 | 6 | 7 | 7 | 7 | 6.65 | 6.7 | 7.0 |
| kydns-02-after-settings-desktop-dark.png | 8 | 7 | 8 | 8 | 8 | 8 | 7.85 | 7.9 | 7.8 |
| kydns-02-after-settings-desktop-light.png | 8 | 7 | 8 | 8 | 8 | 8 | 7.85 | 7.9 | 7.8 |
| kydns-02-after-settings-mobile-dark.png | 8 | 7 | 8 | 8 | 8 | 8 | 7.85 | 7.9 | 7.8 |
| kydns-02-after-settings-mobile-light.png | 8 | 7 | 8 | 8 | 8 | 8 | 7.85 | 7.9 | 7.8 |
