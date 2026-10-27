# Fresh KyDNS baseline setup

Captured once on 2026-10-27, 22:40:52–22:41:14 UTC. This is evidence preparation,
with no scores or acceptance. Product source, tracked files, pinned authority,
brand references and baseline-plan.json stayed unchanged.

## Source and outputs

- Worktree: `kydns-server/.claude/worktrees/dns-eight`, branch `ui/dns-eight`.
- Frozen HEAD: `8e41fae3559c6c6e0e7a5dac7536754d855643e8`.
- Product source: `5d274783d2277c3a44be2cfe11cf068a6df49580` (`origin/main`).
  The two later commits contain only the evidence contract/authority and its root index.
- New evidence: `capture.mjs`, `manifest-00-before.json`, `inventory-v1.json`,
  `near-duplicate-audit-v1.json`, `source-provenance.json`,
  `capture-provenance.json`, `baseline-setup.md`, `fixture-data.json`, and
  36 `screenshots/kydns-00-before-*.png` originals. Inventory enumerates every image.
- Existing AGENTS, authority, brand references and baseline plan are immutable inputs.
  DOX pass: their contracts already cover this work; no instruction or product contract changed.

## Build and local isolation

The owner-reported hard lock limits this task to one capped sequential build and one
owned fixture/browser at a time. No installs, tests of the full product, race/crypto
suites, Docker, production calls, host DNS/resolver changes or unrelated process signals ran.

Actual build, from this worktree, with the existing persistent caches:

```sh
CGO_ENABLED=0 GOMAXPROCS=2 go build -p=1 -trimpath -o "$fixture_dir/kydns" ./cmd/kydns
```

`-p=1` is Go's spelling of the requested single package-build worker. Build passed
without output. Go 1.27.3, linux/amd64; binary build info records the frozen commit,
`CGO_ENABLED=0`, `-trimpath=true`, and `vcs.modified=false`. It was built before any
untracked evidence was created. Current untracked evidence makes the working directory
modified; source hashes/diff remain unchanged. No rebuild was used to hide that state.

The private fixture directory was allocated with `tempfile.mkdtemp(prefix='kydns-dns-eight-')`,
mode 0700, outside the repository. Password, config, setup/bootstrap tokens, data,
launchers, logs and process records are private files mode 0600; the owned binary is
0700 so it can execute. No HOME/CODEX_HOME variables were repurposed. The exact private
directory and credential paths are returned to the parent outside canonical evidence.

Before launch, explicit bind/close checks passed for admin TCP5880, DNS UDP+TCP15880,
unused upstream TCP15881 and refusing proxy TCP15883, all at 127.0.0.1. No busy process
was killed. Replication is standalone, with no listener, so no replication port is needed.
Final `ss` inspection attributed only admin/DNS to the recorded server PID and proxy15883
to its owned launcher. Port5882 and the unused upstream had no listeners.

Public config recipe (replace only the private path):

```yaml
data_dir: <private fixture directory>/data
dns:
  listen: "127.0.0.1:15880"
  private_domain: home.arpa
  upstreams: ["tls://127.0.0.1:15881"]
  allow_query: ["127.0.0.0/8"]
  log_queries: true
  log_client_ip: false
admin:
  listen: "127.0.0.1:5880"
discovery:
  dhcp_lease_file: ""
health:
  workers: 1
replication:
  listen: ""
  primary: ""
```

The server environment contains only `PATH=/usr/bin:/bin`, `GOMAXPROCS=2`,
`KYDNS_BACKUP_DEPOSIT_INTERVAL=0`, `HTTPS_PROXY=http://127.0.0.1:15883`,
`HTTP_PROXY=http://127.0.0.1:15883`, `NO_PROXY=127.0.0.1,localhost`.
No deployment credentials were inherited. There are no service check URLs, discovered
lease source, DHCP listener, replica peers, SSO configuration, recovery pin or backup destination.
The TLS upstream is an unused loopback endpoint; all recorded queries resolve locally
or are blocked before forwarding. External resolution is untested.

