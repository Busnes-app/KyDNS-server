# KyDNS Council round 1 — opus raw review

Reviewer: opus, requested `claude-opus-5-5`, actual `claude-opus-5-5` (Claude Opus 5.5). Rubric `ky-suite-v2` pinned at ky-ui `639e588`. Evidence HEAD `ce4d392`, captured HEAD `8e41fae`, product source `5d27478`. Raw JSON: `opus-round1.json`.

## Result

- **Overall quality (progression value): 6.5.** This is the mean of the 36 rounded per-file totals, exactly 233/36 = 6.47(2).
- Mean of the unrounded vectors, for transparency: (2317/10)/36 = 2317/360 = 6.436(1) -> 6.4.
- Progression threshold 8.0: **not met**.
- Ky brand fit, reported separately and never pooled into quality: 6.8(4) -> 6.8 (per-file mean, n=36).
- **Satisfied: no.**
- Freeze: the entire files array was frozen at 2026-10-27T22:57:43+00:00, sha256 `aa332670630f89efa0f61bc3e9f541084024e5bc7fc75cd46ecc608b468b368b`, 78237 bytes, using `json.dumps(files, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode('utf-8')` before any total or mean was computed.

## Cohorts (Ky only; no competitors captured, so the gap is N/A)

| Cohort | n | Sum | Mean (exact) | Half-up |
|---|---|---|---|---|
| admin-console | 24 | 732/5 | 6.1 | 6.1 |
| auth | 4 | 158/5 | 7.9 | 7.9 |
| settings | 8 | 55 | 6.875 | 6.9 |

## Surfaces (not a ranking)

| Surface | D-light | D-dark | M-light | M-dark | Mean |
|---|---|---|---|---|---|
| appearance | 8.0 | 8.0 | 8.0 | 8.0 | 8 |
| blacklists | 5.5 | 5.5 | 5.1 | 5.1 | 5.3 |
| dashboard | 6.7 | 6.7 | 6.5 | 6.5 | 6.6 |
| dhcp | 6.9 | 6.9 | 6.7 | 6.7 | 6.8 |
| discovered | 6.5 | 6.5 | 5.4 | 5.4 | 5.95 |
| login | 7.9 | 7.9 | 7.9 | 7.9 | 7.9 |
| records | 7.0 | 7.0 | 6.0 | 6.0 | 6.5 |
| services | 6.0 | 6.0 | 4.9 | 4.9 | 5.45 |
| settings | 6.1 | 6.1 | 5.4 | 5.4 | 5.75 |

Desktop mean 6.7(3), mobile mean 6.2(1). Light and dark are equal (6.47(2) / 6.47(2)).

## Strongest visible defects

1. **Mobile tables and fieldsets clip at the card edge (390px).** Services hides every row's Routing/Save/Remove and leaves ~108px-tall empty rows. Records cuts the View column and Remove. Blacklists cuts Source at `raw.githu` and hides the State and actions columns. Settings fieldsets run past the card's right edge. Nothing on screen says the content scrolls sideways.
2. **Discovered mobile header stretched to ~330px.** The brand row and nav row are pulled apart by a ~150px blank band, and the footer line floats mid-page. Every other mobile capture has a ~112px header.
3. **Too many filled accent buttons.** Services shows four filled Saves and Add service above the fold. Blacklists fills both 'Turn filtering off' and 'Enable', and tints the decorative 'on' and 'built-in' pills orange.
4. **Settings nests boxes.** Six fieldset frames with legends sit inside the Server settings card. The two-column masonry leaves uneven gaps, and no Save is visible.
5. **Dashboard spends the viewport on blank charts and repeated counters.** The Cache hit rate chart is empty and Response time shows a single dot. A second KPI strip and a mono Metric/Value table repeat the same counts. The orange 'Forwarded' series reuses the accent hue.
6. **Smaller slips.** Light-theme Remove outlines are faint pink. Mono is used for prose labels and 12px Blacklists descriptions. The DHCP copy uses a closing quote at both ends (`”Fill in from this interface”`), and DHCP fields look enabled while 'Serve DHCP' is off. The Discovered empty state names Settings without linking to it. The login card has a decorative shadow. The stamp icon loses its tile in dark theme.

