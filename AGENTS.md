# KyDNS Server

KyDNS is a Homelab DNS server with easy domain name assignments and blackhole filtering.

Please see `internal/web/static/` (`styles.css`, `fonts/`) for look and feel.

# Ponytail, lazy senior dev mode

Use the smallest correct change.

1. Reuse what already exists.
2. Prefer stdlib and native platform APIs.
3. Add dependencies only when they remove meaningful code.
4. Fix shared root causes, not one caller.
5. If a shortcut has a limit, mark it with `ponytail:` and name the upgrade path.

Non-trivial logic must include one runnable check (unit test or minimal self-check).

# DOX framework

## Core Contract

- AGENTS.md files are binding contracts for their subtree.
- Read from root to nearest AGENTS.md before editing.
- The nearest AGENTS.md controls local details; parent docs keep global rules.

## Update After Editing

- Run a DOX pass for every meaningful change.
- Update nearest owning AGENTS.md when behavior, responsibilities, or verification changes.
- Keep Child DOX Index entries current and delete stale rules.

## User Preferences

- Web themes default to the Busnes.app cream/light and charcoal/dark palettes with orange accents, following the OS until a browser-local choice is saved. Preserve existing named themes and saved choices.

- Best-effort 90-second keyword refresh policy (foreground cadence; background catch-up on resume).
- DOX hierarchy scope is app-only.

## Shared browser UI

- `internal/web/static/ky-ui/` is generated from Busnes-app/ky-ui, pinned by `VERSION` file hashes. Change shared colors, navigation states and storage helpers upstream, then run its consumer sync with an explicit worktree map; do not hand-edit vendored files.
- `GET /appearance` (session only) is the theme picker. `static/theme.js` is a classic `<head>` script after the generated `ky-ui/themes-browser.js` (`window.kyThemes`), so the saved choice applies before first paint; it builds the picker after `DOMContentLoaded`: native radio cards built with DOM calls, `Match system` plus every catalog theme, saved by catalog name under `kydns-theme` (Match system removes the key); older saved names resolve unchanged. `app.css` aliases local colours to the shared tokens and named palettes override them inline. Verify with `go test ./internal/web -run Theme`; rendered checks live in `docs/ui-fleet-2026-10-26/capture.mjs`.
- Below 860px the brand and Sign out share the top row and the links (`.app-nav-links`; the marketing `styles.css` owns `.nav-links`) are one sideways-scrolling row; `static/nav.js` scrolls only that row to keep the current page in view, also after the web font loads. The licence button moves below the page (`.page-legal`). Dashboard charts draw in CSS pixels: `charts.js` sets the viewBox width to the rendered width and keeps the template height, so axis text stays 12px. Evidence: `docs/ui-visual-refinement-2026-10-26/`.
- App sections are flat. Services, Records and Blacklist sources retain native table headers and expose labelled two-column rows below 860px; optional service routing edits use native `details` with the existing POST form. List headers use quiet jump links to their existing add forms; the actual submit remains primary. Phone wordmarks include a neutral scroll-navigation cue. Checkbox/help rows share the input column, routing disclosures use a chevron, and Dashboard query/registry metrics share column tracks. Services and Records share metadata sizing; phone labels sit above paired values and Remove shares the name row. Blacklist name, description and source URL share one readable column; escaped URL segments prefer slash breaks and retain emergency long-token wrapping. Phone grid rows reserve only intrinsic height for navigation, including short empty pages. Server settings retain native fieldset/replica disabling, use plain group dividers and an explicitly associated Save button at the top. `browser/dashboard.spec.mjs` verifies shipped-size containment, keyboard disclosure and real routing persistence.
- Products own layout, routes and saved choice keys; the catalog and palette values come only from ky-ui. Busnes aliases consume shared tokens; mark primary navigation with `ky-nav-item` while preserving current-page semantics.
- Verify vendored files with `node internal/web/static/ky-ui/check-vendor.mjs` from this document's directory. Builds/CI run that check. Rendered evidence and capture limitations are recorded in the repository-root `UI-VERIFICATION.md`; the fleet Appearance stages (screenshots, manifests with hashes and behaviour checks, results CSV, reproducible `capture.mjs`) live in `docs/ui-fleet-2026-10-26/`.

## Child DOX Index

Keep these aligned:

- `docs/ui-dns-eight/AGENTS.md` — fresh Council captures, independent raw reviews and visual progression evidence.

- `browser/AGENTS.md` — real-server Chromium regressions for dashboard, refresh,
  Appearance and record forms; CI and tag releases run them before publishing.
