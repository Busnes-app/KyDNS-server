# KyDNS baseline Council — Sol round 1

Independent baseline recorded: **quality 7.4/10; separate Ky fit 7.8/10; satisfied: false**. Reviewer: **gpt-6.1-sol**, requested and actual.

- Scored all 36 current originals; individually opened all four KyIdentity references for brand comparison only.
- Recorded integer vectors, concrete observations, exhaustive view log, exact arithmetic and a complete-array freeze in [sol-round1.json](sol-round1.json).
- Recommended one bounded layout-and-emphasis batch; product source remains read-only.
**Review this:** mobile Discovered header and Settings/table geometry (findings below); these are the strongest visible defects. The exhaustive JSON is safe to skip unless auditing the judgments or arithmetic.
**Checked:** 40 image hashes/bytes/dimensions, manifest membership, pinned authority hashes, 306-pair audit logic, per-file coverage and scripted Decimal arithmetic.
**Not checked:** executable behavior, complete accessibility, connected services or production readiness; the owner limited this task to static inspection and arithmetic.
**You need to:** nothing for this raw review. Parent owns consolidation and any implementation decision.

Quality is a current-capture coverage summary, **not a competitive ranking**. No competitor capture was supplied: best competitor and Ky gap are N/A.

| Measure | Exact sum / count | Exact mean | Half-up 1dp |
|---|---|---|---|
| Quality: mean of rubric-rounded file totals | 264.8 / 36 | 331/45 = 7.35555… | **7.4** |
| Unrounded-vector mean, transparency only | 263.9 / 36 | 2639/360 = 7.33055… | 7.3 |
| Separate Ky fit, five equal dimensions | 279.2 / 36 | 349/45 = 7.75555… | **7.8** |

| Cohort | n | Exact quality mean | Half-up 1dp |
|---|---:|---|---:|
| admin-console | 24 | 107/15 = 7.1(3) | 7.1 |
| auth | 4 | 171/20 = 8.55 | 8.6 |
| settings | 8 | 297/40 = 7.425 | 7.4 |

Parentheses enclose repeating digits. Cohorts are reported separately; brand, peers and history never enter quality.

| Canonical surface | n | Quality mean | Half-up 1dp |
|---|---:|---:|---:|
| appearance | 4 | 8.25 | 8.3 |
| blacklists | 4 | 6.6 | 6.6 |
| dashboard | 4 | 7.4 | 7.4 |
| dhcp | 4 | 7.7 | 7.7 |
| discovered | 4 | 6.85 | 6.9 |
| login | 4 | 8.55 | 8.6 |
| records | 4 | 7.45 | 7.5 |
| services | 4 | 6.8 | 6.8 |
| settings | 4 | 6.6 | 6.6 |

The strongest screens are the clear password sign-in and the regular Appearance card picker. Cream/charcoal surfaces, type and selected navigation visibly belong to the same family as KyIdentity. Palette-token fit is 9.0; geometry and shell consistency limit the separate brand result. Desktop has one sidebar; phones show one horizontal navigation region. No adjacent content sidebar or compact theme dropdown is visible.

1. **Discovered phone shell wastes the viewport.** In both themes the header is roughly 330px tall, with a large gap between the brand and navigation. The actual empty-state message is honest and readable; the stretched shell delays it. See [light original](screenshots/kydns-00-before-discovered-mobile-light.png) and [dark original](screenshots/kydns-00-before-discovered-mobile-dark.png).
2. **Phone tables and fieldsets need containment.** Settings shows fieldset right borders cut at the panel edge. Services stops at Aliases, Records at Value and Blacklists at Source, without a visible horizontal-scroll cue. This is a static discoverability/layout finding, not proof that scrolling or hidden actions fail. Services also gives three sparse rows nearly 390px of height. See [Settings](screenshots/kydns-00-before-settings-mobile-light.png), [Services](screenshots/kydns-00-before-services-mobile-light.png), [Records](screenshots/kydns-00-before-records-mobile-dark.png) and [Blacklists](screenshots/kydns-00-before-blacklists-mobile-light.png).
3. **Typography and action emphasis are uneven.** Blacklists packs small mono prose and a fragmented URL into a tall single row; desktop Services repeats orange Save buttons alongside Add service. Orange built-in/on badges add unnecessary emphasis. See [Blacklists desktop](screenshots/kydns-00-before-blacklists-desktop-dark.png) and [Services desktop](screenshots/kydns-00-before-services-desktop-light.png).
4. **Extra framing weakens suite precision.** Settings nests panel, fieldset and input borders, while light sign-in carries a large floating-card shadow. See [Settings desktop](screenshots/kydns-00-before-settings-desktop-light.png) and [light sign-in](screenshots/kydns-00-before-login-desktop-light.png).

