# KyDNS Council round 2 — opus raw review

Reviewer: opus, requested `claude-opus-5-5`, actual `claude-opus-5-5`. Rubric `ky-suite-v2` pinned at ky-ui `639e588`. Evidence HEAD `3738cf3`; captured product source `bc75c30` (the later HEAD changes only docs/evidence). Raw JSON: `opus-round2.json`.

## Result

- **Overall quality (progression value): 7.6.** That is the mean of the 36 rounded per-file totals: sum 1366/5 over 36 files = 683/90 = 7.5(8).
- Mean of the unrounded vectors, for transparency: 227/30 = 7.5(6), which rounds to 7.6.
- Progression line 8.0: **not met** (it needs an exact mean of at least 7.95).
- Ky brand fit, separate and never pooled: 8.1(5), which rounds to 8.2 (round 1: 308/45 = 6.8(4)).
- **Independently satisfied: no.** The reasons are below.
- Freeze: the entire published files array (36 rows with vectors, observations, identities, dimensions, per-file totals and ky_fit) was written to disk at 2026-10-27T23:26:38+00:00. It is 84452 bytes, sha256 `ad2a1354c57230b9602f17ab0a0d0769916abe846dae755579d1204e3b318a7e`. The recipe is `json.dumps(files, ensure_ascii=False, sort_keys=True, separators=(',',':'), allow_nan=False).encode('utf-8')` with no newline. The phase-1 file was sha256 `70d6348da0cc90826781ae7a908b5cd8b18100ccbc981999f8a9acd86e7a8b24`. The published array reproduces the hash unchanged. Aggregates, drift, responses and baselines were all opened after the freeze.

## Cohorts (Ky only; no competitors, so the gap is N/A, and this is not a ranking)

| Cohort | n | Sum | Mean (exact) | Half-up |
|---|---|---|---|---|
| admin-console | 24 | 886/5 | 7.38(3) | 7.4 |
| auth | 4 | 164/5 | 8.2 | 8.2 |
| settings | 8 | 316/5 | 7.9 | 7.9 |

## Canonical surfaces (coverage, not a ranking)

| Surface | D-light | D-dark | M-light | M-dark | Mean (exact) | Half-up | Round 1 mean |
|---|---|---|---|---|---|---|---|
| appearance | 8.0 | 8.0 | 7.9 | 7.9 | 7.95 | 8.0 | 8.0 |
| blacklists | 7.3 | 7.3 | 7.4 | 7.4 | 7.35 | 7.4 | 5.3 |
| dashboard | 7.3 | 7.3 | 7.0 | 7.0 | 7.15 | 7.2 | 6.6 |
| dhcp | 7.7 | 7.7 | 7.5 | 7.5 | 7.6 | 7.6 | 6.8 |
| discovered | 7.5 | 7.5 | 7.5 | 7.5 | 7.5 | 7.5 | 5.95 |
| login | 8.2 | 8.2 | 8.2 | 8.2 | 8.2 | 8.2 | 7.9 |
| records | 7.7 | 7.7 | 7.5 | 7.5 | 7.6 | 7.6 | 6.5 |
| services | 7.0 | 7.0 | 7.2 | 7.2 | 7.1 | 7.1 | 5.45 |
| settings | 7.8 | 7.8 | 7.9 | 7.9 | 7.85 | 7.9 | 5.75 |

Desktop mean 7.6(1) and mobile mean 7.5(6). Light and dark are equal at 7.5(8). Dimension means: hierarchy 7.7(2), typography 7.(6), density_layout 7.2(7), color 7.(6), visible_affordances 7.(4), coherence 7.(6).

## Strongest remaining findings (current pixels)

1. **services, records (desktop)**: A filled header "Add service"/"Add record" and an outlined form submit carry the same label; the button that actually commits is demoted to secondary.
2. **all 16 signed-in phone captures**: The sideways nav row starts or ends on a clipped label/icon ("cklists", "ecords", partial search icon) with no fade, chevron or other scroll cue.
3. **dashboard**: The 5-column and 4-column stat rows use different grids so dividers do not align; on phones the stat grid keeps a ~38px inset (left over from the removed card) against the 16px edge, with Uptime orphaned. Counts repeat in the legend and table; orange doubles as a data hue (declined, kept as recorded).
4. **services**: Native bold "▶ Direct" disclosure marker reads like a play glyph rather than the contract chevron; the "Send DNS to the proxy" checkbox sits in the label column while its help starts in the input column (same checkbox/label-column split on DHCP and Settings).
5. **blacklists**: Narrow Name column wraps the description to eight lines (~220px row) and the mono source URL breaks mid-token; three pill variants on one page.
6. **services, records (mobile)**: Left/right label-value columns sit on staggered baselines and a lone danger Remove takes its own line per row, making Remove the loudest colour on the page.
7. **dhcp**: The guidance still opens the quoted button name with a closing quote glyph (”Fill in from this interface”); flagged in round 1 and not listed in the responses.
8. **settings desktop; discovered**: Settings fields and rules end at ~1016px while Save settings aligns to 1240px (two right edges). Discovered's only next action is an orange, non-underlined inline link.