- `README.md` — product scope and user-facing capabilities.
- `DESIGN.md` — architecture. Replication ships (`internal/replica`).
- `LOGGING.md` — logging and privacy requirements.
- `SECURITY.md` — security policy and trust boundaries.
- `CONTRIBUTING.md` — contribution and verification workflow.
- `docs/STRONG-CRYPTO.md`, `docker-compose.strong-crypto.yml` — supported process/edge key policy and data-preserving migration; `scripts/check-strong-crypto-compose.py` verifies overlay preservation.
- `Makefile`, `scripts/build-rpi-image.sh` — release artifacts, including the
  Raspberry Pi OS Lite arm64 SD-image builder.
- `.github/workflows/release.yml`, `.github/workflows/dependency-release.yml` —
  tag releases `vX.Y.Z` and the daily Dependabot releases `vX.Y.Z.N`; the tag is
  the only version source. Described in `CONTRIBUTING.md` "Releases".
- `docs/superpowers/specs/2026-08-11-kydns-v1-design.md` — approved v1 design.
  Single-node with per-subnet views; replication deferred to a later spec.
- `docs/superpowers/specs/2026-08-12-kydns-blacklists.md` — approved DNS
  blackhole filtering design.
- `docs/superpowers/specs/2026-08-13-kydns-settings-in-the-ui-design.md` —
  approved design for settings in the database: what stays in the config file,
  what moves, what applies live, and the `allow_query` guardrail.
- `docs/superpowers/plans/2026-08-13-kydns-settings-in-the-ui.md` — the
  implementation plan for that spec.
- `docs/superpowers/plans/2026-09-04-kydns-kyrecovery-deposit.md` — implementation
  plan for adopting `ky-primitives` and adding sealed KyRecovery pairing,
  capsule export, scheduled deposits, drills, and restore.
- `docs/superpowers/plans/2026-09-04-kydns-backup-to-suite-spec.md` — follow-on plan
  bringing that integration to the suite spec on `ky-primitives/recoveryclient`: local
  copies, admin schedule, pin by hand, unpair, restore runbook. Phase B is on
  ky-primitives v0.5.0.
- `docs/superpowers/specs/2026-10-20-kydns-mcp-agent-access-design.md` — MCP
  endpoint for AI agents: agent token scopes, the query-log grant, the in-memory
  query ring, bounded pause.
- `docs/RESTORE.md` — restore runbook; `scripts/restore-drill.sh` proves Step 1.
- `docs/superpowers/plans/2026-09-05-kydns-recoveryclient-v051.md` — v0.5.1
  drill migration, existing-pairing compatibility, and separate fixture/live proof.

Code, one concern per package:

- `cmd/kydns` — command dispatch. `serve`, `admin`, and the API-backed verbs. Restore
  checks the unverified manifest for service `KyDNS` before reading shares; authenticated
  extraction then proves that same service binding.
- `internal/app` — process wiring: config, store, servers, background loops. It wires
  the local store check into the public instance `/healthz` endpoint. The scheduled
  backup loop is quiet only for unconfigured backups; a pinned key whose file is missing is
  audited and logged as a failed backup run.
- `internal/config` — the YAML config and its defaults. The file owns three
  keys (`data_dir`, `dns.listen`, `admin.listen`); every other key it carries
  is a first-run seed for the database. `internal/config/example_test.go`
  asserts `kydns.example.yaml` against the defaults and against that split, so
  a key the file no longer controls cannot be documented as if it did.
- `internal/settings` — the settings snapshot the runtime reads, and the single
  path by which it changes: validate, persist, rebuild, apply, all or nothing.
- `internal/store` — SQLite schema and migrations, the single write chokepoint.
  `ResetAdmin` atomically revokes all API tokens and browser sessions, unlinks local
  SSO, and disables unmanaged SSO; installer-managed SSO remains unchanged.
  Sessions bind to the monotonic recovery generation captured before authentication,
  so an in-flight login using old credentials cannot survive recovery. Token issuance
  carries that generation from browser/API authentication and checks it in the insert
  transaction; a pre-recovery request cannot mint a surviving replacement token.
  Verify with `go test ./internal/store ./internal/app -run Reset`.
  `sso_settings` seeds an empty issuer, and the settings form refuses to enable
  SSO without one. Verify with `go test ./internal/web -run EnableNeedsIssuer`.
  The last migration slot is an empty placeholder for a retired one-off cleanup;
  keep it, since `user_version` counts slots.
  `PingContext` checks the local database for instance health.
  `SnapshotTo` delegates to `recoveryclient.SQLiteSnapshot`, which binds the
  `VACUUM INTO` destination path rather than interpolating it.
  `OpenSnapshot` verifies and opens restored artifacts read-only without migrations.
  Connection pragmas live in the DSN so every pooled connection gets them.
  `Open` creates the database 0600 (SQLite copies that to `-wal`/`-shm`).
  A permission error there names the uid and the `chown` that fixes it. Verify
  with `go test ./internal/store -run OwnershipFix` as a non-root user.
  `ReplicationSnapshot` reads the replicated config in one transaction;
  `ApplySnapshot` runs its check on the merged row before writing anything.
