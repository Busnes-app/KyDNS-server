# Contributing to KyDNS

KyDNS is a Go server with a web UI and a CLI. Contributions should keep the
code and the product scope, design, security, and logging documents consistent.

## Before contributing

- Read [AGENTS.md](AGENTS.md) before editing.
- Read [SECURITY.md](SECURITY.md) before changing trust boundaries,
  replication, authentication, or data handling.
- Keep changes focused and avoid speculative implementation or dependencies.
- Update the owning documentation when behavior, responsibilities, or
  verification requirements change.

## Documentation changes

Use plain Markdown and link related requirements instead of duplicating them.
Keep `README.md`, `DESIGN.md`, `LOGGING.md`, and `SECURITY.md` aligned when a
change affects more than one concern. The blacklist behavior is owned by
[`docs/superpowers/specs/2026-08-12-kydns-blacklists.md`](docs/superpowers/specs/2026-08-12-kydns-blacklists.md).

## Verification

Every non-trivial logic change needs a runnable check in the same commit.

```sh
make test    # go test ./...
make build   # bin/kydns
```

CI additionally runs `gofmt -l`, `go vet`, a check that both release builds
(`make dist` and the Dockerfile's) give byte-identical binaries from two
directories, a `go mod tidy` cleanliness check, the tests under `-race`, a
cgo-free build, and a Docker job that resolves a local service name and a
public name end to end. The build must stay cgo-free:
the image is distroless, so the pure-Go SQLite driver is not optional.

The `browser/` suite runs Chromium against a fresh loopback server built from this
source, on desktop and phone viewports. It executes dashboard SVG charts, live
refresh failure/recovery and session-expiry handling, Appearance persistence/OS/
cross-tab behavior, and real setup and record forms. Run it with Go and Node 26:

```sh
cd browser
npm ci
npx playwright install --with-deps chromium
npm test
```

The existing CI `test` job and tag-release job run this suite and its dependency
audit before publishing. Dependency installation has a separate 20-minute budget;
the browser tests retain their own 10-minute limit. Only deterministic stats and
failure responses are mocked; server setup, authentication, templates and record
mutations are real. Fixtures
use temporary data, synthetic credentials and loopback listeners/upstreams; they
never target a deployed instance. This is targeted browser coverage, not proof
that most code branches, inputs or functionality are covered.

The supported [strong-key profile](docs/STRONG-CRYPTO.md) also runs the full race
suite with `GODEBUG=fips140=on` in CI. Run it for TLS/OIDC/profile changes;
`python3 scripts/check-strong-crypto-compose.py` verifies the optional overlay
with synthetic inputs and starts no containers. The Docker smoke reuses its
existing data volume after enabling the profile and verifies DNS again.

For documentation-only changes:

- check Markdown links and headings;
- search for stale project names or contradictory requirements;
- inspect the complete diff for unintended changes.

CI's `test` job and tag releases additionally run `scripts/restore-drill.sh`
(proves the restore runbook's Step 1 and its refusals against a throwaway 2-of-3 key)
and `scripts/backup-live-proof.sh` (starts a real server on a throwaway data dir and
proves pin, run, audit records and schedule over the API). Run both locally for backup
changes, together with `go test -race ./internal/backup ./internal/store`. The scripts
need Go, curl and sqlite3; neither contacts KyRecovery. The live proof defaults to
loopback DNS port 15353/admin port 18053; on shared machines select unused ports with
`KYDNS_LIVE_DNS` and `KYDNS_LIVE_ADMIN`. Leave `KYDNS_LIVE_DIR` unset for a fresh,
automatically removed directory. These checks must preserve
legacy password verification, WAL-safe snapshots, token-at-rest encryption, receipt
verification, recovery URL restrictions, and the stdin-only restore boundary.

## Coding standard

Contributions follow these style guides. CI enforces the parts a tool can
check, so a style failure is a red build, not a review comment.

| Language | Style guide | Enforced by |
|---|---|---|
| Go | [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) | `gofmt -l .` (must print nothing), `go vet ./...` and the `go mod tidy` check in the `test` job |
| Shared browser UI (`internal/web/static/ky-ui/`) | Generated from Busnes-app/ky-ui; never hand-edited | `node internal/web/static/ky-ui/check-vendor.mjs` in the `test` job |
| JavaScript (`internal/web/static/*.js`) and shell (`scripts/`, `packaging/`) | Match the surrounding file | No linter in CI; CodeQL scans the JavaScript |

Beyond what the tools check: the smallest correct change, the standard library
first, and a `ponytail:` comment on any deliberate shortcut, as
[AGENTS.md](AGENTS.md) sets out.

## Releases

The version has one source, the git tag; `make` and the Docker build stamp it
into the binary with `-X main.version`. Push a tag `vX.Y.Z` on `main` to
release: `release.yml` tests the tagged commit, builds the tarballs, `.deb`,
`.rpm` and Raspberry Pi image, attests them (identity
`release.yml@refs/tags/<tag>`), publishes the GitHub release and pushes the
`:<tag>` and `:<version>` images. [docs/VERIFYING-RELEASES.md](docs/VERIFYING-RELEASES.md)
shows users how to verify them and rebuild the binary. A manual run of `release.yml` is a dry run
unless `publish` is set, and `publish` refuses a branch, a tag that is not
`vX.Y.Z` or `vX.Y.Z.N`, and a tag that already has a release.

`dependency-release.yml` runs daily at 23:00 UTC. If Dependabot commits landed
on `main` since the newest release and the push CI run on `main`'s tip
succeeded, it tags the next `vX.Y.Z.N` (one per day however many PRs merged)
and dispatches `release.yml` on it with `publish`, since a tag created by
`GITHUB_TOKEN` raises no push event. Run by hand, it releases `main` without
the Dependabot condition. The `X.Y.Z` is the newest release's, so a new line
starts when you push `vX.Y.Z` by hand.

## AI-assisted contributions

Disclose AI assistance in the commit or pull request. State whether the work
was assisted, generated, or agentic, and record the checks a human performed.

## Code of conduct

All contributors must follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