Strengths: the Appearance picker (8.0 on all four captures) matches the KyIdentity reference geometry. Sign-in is clean (7.9). The desktop shell already has the 240px sidebar, the accent-rail selection and a single left navigation region.

## One bounded fix batch (no new features)

- **Responsive containment.** Below 860px, turn the Services, Records and Blacklists tables into stacked rows (name and value first, row actions on their own line) or a clearly scrollable container that stays inside the card. Make Settings fieldsets fit the card width.
- **Header.** Fix the mobile header so it never stretches to fill short pages (Discovered).
- **Accent discipline.** Keep one filled primary per view. Make per-row Save and the Blacklists list 'Enable' secondary outlines, make 'Turn filtering off' a secondary or toggle-style control, and use neutral pills for 'on' and 'built-in'.
- **Settings.** Flatten the fieldsets into plain section headings on the one surface, stack the sections in a single column with labels above, and keep the existing Save reachable.
- **Copy and type.** Use Grotesk for prose labels (keep mono for addresses and FQDNs), raise the 12px descriptions to 13px, fix the DHCP quote pair, and link 'Settings' in the Discovered empty state.
- **Shell polish, toward the KyIdentity refs.** Pin the signed-in name and role above Sign out. Keep the stamp tile in dark theme.

## Gates and limits

- Keyboard, focus, contrast, states and responsive are **untested**: screenshots cannot pass them. The visible 390px clipping suggests responsive would fail.
- Replication (4 combinations) is N/A: the standalone fixture hides the link and redirects with 303 to Dashboard. Below-fold sections are unscored, and no ordinary-user flows exist.
- The images were viewed one at a time through the native image Read tool on the original PNG paths. Whether that viewer resamples internally cannot be proven from inside the tool. Pixel positions such as `x~372` are visual estimates, not measurements.
- Input verification: all 36+4 hashes, byte counts and IHDR dimensions match, and the inventory, artifacts and disk sets agree exactly. Duplicate-audit membership (306 of 630 pairs) and the threshold logic were rechecked, with no exact or near duplicates. The brand-references.json purpose strings say 'KyDrive', a label slip.
- Drift is N/A for round 1. No historical, peer or parent files were opened.

## Per-file scores