Strengths: the Appearance picker matches the KyIdentity reference card for card. Sign-in is clean (8.2). Every signed-in page now uses one flat surface with a 240px left sidebar, line icons and accent-rail selection, and one filled orange action per view. Phone tables became labelled rows with no clipping.

## Drift from my own round 1 (implementation vs reviewer drift)

Prior overall 6.47(2) (half-up 6.5) moved to current 7.5(8) (half-up 7.6). The mean delta of rounded totals is 67/60 = 1.11(6). Splitting the unrounded weighted delta, 1.0417 per file comes from visible implementation change and 0.0889 from my re-scoring of pixels that did not change (named per row). The rubric did not change, and no competitor drift applies. I did not re-calibrate the current scores after reading round 1.

| Surface/device (both themes) | Prior | Current | Delta | Cause | Visible change |
|---|---|---|---|---|---|
| appearance desktop | 8.0 | 8.0 | +0.0 | none | Picker unchanged; shell gained line icons in nav rows and an "Administrator" label above Sign out. No score change. |
| appearance mobile | 8.0 | 7.9 | -0.1 | anchor | Picker unchanged. The baseline nav row was already clipped at the left ("ered"); this round I scored the clipped label with no scroll cue (affordances -1). Not a product regression. Reviewer-drift dims: visible_affordances. |
| blacklists desktop | 5.5 | 7.3 | +1.8 | implementation | Baseline Filtering/Lists/Add cards removed; filled orange "Turn filtering off" and row "Enable" became outlined secondaries; a single filled "Add list" now sits in the page header; orange-tinted "on"/"built-in" pills became neutral; 12px mono description became 13px sans; prose capped at ~720px. Residual: tall wrapped row and mid-token URL break. |
| blacklists mobile | 5.1 | 7.4 | +2.3 | implementation | Baseline Lists table clipped at 390px (Source cut at "raw.githu", State/actions hidden inside the card); now a labelled two-column row with Format/Entries/Last update/State visible, plus the desktop accent and flattening changes. |
| dashboard desktop | 6.7 | 7.3 | +0.6 | implementation | Card frames around both stat strips, the chart block and the metric table removed; charts are shorter; metric labels moved from mono to sans; nav icons added. Affordances +1 is reviewer drift (no interactive element changed). Reviewer-drift dims: visible_affordances. |
| dashboard mobile | 6.5 | 7.0 | +0.5 | implementation | Stat/chart cards removed and metric type changed as on desktop, but the stat grid kept its ~38px inset after the card was removed, so density did not improve. Affordances +1 is reviewer drift: the current nav row now clips an icon at the right edge where the baseline did not. Reviewer-drift dims: visible_affordances. |
| dhcp desktop | 6.9 | 7.7 | +0.8 | implementation | Status/Settings/Leases cards flattened into plain sections and nav icons added (hierarchy, coherence). Typography +1 and affordances +2 are reviewer drift: the mismatched ”Fill in from this interface” quote and the enabled-looking fields while Serve DHCP is off are visually unchanged. Reviewer-drift dims: typography, visible_affordances. |
| dhcp mobile | 6.7 | 7.5 | +0.8 | implementation | Cards flattened so fields run the full 358px column without a card inset (hierarchy, density, coherence). Typography +1 and affordances +1 are reviewer drift on unchanged copy/fields. Reviewer-drift dims: typography, visible_affordances. |
| discovered desktop | 6.5 | 7.5 | +1.0 | implementation | Card around the guidance sentence removed; "Settings" in the sentence is now a link (the next action now exists); nav icons. Typography +1 is reviewer drift (same copy and face). Reviewer-drift dims: typography. |
| discovered mobile | 5.4 | 7.5 | +2.1 | implementation | Baseline phone header stretched to ~330px with a ~150px blank band between brand row and nav; now ~113px like every other page. Card removed and Settings linked. Typography +1 is reviewer drift. Reviewer-drift dims: typography. |
| login desktop | 7.9 | 8.2 | +0.3 | implementation | Decorative card shadow removed (coherence +1). Hierarchy +1 is reviewer drift on an unchanged composition. |delta| < 0.5. Reviewer-drift dims: hierarchy. |
| login mobile | 7.9 | 8.2 | +0.3 | implementation | Same as desktop: card shadow removed; hierarchy +1 reviewer drift. Mobile baselines not reopened (|delta| < 0.5). Reviewer-drift dims: hierarchy. |
| records desktop | 7.0 | 7.7 | +0.7 | implementation | Table and form cards removed; faint pink Remove outlines became saturated danger borders; "all views" moved from mono to sans; a filled "Add record" moved to the header while the form submit became an outline. Hierarchy stays 7: the old issue was replaced by a duplicated label (filled header link vs outlined committing submit). |
| records mobile | 6.0 | 7.5 | +1.5 | implementation | Baseline table clipped the View column and every Remove at the card edge; now labelled row pairs with Remove visible, and a header primary. |
| services desktop | 6.0 | 7.0 | +1.0 | implementation | Per-row routing input, Route checkbox and three filled Saves collapsed into a native "▶ Direct" disclosure; four filled buttons above the fold became one header primary; Remove borders strengthened; cards removed. Residual: native marker, checkbox/help column split, duplicated "Add service" label. |
| services mobile | 4.9 | 7.2 | +2.3 | implementation | Baseline table clipped every row's Routing/Save/Remove and left ~108px blank rows; now labelled two-column rows with Routing and Remove visible. |
| settings desktop | 6.1 | 7.8 | +1.7 | implementation | Six bordered fieldsets in a two-column masonry inside a card became one column of plain groups with rule dividers; Save settings is now visible at the top. |
| settings mobile | 5.4 | 7.9 | +2.5 | implementation | Baseline fieldsets ran past the card's right edge with no Save in view; now contained full-width single column with Save settings beside the section heading. |

