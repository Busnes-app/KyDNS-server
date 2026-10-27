# Assurance case

[Product overview](../README.md) · [Security policy](../SECURITY.md) · [Design](../DESIGN.md) · [Verifying releases](VERIFYING-RELEASES.md)

This is the argument that KyDNS meets its security requirements: what it protects, from whom,
where the trust boundaries are, and why the design and the code hold at each one. The detailed
rules live in [SECURITY.md](../SECURITY.md) and [LOGGING.md](../LOGGING.md); this page ties them
together and points at the evidence. Where a claim names code, the path is relative to the
repository root.

## What KyDNS must protect

KyDNS answers DNS for a home or homelab network, names its private services, filters with
blacklists, can serve DHCP, and can replicate to linked nodes. Its security requirements:

1. **DNS clients cannot change anything**, and only the networks the operator lists get answers.
2. **Only the administrator administers.** The UI, `/api/v1` and the CLI need an authenticated
   administrator; a hostile web page in the administrator's browser cannot act for them.
3. **AI agents get only what they were granted:** read or write on DNS data, the query log only by
   a separate grant, and never settings, upstreams or listeners.
4. **Upstream answers and remote lists are untrusted** and cannot override local records or run
   code.
5. **Secrets at rest stay secret:** the admin password, tokens, the KyRecovery token and node
   keys.
6. **Replication cannot be joined or steered** by anyone but an operator who compared fingerprints.
7. **Private data stays private:** query history is off by default, client IPs are a separate
   opt-in, and no credential reaches a log.

## Threat model

| Attacker | Can | Wants |
|---|---|---|
| Device on the LAN | Send DNS queries and DHCP packets | Change records, shadow a service name, exhaust the lease range |
| Internet host, if the resolver is exposed | Send DNS queries | Use KyDNS as an open resolver or amplifier |
| Hostile web page in the administrator's browser | Make the browser send requests to the admin listener, or rebind a name to it | Read tokens or change settings |
| Network client reaching the admin listener | Send any HTTP request | Guess the password, get a session |
| AI agent with a token, or whoever can prompt it | Call the MCP tools | Exceed its grant, read the query log, change settings |
| Upstream resolver, list provider or path attacker | Answer queries, serve list contents | Poison answers, override local names |
| Rogue node | Try to pair or pull | Join replication, receive the registry |
| Someone with a copy of the data directory | Read `kydns.db` and the keys | Recover tokens or the KyRecovery credential |

Out of scope: an attacker with write access to the data directory (it is the trust boundary, as
SECURITY.md says), an attacker on the host, and a malicious administrator.

## Trust boundaries

