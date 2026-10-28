# R6 capture stopped — incomplete evidence

The capture worker introduced an incorrect Appearance font assertion and stopped the single capture run at `capture-round6.mjs:184:50`, exit1:

```text
AssertionError [ERR_ASSERTION]: mono font required only for actual mono text

false !== true
```

No retry, recapture, overwrite, correction, cleanup, extra browser, DNS reseed, build or test followed. The driver is preserved as executed. Browser closed through its awaited `finally`. Parent-owned server/proxy remain running.

## Actual output and limits

- Eight original desktop-light PNGs: login, Dashboard, Services, Records, Discovered, Blacklists, DHCP and Settings. Matrix8/36; no Appearance, mobile or dark original. All eight individually viewed once at original detail, no montage/crops/resizing or exposed secrets.
- One fresh Playwright1.64.1/Chromium156.0.8078.4 context and one successful real password-form login completed. No auth/cookie/storage injection, interception or DOM/style/content mutation.
- Dashboard assertions passed before its screenshot:24total,18authoritative,6blocked,0forwarded, `refused_total=0`; raw cache buckets had no lookups, exact cache-empty text/aria-description and plot containment; real latency timings rounded0ms, no empty text/description and an existing circle. Only desktop-light was reached. Raw stats and measured bounds were held in memory and lost at exit; no geometry or raw history is reconstructed.
- Blacklist assertions passed before its screenshot: quiet native POST Refresh all lists button beside Lists, nonempty CSRF presence checked as boolean,40px target, transparent border/background; Filtering contains only Turn filtering off; row actions Enable/Refresh, list off/never loaded/0entries and unchanged URL metadata. Button was not submitted.
- Driver reached15-served-assets final gate neither successfully nor with persisted metadata; no complete15-asset claim. Each observed static response was compared with worktree and HEAD when handled.
- No manifest, capture/source provenance, fixture, inventory, duplicate audit, artifacts or review-inputs JSON was emitted for R6. Required46new-output/220image/630pair checks are **NOT COMPLETE**. Existing184image inputs plus8partial originals total192, hashed and IHDR checked offline. No pixel-pair audit ran. No scores or acceptance.
- Replication4N/A and below-fold/live/gates remain the documented scope limitations; R6 never reached its formal replication checks. Null-window→valid-zero transition regression/full product suite remain hosted-CI-only and were not run here.

## Qualified font diagnosis

The new assertion used computed CSS family references and nonzero element bounds as a loading requirement. That cannot establish that this particular `document.fonts.check('13px "IBM Plex Mono"')` probe is required. R5 records Space Grotesk true/IBM false on all four Appearance views; the task explicitly requires retaining that qualified unused-font fact. Space Grotesk passed on R6 Appearance before the bad IBM assertion. The driver does not persist the failed page state. No blanket font-loading failure or pass is claimed. No product source changed.

Native R6 checks remain **FALSE**, not passed. The private probe's original errors are retained:

- preview_type returned success but immediate field inspection reported filled:false.
- Preview automation waitFor timed out after 10000ms.
- Preview automation evaluate failed: TypeError: Cannot read properties of null (reading 'textContent').
- No preview automation host is available for evaluate in environment 52a9fa6f-a5d5-4167-a63b-65f0d76e09e4.

## Source, fixture and ownership

Frozen HEAD/product source `6ce6fb505d6f2e4c24256953464efbf903758b21`. All85build-source hashes match current bytes and frozen HEAD. Actual parent build revision `a97c7b48e135657687f8cc56f6189dd3a29bb002`, `vcs.modified=true`, retained verbatim in private build state; capture worker built nothing. The later6ce commit contains terminal R5 reports; no clean-build claim.

Running binary SHA256 `c448ccce3026d02252329827acd842a9bdacbc3a1b29684bce62b6fc44220979`. Server PID487240/startticks1735765, proxy PID487230/startticks1735759; final executable/startticks/hash and four owned loopback listeners verified. Private fixture stays0700/password0600. Parent retains ownership; do not signal or repeat the24questions. No host-name-change claim.

Exactly24real Aquestions issued once under an exclusive started marker:18UDP then6TCP, eight baseline names repeated three times, transaction IDs26000–26023 verified;18authoritative answers+6policyNXDOMAIN. No other DNS queries. Read-only SQLite whitelist still matches retained3services/3records/all views/list off0entries/denyads, DHCP/discovery off and SSO disabled with empty issuer/default clientkydns. No DB writes or fixture edits apart from normal real login/session and prescribed query counters. Proxy events unchanged; no external request.