Light and dark rows have identical deltas. I re-opened 30 baselines: both themes for every file with |delta| ≥ 0.5, plus Appearance-mobile-light and Login-desktop-light. The other six have |delta| < 0.5 and were not re-opened.

Responses audit (post-freeze, did not change any score): every listed response is visible in the captures except two. First, the header add-link primary created the same-label pair in finding 1. Second, the DHCP quote glyph (finding 7) is unaddressed. The declined items are recorded as declined and not re-argued: DHCP fields stay editable, Forwarded keeps its hue, metrics are retained, and no fictional username is added. The below-fold phone Settings scroller is outside the captured viewport and unscored.

## One bounded batch (recommended, because the gate is not met and the defects are concrete)

- List pages: make the form submit the visible committing action (e.g. header link quiet/secondary or anchor-styled, submit filled), or otherwise remove the same-label primary/secondary pair. Ask the owner before relabelling.
- Phone nav: add a scroll cue (edge fade or chevron) to the nav row so clipped labels read as scrollable.
- Shared form grid: put checkbox rows and help text on the input column (Services, DHCP, Settings); replace the native disclosure marker with the contract chevron on Services routing.
- Dashboard: one grid for both stat rows; remove the leftover phone stat inset so stats align to the 16px edge.
- Blacklists: give the Name/description column more width or clamp it, and break URLs at "/" rather than mid-token.
- Fix the DHCP opening quote glyph; cap Settings Save to the form column edge.

Whether this reaches 8.0 is unknown until recaptured and blind-scored.

## Gates, unscored and limits

- Keyboard, focus, contrast, states and responsive are **untested**. Visible pixels and partial captures cannot pass them.
- Unscored: replication (4 combinations). N/A: standalone fixture hides the link and /replication 303-redirects to Dashboard; no duplicate Dashboard captured or scored.
- Unscored: below-fold sections. Settings API tokens, SSO, backup/recovery, upstream/config sections; lower forms on Services/Records/Blacklists/DHCP; Dashboard table remainder.
- Unscored: live integrations. real replication, SSO, KyRecovery, external forwarding and blacklist fetch.
- Unscored: ordinary-user workflows. KyDNS is administrator-only.
- Unscored: behaviour, keyboard, all-theme contrast, accessibility, security, production. outside static review.
- Input verification: all 76 artifacts (36 current, 36 historical, 4 brand) match on sha256, bytes and PNG IHDR dimensions. Inventory, current role and disk glob agree, the 9×2×2 matrix is complete, and all 76 hashes are distinct. The duplicate audit covers exactly the 306 same-size pairs of 630. I recomputed the threshold logic for all 306 with 0 mismatches. There are no exact or near duplicates, and the closest pair differs on 4.2% of pixels. I did not repeat the pixel diff. Historical images were only hashed and IHDR-read before the freeze. brand-references.json purpose strings say "KyDrive"; artifacts-v2.json says KyDNS. Cosmetic; hashes match.
- Blind access: before the freeze I opened only the allowlist. The extra steps were HEAD and status in the worktree, sha256 of the pinned ky-ui objects (content not shown), and a directory listing that showed file names only. I never opened a peer review, the disagreement audit, COUNCIL, ratings, workbook, provenance, plan, setup or capture checks, or the git log.
- Viewing: I opened each PNG individually with the native image tool at its original path. I cannot prove from inside the tool whether it resamples. Pixel positions are visual estimates.
- Live, security and production assurance are outside this review.

## Per-file scores (frozen)