- `internal/registry`, `internal/zone` — services, records, views, validation,
  and the immutable zone snapshot the DNS server reads. `ValidateDocument` is
  the store-free whole-registry check import and replica apply share.
- `internal/backup` — the KyDNS adapter over `ky-primitives/recoveryclient`: what a
  capsule carries, the drill checks, and the `Service` the routes, scheduler and CLI
  share. Pairing, pinning, delivery and schedule live in the library.
- `internal/replica` — linked-node pairing accepts only literal IP:port peer
  addresses and authenticates self-signed peers by Ed25519 key fingerprint.
  `node_key` (raw 32-byte seed) loads through `keyfile`, so a non-0600 key is
  refused. A replica validates a pulled snapshot (settings and registry) before
  committing it. A peer's `Address` is the host it paired from, for display
  only; the primary never dials a replica.
- `internal/dnsserver` — ACL, authoritative answers, cache, forwarding, and the
  in-memory query ring (`QueryLog`) that fills only under `log_queries`, holds
  clients only under `log_client_ip`, and empties when either turns off.
- `internal/upstream` — DoT, DoH, and opt-in plain-UDP upstreams.
- `internal/policy` — blacklist normalization, list parsing, fetch, refresh,
  and decision (deny/allow/list) for the DNS pipeline. `Explain` names every
  matching source; a pause is an in-memory deadline (at most `MaxPause`) that
  `Decide` compares on every query, so it resumes with no timer.
- `internal/discovery`, `internal/health` — DHCP leases and check probes.
  Runtime state only; never persisted unless an operator promotes it.
- `internal/dhcpd` — the optional built-in DHCPv4 server. It implements
  `discovery/dhcp.Source`, so its leases reach DNS by the lease-file path.
  Node-local: its settings never replicate, and a replica never serves it.
  Its leases are the one discovery result that is persisted, so a restart
  cannot re-issue an address that is still in use.
- `internal/adminapi`, `internal/web`, `internal/auth` — JSON API, server-side
  rendered UI, sessions, password hashing, and verified OIDC login.
  The Services page owns names, addresses, aliases and routing; health-check
  configuration and status remain available through the CLI/API.
  `POST /auth/sso/backchannel-logout` is the anonymous OpenID Connect Back-Channel
  Logout receiver: the provider-signed `logout_token` is its authority, and it is not
  rate limited (a KyIdentity restore sends a burst; a refusal spends a retry), so
  `auth.LogoutVerifier` discovers once per (issuer, client ID), keeps the key set, and
  retries a failed discovery after 30s; never build a verifier per request. Sessions
  record the login's issuer, subject, ID-token `sid` and ID-token `iat`; a logout ends
  the issuer's `sid`, or the subject's logins issued before the second after its `iat`,
  and fences that `sid` or (issuer, subject) cutoff for ten minutes so an in-flight
  callback answers 401, all under the session store's one lock. The subject fence is
  keyed by subject, so it covers a subject with no session yet, and never moves back. Verify with `go test ./internal/auth ./internal/web -run 'Logout'`.
  `render` fills the chrome every screen shares: replica read-only state and
  the open-resolver banner (`PublicRanges`), so no page opts in by hand.
  `internal/adminapi/mcp.go` serves `/mcp` for AI agents on the admin listener
  only. Agent tokens are `tokens` rows with an MCP `scope` and a separate
  `query_log` grant; they never authenticate `/api/v1`, admin tokens never
  authenticate `/mcp`, and an agent token idle 90 days is revoked. Every tool is
  in `TestMCPAccessMatrix`. Verify with `go test ./internal/adminapi -run 'MCP|Agent'`.
- `internal/web/origin.go` — browser-origin gate in `TransportGate`: Host must be
  `localhost`, an IP or in `KYDNS_ADMIN_HOSTNAMES` (DNS rebinding); cross-origin
  state changes and preflights are refused; no route ever sends CORS headers.
  Verify with `go test ./internal/web -run 'MCPRefuses|NoCORS|Forged|HostHeader|TokenScreen'`.
- `internal/agentskill` — the embedded `SKILL.md` served at
  `/agent/skills/kydns/SKILL.md`; see its `AGENTS.md`. The admin API
  serves public `ky.health/1` JSON on `/healthz` and `/api/v1/healthz` from one
  five-second cache. A failed local database check returns 503 with safe status
  codes; service discovery health remains on the authenticated `/api/v1/health`.
- `internal/cli` — the API client behind the non-`serve` commands. It refuses
  plain `http://` to a non-loopback `KYDNS_URL`, since the token is a bearer.