1. **DNS clients → KyDNS.** Queries outside `allow_query` and non-IN classes get `REFUSED`
   (`internal/dnsserver/server.go`). Widening `allow_query` to a public prefix needs the canonical
   prefix retyped and leaves a standing banner and a startup warning (SECURITY.md, "The
   `allow_query` guardrail").
2. **Browser → admin listener.** `TransportGate` (`internal/web/middleware.go`) serves the admin UI
   only to a loopback peer or through a trusted proxy that reports HTTPS. It refuses a Host that is
   not this server's (DNS rebinding) and refuses cross-origin state changes and preflights, using
   `Origin` and `Sec-Fetch-Site` (`internal/web/origin.go`). No route sends CORS headers. UI forms
   also need the session's CSRF token (`requireCSRF`).
3. **AI agents → `/mcp`.** Agent tokens have their own scopes (`mcp:read`, `mcp:write`) and a
   separate query-log grant. `/mcp` refuses any request with an `Origin`. Agent tokens never
   authenticate `/api/v1`, and admin tokens never authenticate `/mcp` (`internal/adminapi/mcp.go`).
4. **KyDNS → upstreams and list providers.** DoT and DoH verify certificates
   (`internal/upstream/dot.go`). Plain UDP is a per-upstream opt-in. Blacklists are fetched over
   HTTPS only, with redirects followed only while they stay HTTPS, and a 32 MB body cap
   (`internal/policy/fetch.go`).
5. **Node ↔ node.** Replication uses TLS 1.3 with each side pinning the other's Ed25519 key
   fingerprint instead of a CA (`internal/replica/transport.go`). Peers are enrolled with a one-time
   code and an operator-compared fingerprint.
6. **LAN devices → DHCP.** Packets from any device are parsed; a malformed one is dropped. Hostnames
   are cut to one safe DNS label (`sanitizeHostname`, `internal/dhcpd/server.go`) and can never
   shadow a service, alias or manual record.
7. **KyDNS → its data directory.** `kydns.db` is created `0600` and tightened if looser
   (`internal/store/store.go`). Key files load through `ky-primitives/keyfile`, which refuses a
   key file that is not `0600`.

## Secure design principles applied

- **Least privilege.** The container runs as uid 65532 with every capability dropped; the systemd
  unit runs as `kydns` with only `CAP_NET_BIND_SERVICE` (both checked in CI). Agent tokens are
  read-only unless an administrator grants write.
- **Fail-safe defaults.** The admin listener binds `127.0.0.1:8053`. Query logging and client-IP
  logging are off. `KYDNS_TRUSTED_PROXIES` is empty and refuses `/0`. Plain-UDP upstreams are
  opt-in. Private KyRecovery destinations are refused unless the operator allows them.
- **Complete mediation.** Every settings change goes through one path: validate, persist, rebuild,
  apply, all or nothing (`internal/settings`). The config file only seeds the first run, so editing
  it later changes nothing.
- **Economy of mechanism.** In-memory sessions (never in the database or backups), one origin gate
  for UI, API and MCP, and shared `ky-primitives` code for password hashing, key files and backup
  sealing.
- **Separation of privilege.** Admin tokens, agent tokens and the query-log grant are separate
  credentials. A replica refuses administrative writes.
- **Open design.** Only published algorithms: Argon2id, AES-256-GCM, Ed25519, SHA-256, TLS 1.2 and
  1.3. The code and this argument are public.

## Supported key-strength configuration

[STRONG-CRYPTO.md](STRONG-CRYPTO.md) defines the supported process/edge profile,
all enabled key/TLS boundaries and data-preserving migration. OIDC always
refuses RSA signing keys below 2048 bits; the optional process profile filters
TLS chain keys and refuses unsafe OIDC endpoints/redirects. It is not a claim
about a live installation or FIPS certification.

## Common weaknesses and how they are countered

| Weakness | Counter | Where |
|---|---|---|
| Open resolver and amplification | `allow_query` ACL answers `REFUSED` outside the listed ranges; a public prefix needs a retyped confirmation | `internal/dnsserver/server.go`, `internal/settings` |
| Credential guessing | Argon2id from `ky-primitives/password`; an exponential per-source delay (100 ms doubling to 30 s, IPv6 by /64) that never locks the account | `internal/auth/password.go`, `internal/web/auth_handlers.go` |
| Session theft | Random session IDs from `crypto/rand`, held only in memory; `HttpOnly`, `SameSite=Lax`, `Secure` over TLS; idle and absolute expiry | `internal/auth/session.go` |
| Session outliving its identity-provider login | Back-channel logout token verified (signature, issuer, audience, expiry, `iat`, logout event, no `nonce`) before ending the sessions of its issuer's `sid`, or of its subject issued before it; the `sid` or the subject's cutoff is fenced so an in-flight callback cannot revive it | `internal/auth/backchannel.go`, `internal/auth/session.go`, `TestVerifyLogoutToken`, `TestBackchannelLogoutEndsTheSSOLogin`, `TestBackchannelLogoutDiscoversOncePerSettings` |
| CSRF, cross-origin requests and DNS rebinding | Host allow-list, `Origin`/`Sec-Fetch-Site` refusal of cross-origin state changes, CSRF token on forms, no CORS | `internal/web/origin.go`, `internal/web/middleware.go` |
| Token theft from storage | API and agent tokens stored as SHA-256 hashes (`registry.HashToken`); the KyRecovery token sealed with AES-256-GCM under a node-local `0600` key | `internal/registry/registry.go`, `internal/backup/backup.go` |
| SQL injection | `database/sql` with `?` parameters; the only formatted statements write `PRAGMA user_version` from an integer, and the list query joins only a column constant | `internal/store` |
| Cross-site scripting | Server-side rendering with `html/template` (no `template.HTML` conversions); `static/theme.js` builds the theme picker with DOM calls and `textContent`, with no `innerHTML` | `internal/web/render.go`, `internal/web/static/theme.js` |
| Untrusted DHCP input | Bounded range, sanitised hostnames, reserved names refused, RELEASE honoured only from the leaseholder; `FuzzHandle` exercises the packet handler | `internal/dhcpd/server.go`, `internal/zone/snapshot.go`, `internal/dhcpd/server_test.go` |
| Hostile list contents | HTTPS only, a 32 MB cap and a ceiling of 2^21 parsed entries; the blacklist is consulted only for names the local zones do not answer, so a list cannot blackhole a local service | `internal/policy/fetch.go`, `internal/policy/parse.go`, `internal/dnsserver/server.go` |
| Rogue replica | TLS 1.3, pinned Ed25519 fingerprints, operator-approved pairing, literal IP:port peers only | `internal/replica` |
| Secrets in logs | A logging contract; the setup token goes to a `0600` file (and the log, which SECURITY.md tells operators to restrict) | [LOGGING.md](../LOGGING.md), `internal/app/serve.go` |
| Vulnerable dependencies | Dependabot; a daily dependency release ships the updates | `.github/dependabot.yml`, `.github/workflows/dependency-release.yml` |
| Supply-chain tampering | Every GitHub Action pinned by commit; base images pinned by digest; every release file and image attested by `release.yml`; the binaries rebuild to the same bytes | [VERIFYING-RELEASES.md](VERIFYING-RELEASES.md) |

## Residual risk

- The admin listener speaks plain HTTP; beyond loopback it needs a TLS-terminating proxy.
- KyDNS is not a validating resolver; an `AD` bit is the upstream's word. A `SERVFAIL` without an
  Extended DNS Error fails over to the next upstream, which may not validate (SECURITY.md, "Known
  limitations").
- Health checks may opt out of certificate verification per service (`check_insecure`), which suits
  self-signed homelab services but means such a check proves reachability, not identity.
- The browser CSP sets only `frame-ancestors 'self'`. Script injection is countered by template
  escaping alone, not also by a `script-src` policy.
- Login is slowed per source address but never locked. An attacker spread over many addresses is
  slowed only by Argon2id.
- `govulncheck` does not run in CI. Dependabot alerts are the only automated check against the
  Go vulnerability database.
- The `:latest` and `:<commit sha>` images from `main` carry no attestation; release tags do.
- No Ky product has had an independent security audit.

## Evidence

- **Tests:** `go test -race ./...` on every push and pull request, including `hardening_test.go` in
  `internal/auth` and `internal/web`, the origin and MCP access-matrix tests, and the `FuzzHandle`
  seed corpus. CI also installs the `.deb` and `.rpm` on real systemd (Ubuntu on amd64 and arm64,
  Fedora, Rocky 9), proves the non-root user, the hardening directives and the `0700`/`0600` modes,
  and runs the container with all capabilities dropped and resolves names end to end.
- **Backup and restore:** CI's `test` job and tag releases run `scripts/restore-drill.sh`
  and `scripts/backup-live-proof.sh`. The disposable fixtures prove stdin-only 2-of-3
  restoration and its refusal paths, write-once key pinning, a local capsule with mode `0600`,
  audit records, and schedule changes through the real server. They do not test live
  KyRecovery pairing/deposits or establish most-branches/input/functionality coverage.
- **Browser behavior:** CI's publication-gating `test` job and tag releases run
  `browser/` Chromium regressions on desktop and phone viewports, against a current-source
  loopback server. Real setup/session/record forms, chart geometry and gaps, refresh
  errors/recovery/401 and Appearance storage/OS/cross-tab behavior are exercised.
  Stats data and failures are deterministic fixtures; broader branch/input coverage,
  live SSO, real DHCP and production deployment are outside this suite.
- **Static analysis:** CodeQL (GitHub default setup: Go, JavaScript/TypeScript, Actions) and
  `go vet`.
- **Dynamic analysis:** the race detector, the DHCP fuzz target's seeds, and the end-to-end
  package and container jobs.
- **Releases:** attested files and images, and repeatable binaries ([VERIFYING-RELEASES.md](VERIFYING-RELEASES.md)).

This case is updated when a trust boundary or a security requirement changes. A pull request that
moves one must update this page and [SECURITY.md](../SECURITY.md) in the same change.