The shipped built-in list seeds enabled and refreshes immediately. The owned stdlib
HTTP proxy refuses CONNECT with403 and never opens an upstream connection. One CONNECT
for `raw.githubusercontent.com:443` was refused; no external source was contacted.
Then the real native UI disabled `steven-black` (ID1). Its entries remain0, last success
never, state off/never-loaded; the stored error is `download failed: Fixture blocks external traffic`.
No replacement list content or successful refresh was fabricated. The proxy remains
running with the fixture as its external-fetch guard; it is not a live integration.

## Native setup and synthetic state

Used T3 `preview_status`, then an owned Incognito tab. The first `preview_open` with
`about:blank` was rejected as an invalid URL; opening without a URL worked. One initial
navigation preceded readiness and returned ERR_CONNECTION_REFUSED; local health200 and
the next navigation succeeded. One unsupported exact-role locator was corrected using
the current snapshot's aria-ref before any seed mutation.

Read the private setup-token and generated password in memory; native type/click completed
the actual `/setup` form. Password was randomly generated (`token_urlsafe(30)` plus
mixed-case/digit/punctuation suffix), stored privately and never printed. Authenticated
shell and zero password/setup-token fields were checked before any post-submit snapshot.
Native Sign out followed by a real native password login succeeded. No filled/pending
setup/login secrets were snapshotted.

Seeded by real native forms, in this order:

1. Services IDs1–3: `atlas` →127.0.0.21, alias`files`; `beacon` →127.0.0.22,
   alias`status`; `cedar` →127.0.0.23, alias`notes`. All views, no health URL,
   proxy, MAC or routing override.
2. Manual A records IDs1–3: `archive.home.arpa.` →127.0.0.31,
   `printer.home.arpa.` →127.0.0.32, `router.home.arpa.` →127.0.0.33; all views.
3. Disable built-in list ID1; add deny rule ID1 for `ads.example`.

Read-only SQLite confirmed IDs and values. No sessions/roles/config were written directly.
The product's automatic bootstrap token and local backup sealer file `data/backup_key`
exist privately from normal startup. No additional API/agent tokens, SSO links, suite
recovery public-key pin/ceremony/pairing or replica invitations were created.
No destructive action or live integration was exercised.

Issued exactly24 real A questions through owned loopback sockets:18 UDP then6 TCP.
Repeated `[atlas.home.arpa, beacon.home.arpa, cedar.home.arpa, archive.home.arpa,
printer.home.arpa, router.home.arpa, ads.example, ads.example]` three times. Checked
transaction IDs and rcodes:18 successful authoritative responses,6 policy NXDOMAIN,
0 forwarded/refused/SERVFAIL. `fixture-data.json` records each query time/transport.
No further DNS traffic or fixture mutation ran during capture. Ages, uptime, response
time and minute positions remain real and drift naturally; future matching uses these
same names, values, role, counter totals and fixture process. No clock override or mocked stats.

Native inspected all accessible surfaces at desktop light and phone dark, plus empty
signed-out login and the standalone `/replication` redirect. Native probe DPR was2;
images remain tool-history probes, separate from exact-DPR1 evidence. The tab was closed
before launching the formal headless browser. Probe measurements are in capture-provenance.json.

## Formal capture

Reused installed Playwright1.64.1 from:
`/home/yoshi/git/busnes.app/KyDrive-server/.claude/worktrees/drive-eight/web/node_modules/@playwright/test/index.mjs`.
Actual Chromium156.0.8078.4, Nodev24.22.0, headless Linux/amd64. Package/lock/browser/driver
hashes are preserved in capture-provenance.json. No npm install/ci or browser download ran.

Invocation, once only after native tab closure and fixture seeding:

```sh
KYDNS_CAPTURE_PRIVATE="$fixture_dir" node docs/ui-dns-eight/capture.mjs
```

The driver refuses to overwrite any original image or its manifests. Do not rerun it
against this source set. A subsequent stage belongs to the parent and must keep the
originals and captured driver bytes intact. Published Node tooling reads files with
`readFile`, with no `cat` childprocess.