| File | H | T | D | C | A | Co | Total (exact) | Total | Ky fit |
|---|---|---|---|---|---|---|---|---|---|
| kydns-00-before-appearance-desktop-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-00-before-appearance-desktop-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-00-before-appearance-mobile-dark.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-00-before-appearance-mobile-light.png | 8 | 8 | 8 | 8 | 8 | 8 | 8.00 | 8.0 | 8.0 |
| kydns-00-before-blacklists-desktop-dark.png | 5 | 5 | 5 | 5 | 7 | 6 | 5.45 | 5.5 | 5.6 |
| kydns-00-before-blacklists-desktop-light.png | 5 | 5 | 5 | 5 | 7 | 6 | 5.45 | 5.5 | 5.6 |
| kydns-00-before-blacklists-mobile-dark.png | 5 | 5 | 4 | 5 | 6 | 6 | 5.10 | 5.1 | 5.6 |
| kydns-00-before-blacklists-mobile-light.png | 5 | 5 | 4 | 5 | 6 | 6 | 5.10 | 5.1 | 5.6 |
| kydns-00-before-dashboard-desktop-dark.png | 7 | 7 | 6 | 7 | 7 | 6 | 6.65 | 6.7 | 6.2 |
| kydns-00-before-dashboard-desktop-light.png | 7 | 7 | 6 | 7 | 7 | 6 | 6.65 | 6.7 | 6.2 |
| kydns-00-before-dashboard-mobile-dark.png | 7 | 7 | 6 | 7 | 6 | 6 | 6.50 | 6.5 | 6.2 |
| kydns-00-before-dashboard-mobile-light.png | 7 | 7 | 6 | 7 | 6 | 6 | 6.50 | 6.5 | 6.2 |
| kydns-00-before-dhcp-desktop-dark.png | 7 | 6 | 7 | 8 | 6 | 7 | 6.85 | 6.9 | 7.0 |
| kydns-00-before-dhcp-desktop-light.png | 7 | 6 | 7 | 8 | 6 | 7 | 6.85 | 6.9 | 7.0 |
| kydns-00-before-dhcp-mobile-dark.png | 6 | 6 | 7 | 8 | 6 | 7 | 6.65 | 6.7 | 7.0 |
| kydns-00-before-dhcp-mobile-light.png | 6 | 6 | 7 | 8 | 6 | 7 | 6.65 | 6.7 | 7.0 |
| kydns-00-before-discovered-desktop-dark.png | 6 | 7 | 6 | 8 | 5 | 7 | 6.45 | 6.5 | 7.2 |
| kydns-00-before-discovered-desktop-light.png | 6 | 7 | 6 | 8 | 5 | 7 | 6.45 | 6.5 | 7.2 |
| kydns-00-before-discovered-mobile-dark.png | 5 | 7 | 3 | 8 | 5 | 5 | 5.35 | 5.4 | 6.8 |
| kydns-00-before-discovered-mobile-light.png | 5 | 7 | 3 | 8 | 5 | 5 | 5.35 | 5.4 | 6.8 |
| kydns-00-before-login-desktop-dark.png | 8 | 8 | 8 | 8 | 8 | 7 | 7.85 | 7.9 | 8.0 |
| kydns-00-before-login-desktop-light.png | 8 | 8 | 8 | 8 | 8 | 7 | 7.85 | 7.9 | 8.0 |
| kydns-00-before-login-mobile-dark.png | 8 | 8 | 8 | 8 | 8 | 7 | 7.85 | 7.9 | 8.0 |
| kydns-00-before-login-mobile-light.png | 8 | 8 | 8 | 8 | 8 | 7 | 7.85 | 7.9 | 8.0 |
| kydns-00-before-records-desktop-dark.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 7.0 |
| kydns-00-before-records-desktop-light.png | 7 | 7 | 7 | 7 | 7 | 7 | 7.00 | 7.0 | 7.0 |
| kydns-00-before-records-mobile-dark.png | 6 | 7 | 5 | 7 | 5 | 6 | 5.95 | 6.0 | 7.0 |
| kydns-00-before-records-mobile-light.png | 6 | 7 | 5 | 7 | 5 | 6 | 5.95 | 6.0 | 7.0 |
| kydns-00-before-services-desktop-dark.png | 5 | 7 | 6 | 6 | 6 | 6 | 5.95 | 6.0 | 6.0 |
| kydns-00-before-services-desktop-light.png | 5 | 7 | 6 | 6 | 6 | 6 | 5.95 | 6.0 | 6.0 |
| kydns-00-before-services-mobile-dark.png | 4 | 7 | 3 | 7 | 4 | 5 | 4.85 | 4.9 | 6.0 |
| kydns-00-before-services-mobile-light.png | 4 | 7 | 3 | 7 | 4 | 5 | 4.85 | 4.9 | 6.0 |
| kydns-00-before-settings-desktop-dark.png | 5 | 7 | 5 | 8 | 7 | 5 | 6.05 | 6.1 | 6.8 |
| kydns-00-before-settings-desktop-light.png | 5 | 7 | 5 | 8 | 7 | 5 | 6.05 | 6.1 | 6.8 |
| kydns-00-before-settings-mobile-dark.png | 5 | 7 | 3 | 8 | 6 | 4 | 5.35 | 5.4 | 6.8 |
| kydns-00-before-settings-mobile-light.png | 5 | 7 | 3 | 8 | 6 | 4 | 5.35 | 5.4 | 6.8 |