| File | H | T | D | C | A | Co | Exact | Total | Ky fit |
|---|---|---|---|---|---|---|---|---|---|
| kydns-01-after-appearance-desktop-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.6 |
| kydns-01-after-appearance-desktop-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.6 |
| kydns-01-after-appearance-mobile-dark.png | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 8.6 |
| kydns-01-after-appearance-mobile-light.png | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 8.6 |
| kydns-01-after-blacklists-desktop-dark.png | 8 | 7 | 6 | 8 | 8 | 7 | 7.30 | 7.3 | 8.0 |
| kydns-01-after-blacklists-desktop-light.png | 8 | 7 | 6 | 8 | 8 | 7 | 7.30 | 7.3 | 8.0 |
| kydns-01-after-blacklists-mobile-dark.png | 8 | 7 | 7 | 8 | 7 | 7 | 7.35 | 7.4 | 8.0 |
| kydns-01-after-blacklists-mobile-light.png | 8 | 7 | 7 | 8 | 7 | 7 | 7.35 | 7.4 | 8.0 |
| kydns-01-after-dashboard-desktop-dark.png | 7 | 8 | 7 | 7 | 8 | 7 | 7.30 | 7.3 | 7.4 |
| kydns-01-after-dashboard-desktop-light.png | 7 | 8 | 7 | 7 | 8 | 7 | 7.30 | 7.3 | 7.4 |
| kydns-01-after-dashboard-mobile-dark.png | 7 | 8 | 6 | 7 | 7 | 7 | 6.95 | 7.0 | 7.2 |
| kydns-01-after-dashboard-mobile-light.png | 7 | 8 | 6 | 7 | 7 | 7 | 6.95 | 7.0 | 7.2 |
| kydns-01-after-dhcp-desktop-dark.png | 8 | 7 | 7 | 8 | 8 | 8 | 7.65 | 7.7 | 8.4 |
| kydns-01-after-dhcp-desktop-light.png | 8 | 7 | 7 | 8 | 8 | 8 | 7.65 | 7.7 | 8.4 |
| kydns-01-after-dhcp-mobile-dark.png | 7 | 7 | 8 | 8 | 7 | 8 | 7.50 | 7.5 | 8.4 |
| kydns-01-after-dhcp-mobile-light.png | 7 | 7 | 8 | 8 | 7 | 8 | 7.50 | 7.5 | 8.4 |
| kydns-01-after-discovered-desktop-dark.png | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.0 |
| kydns-01-after-discovered-desktop-light.png | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.0 |
| kydns-01-after-discovered-mobile-dark.png | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.0 |
| kydns-01-after-discovered-mobile-light.png | 7 | 8 | 7 | 8 | 7 | 8 | 7.45 | 7.5 | 8.0 |
| kydns-01-after-login-desktop-dark.png | 9 | 8 | 8 | 8 | 8 | 8 | 8.20 | 8.2 | 8.5 |
| kydns-01-after-login-desktop-light.png | 9 | 8 | 8 | 8 | 8 | 8 | 8.20 | 8.2 | 8.5 |
| kydns-01-after-login-mobile-dark.png | 9 | 8 | 8 | 8 | 8 | 8 | 8.20 | 8.2 | 8.5 |
| kydns-01-after-login-mobile-light.png | 9 | 8 | 8 | 8 | 8 | 8 | 8.20 | 8.2 | 8.5 |
| kydns-01-after-records-desktop-dark.png | 7 | 8 | 8 | 7 | 8 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-01-after-records-desktop-light.png | 7 | 8 | 8 | 7 | 8 | 8 | 7.65 | 7.7 | 8.2 |
| kydns-01-after-records-mobile-dark.png | 8 | 8 | 7 | 7 | 7 | 8 | 7.50 | 7.5 | 8.2 |
| kydns-01-after-records-mobile-light.png | 8 | 8 | 7 | 7 | 7 | 8 | 7.50 | 7.5 | 8.2 |
| kydns-01-after-services-desktop-dark.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 8.0 |
| kydns-01-after-services-desktop-light.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 8.0 |
| kydns-01-after-services-mobile-dark.png | 8 | 7 | 7 | 7 | 7 | 7 | 7.20 | 7.2 | 8.0 |
| kydns-01-after-services-mobile-light.png | 8 | 7 | 7 | 7 | 7 | 7 | 7.20 | 7.2 | 8.0 |
| kydns-01-after-settings-desktop-dark.png | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-01-after-settings-desktop-light.png | 8 | 8 | 7 | 8 | 8 | 8 | 7.80 | 7.8 | 8.4 |
| kydns-01-after-settings-mobile-dark.png | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 8.4 |
| kydns-01-after-settings-mobile-light.png | 8 | 8 | 8 | 8 | 7 | 8 | 7.85 | 7.9 | 8.4 |