One bounded fix batch: keep the existing compact phone shell consistent on empty pages; constrain fieldsets and table wrappers, make scrolling of existing columns visibly discoverable, and compact Services rows. Use body type for descriptive prose, retain mono for technical values, quiet repeated secondary Save actions/status badges, and remove redundant framing/shadow. Preserve DNS/admin truth, fields, actions, routes, permissions, factual guidance, one navigation region and the existing shared Appearance picker. No new feature is proposed.

**Independent satisfaction: false.** The mobile geometry defects are substantive enough to merit that single batch. Sol is advisory to progression: only Claude Opus 5.5’s current-round overall quality mean, half-up to one decimal, controls the 8.0 gate. Opus was not inspected; this review cannot pass, fail or veto that gate. Historical fleet material was never read. Drift is N/A because no previous fresh-round KyDNS review exists. This is not full behavior, security, integration, production or 9/10 acceptance.

All five complete gates — keyboard, focus, contrast, states and responsive — remain **untested**. The one visible login focus ring and the two viewport sizes cannot pass a complete gate. Below-fold flows, live SSO/recovery/replication and external forwarding remain unscored. Replication’s four combinations are N/A: the standalone fixture hides the link and redirects 303 to Dashboard; no Dashboard image was double-counted. Ordinary-user flows are N/A for this administrator-only product.

Inventory verification found no membership/hash/dimension mismatch. The duplicate audit contains every one of the 306 same-size pairs, with zero exact/near duplicates. Pair membership, unique pairs, dimensions, fraction arithmetic, SHA flags and threshold booleans were verified; expensive pixel-diff measurements were not recomputed. `brand-references.json` has stale “KyDrive quality scores” purpose text; its four file/SHA pairs correctly match the brand-only KyDNS artifact roles.

Freeze: **2026-10-27T22:56:29.734946+00:00**, **103621 bytes**. SHA-256:

`9a1a489ff0b364abc4d9ba44076171719c1f2c8fc881777ae33a76c9be3936f0`

The hash covers the **entire 36-row files array**, including all quality vectors, observations, identities, dimensions, totals and separate brand judgments. Reproduce from the saved JSON with:

```python
hashlib.sha256(json.dumps(report['files'], ensure_ascii=False, sort_keys=True,
    separators=(',', ':'), allow_nan=False).encode('utf-8')).hexdigest()
```

No trailing newline enters the canonical array bytes. The freeze precedes any non-allowlisted input; only the change-brief skill was subsequently read for closeout. No peer or historical scores/images were accessed before or after freeze, and the published ratings will not be revised after feedback.

Authority is fixed at `ky-ui` `639e5882cada099dd189197beb2a3e3b4c188658`. Product source is `5d274783d2277c3a44be2cfe11cf068a6df49580`, captured HEAD `8e41fae3559c6c6e0e7a5dac7536754d855643e8`, frozen evidence HEAD `ce4d392b2199a76bc0dc29b3f971d0681c111dc0`. Hashes, capture conditions, inventory, per-file caveats, all 40 original-resolution view events and exact rational/repeating-decimal arithmetic are in the JSON.

DOX pass: root, repository and review-directory contracts were read. Owning AGENTS.md files were intentionally unchanged: this assigned report pair adds evidence within the existing contract, without changing product behavior, durable structure or workflow. No build, test, browser, install, source edit, commit, push, PR mutation, subagent, Gemini or MySlop action occurred.