DOX pass: ancestor/worktree/evidence contracts read; only new incomplete R6 evidence and private owned checks were written. Existing scope/workflow/index still cover this work. All AGENTS intentionally unchanged as requested. Concurrent `round5-disagreement-audit.json` excluded without reading or editing. No earlier evidence/review/exporter edits, delegation, commit or push.

## Offline stop-state verification

At 2026-10-28T01:27:14.901121+00:00: frozen HEAD/diff; all805tracked files and283prior evidence preserved byte-for-byte;85source/HEAD bindings; actual dirty build metadata;24wire records/exclusive marker; retained whitelist fixture/SSO;192PNG byte hashes/IHDR dimensions;8original-detail views; final process identity/listeners/proxy-event preservation checked. This validates the stop state, **not a completed R6 matrix**.

Driver `capture-round6.mjs` SHA256 `8a0507e99ad2a3c0844481c949f5ee7e035fef33aede3ee69f0f378f4332663e`.

## Partial originals and exact R5 matches

- `screenshots/kydns-05-after-blacklists-desktop-light.png` SHA256 `d4c288819855afd10a70e4dd5a0e3d8d238b35ad7fb18dac768334a8887ceb6f`, 96939bytes,1280×900.
- `screenshots/kydns-05-after-dashboard-desktop-light.png` SHA256 `9a48714e8de17596ca69ac45f812c8121a4fc77ab80dc263b6733e061b270381`, 71150bytes,1280×900.
- `screenshots/kydns-05-after-dhcp-desktop-light.png` SHA256 `794b841d14612c83ff48591a795c8c34340da81e3a1bc9f54902370256576d79`, 66340bytes,1280×900.
- `screenshots/kydns-05-after-discovered-desktop-light.png` SHA256 `d97c166cf5581f84b4300df803de5e4703cf215c0a7213aecccfdd57d90855ed`, 38339bytes,1280×900.
- `screenshots/kydns-05-after-login-desktop-light.png` SHA256 `28ccbf2e360abcb8aa2f22d7b814f362d5dc23ec8c9b07d1d341c473a8bfa39d`, 14837bytes,1280×900.
- `screenshots/kydns-05-after-records-desktop-light.png` SHA256 `306d81c9848b4b9773816d52b9e8c513973937d3314948f90e047342453c24c4`, 60334bytes,1280×900.
- `screenshots/kydns-05-after-services-desktop-light.png` SHA256 `cae91e2300c46b90516541b5d3d10bd285fd15591b3949e3128454b8feec71fc`, 79159bytes,1280×900.
- `screenshots/kydns-05-after-settings-desktop-light.png` SHA256 `b903652771c3e706cefca41f8448049e3537f16b92b3232502245022e7467131`, 66172bytes,1280×900.

6of8 partial originals byte-identical to R5; no quality implication:
- `screenshots/kydns-05-after-dhcp-desktop-light.png` = `screenshots/kydns-04-after-dhcp-desktop-light.png`.
- `screenshots/kydns-05-after-discovered-desktop-light.png` = `screenshots/kydns-04-after-discovered-desktop-light.png`.
- `screenshots/kydns-05-after-login-desktop-light.png` = `screenshots/kydns-04-after-login-desktop-light.png`.
- `screenshots/kydns-05-after-records-desktop-light.png` = `screenshots/kydns-04-after-records-desktop-light.png`.
- `screenshots/kydns-05-after-services-desktop-light.png` = `screenshots/kydns-04-after-services-desktop-light.png`.
- `screenshots/kydns-05-after-settings-desktop-light.png` = `screenshots/kydns-04-after-settings-desktop-light.png`.

## Individual original-detail view log

- `screenshots/kydns-05-after-login-desktop-light.png` — 2026-10-28 01:25:37 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-dashboard-desktop-light.png` — 2026-10-28 01:25:37 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-services-desktop-light.png` — 2026-10-28 01:25:40 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-records-desktop-light.png` — 2026-10-28 01:25:40 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-discovered-desktop-light.png` — 2026-10-28 01:25:45 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-blacklists-desktop-light.png` — 2026-10-28 01:25:45 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-dhcp-desktop-light.png` — 2026-10-28 01:25:50 UTC,original detail; no secret/prompt visible.
- `screenshots/kydns-05-after-settings-desktop-light.png` — 2026-10-28 01:25:50 UTC,original detail; no secret/prompt visible.