The build must stay cgo-free: the image is distroless, so the pure-Go SQLite
driver is not optional.
- KyRecovery owns the authoritative pairing and deposit contract in
  `kyrecovery-server/zero_code_pairing_handoff_spec.md`. KyDNS implements the product
  client and keeps pairing settings, audit events, keys, and receipts node-local.

## Installer boundary

- `docs/INSTALLER_SETUP.md` owns the container-local `apply-setup` contract; dispatch lives in `cmd/kydns`, private live IPC in `internal/app` and `internal/web`, atomic owned configuration and directory fences in `internal/store`.
- Managed SSO grants exact verified issuer/subject administrators without adopting the local recovery account. KyDNS is administrator-only; everyday identities are refused. Missing local accounts receive an empty disabled password slot, never a generated password.
- Check and matching resume never migrate, generate keys, audit, claim pairing codes or reactivate revoked authority. Existing DNS data, local recovery passwords and unbound legacy tokens stay intact.
- Signed installer directory events require durable SCIM revisions and the `kydns.admin` app role. Revocation removes sessions' authority and deletes issuer/subject-bound API tokens and descendants; their authority is checked per request. Sessions are fenced by the directory state's `RevokedAt` revision; only `Store.SSOClient` (the OIDC exchange) reads a managed client secret, while rendered views use `SSOSettings`.
- Managed SSO cannot replace native configuration or link its identities to the independent recovery account. Backup collection includes every protected managed credential and its original destination map; only local recovery authority (password session or unbound token) enrolls the first recovery key, on both pin and pair paths.
- `KYDNS_TRUSTED_PROXIES` supplies explicit trusted peers and refuses `/0`; public `/api/v1/installer/network`, rate keys, callback scheme and secure cookies share that trust.
- Verify `go test -race ./internal/store ./internal/registry ./internal/adminapi ./internal/web ./internal/auth ./internal/config ./internal/app ./internal/backup ./cmd/kydns` and `go vet` for those packages. New shared setup code uses the isolated development Go workspace until its release is published.

## Browser verification

- CI and tag releases give dependency installation its own 20-minute budget, followed by the browser suite’s separate 10-minute limit. Keep install/audit/browser execution as required steps; a download delay must not consume the test budget.

## Backup verification

- CI's `test` job and the tag-release job run `scripts/restore-drill.sh` and `scripts/backup-live-proof.sh` before publishing. They require Go, curl and sqlite3, use disposable data and recovery keys, and do not contact KyRecovery.
- The live proof defaults to loopback DNS port 15353/admin port 18053; use `KYDNS_LIVE_DNS` and `KYDNS_LIVE_ADMIN` for unused ports on shared machines. Never point `KYDNS_LIVE_DIR` at existing data: the script deletes its owned fixture.

## Operator documentation

`INSTALL.md` is the root-owned standalone entry point. When installation prerequisites, bootstrap, persistence or verification change, update it together with `README.md` and the linked operator guides.


## README and topic guides

Keep `README.md` as the concise product overview and navigation. Detailed procedures live in `docs/AI-AGENTS.md`, `docs/BACKUPS.md`, `docs/FEATURES.md`, `docs/CONFIGURATION.md`, `docs/REPLICATION.md`, `docs/PACKAGES.md`, `docs/RASPBERRY-PI.md`, `docs/RUNNING.md`, `docs/SECURITY-MODEL.md`, `docs/VERIFYING-RELEASES.md`. `docs/ASSURANCE-CASE.md` is the security argument; keep it aligned with `SECURITY.md`. Update the owning topic guide and preserve README links when a procedure changes. Commands in these guides run from the repository root.

## Strong-key configuration

- `docs/STRONG-CRYPTO.md` owns the supported `GODEBUG=fips140=on` process/edge profile and migration; the optional Compose overlay adds only that setting. Preserve node/backup keys, database, recovery pin and credential paths.
- OIDC keeps coreos/go-oidc signature/claim checks. Its private JWKS transport matches the verifier's envelope/member parsing and refuses every RSA modulus below 2048 bits, bounded malformed inputs and redirects. `TestOIDCSigningKeyFloor` covers real signatures and case-variant members in both process modes. Configured-mode discovery/exchange requires direct HTTPS metadata endpoints and returns an explicit login failure. Preserve injected trust roots/timeouts.
- `internal/upstream/strong_crypto_test.go` runs the TLS peer in a separate ordinary-mode process so the selected client's key policy is what is tested. Keep strong acceptance and weak-leaf/trusted-root refusal evidence for both DoT and DoH.
- Run full ordinary and `GODEBUG=fips140=on` race suites; `python3 scripts/check-strong-crypto-compose.py` proves overlay preservation. Docker CI reuses its data volume to verify DNS after enabling the profile; review the selected profile's integration tests and local-data/restore checks before rollout. Health certificate opt-out is outside the profile; pinned Ed25519 replication and durable recovery remain supported.