Each OS scheme × viewport gets a fresh context and one page: light desktop, light phone,
dark desktop, dark phone. Each context captures empty signed-out login, signs in through
the real password form, then captures dashboard, services, records, discovered, blacklists,
DHCP, settings and appearance. No storageState, auth/cookie/storage injection or bypass.
Normal service workers (`allow`), no request interception, no styles/templates/DOM/content
rewrites. Static assets fetched by the browser were hashed against frozen worktree bytes.

Every image:1280×900 or390×844, DPR1, en-US, America/New_York, OSlight/dark,
no `kydns-theme` saved choice, Busnes Light/Dark resolved naturally, initial viewport,
PNG, `fullPage:false`. Waited for headings, fonts.ready and actual dashboard SVG axis
text (first real stats poll); Appearance waits for its23rd card. Moved pointer(0,0)
before500ms settling. Playwright screenshot defaults apply (animations allowed,
caret hidden, device scale); no screenshot CSS or crop. Login's shipped autofocus remains.
Per-image manifest records hover controls (none), active element, errors, HTTP≥400,
failed requests, scroll state, theme, loaded fonts, nav and internal overflow. All
document scroll offsets were0,0. No original was recaptured, overwritten or corrected.

## Coverage and observed limitations

- 36 originals:8 accessible navigation destinations plus signed-out login ×2 devices ×2 schemes.
  Replication is N/A for all4 combinations: standalone hides its link and `/replication`
  redirects303 to Dashboard. Code and native/formal navigation agree. No fake screenshot
  or duplicate Dashboard is labelled Replication.
- DHCP disabled/no interface selected; Discovered truthfully reports discovery off.
- Settings has real API tokens, Single sign-on, Backup and maintenance, Disaster recovery,
  Upstreams and Configuration sections below the initial viewport. They are unscored;
  there is no invented backup route. Existing unsubmitted forms are captured as shipped.
- Desktop has exactly one240px left nav. Phone uses a single horizontal nav row below
  the brand/Sign out row; selected destination is scrolled into view. No sidebar theme dropdown.
- Appearance has23 real radio cards (Match system plus22 catalog entries), checked system,
  no saved choice, follows both OS schemes. Desktop shows all cards; phone shows its
  initial rows only. Persistence/keyboard/cross-tab behavior was not exhaustively tested.
- Phone Services, Records and Blacklists have horizontally scrolling table cards; actions
  and columns are offscreen initially. Settings has clipped/horizontally scrollable
  fieldset containers (measured356px client /368px content for the first card). Document
  overflow itself is0px. No scrolling was used to improve the formal pictures.
- The short phone Discovered page stretches its header/nav to330.0625px, compared with
  113px on the other inspected phone destinations. This is shipped behavior in both schemes.
- Dashboard plots one brief burst of real queries. No forward/cache-hit series was invented.
- Formal run:0 console/page errors,0 HTTP≥400,0 failed browser requests,0 external browser
  requests,0 hovered controls. These are capture observations, not whole-product assurance.

Every one of the36 current originals was individually inspected with `view_image(detail=original)`:
no contact sheets, crops, resizing or prior quality images. Filename/product/surface/device/theme
match all36. No secrets, setup remnants or browser prompts appear. Inventory verifies SHA256,
bytes, exact dimensions and the full36-combination matrix. Sequential Pillow12.3.0/NumPy2.5.4
audit compares all306 same-sized pairs, excludes324 differently sized pairs, and records
every measurement: changed pixels<0.1% AND max channel delta≤8. No exact or near duplicates;
all `duplicate_of` values are null.

Checked: capped build; Node syntax; ky-ui vendor checker; real setup/password login; UI seeding;
24 local UDP/TCP queries;36 capture assertions;36 original image inspections; inventory/duplicate
audit;15 served-asset hash matches; before/after SHA256 of every prior tracked file, pinned authority
and brand references; binary identity and loopback listener ownership. No full suite, hosted CI,
race/crypto, live DHCP/replication/recovery/SSO, accessibility or production readiness claim.

The owned server and refusing proxy remain running for the parent. Browser closed. Private
process record contains PID, executable, startticks and hashes; verify those before any later
signal. No MySlop retry/post ran because the caller supplied the known401 restriction.
