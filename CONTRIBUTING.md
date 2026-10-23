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

CI additionally runs `gofmt -l`, `go vet`, a `go mod tidy` cleanliness check,
the tests under `-race`, a cgo-free build, and a Docker job that resolves a
local service name and a public name end to end. The build must stay cgo-free:
the image is distroless, so the pure-Go SQLite driver is not optional.

For documentation-only changes:

- check Markdown links and headings;
- search for stale project names or contradictory requirements;
- inspect the complete diff for unintended changes.

Backup changes additionally run `go test -race ./internal/backup ./internal/store`,
`scripts/restore-drill.sh` (proves the restore runbook's Step 1 and its refusals against a
throwaway 2-of-3 key) and `scripts/backup-live-proof.sh` (starts a real server on a
throwaway data dir and proves pin, run and schedule over the API). They must preserve
legacy password verification, WAL-safe snapshots, token-at-rest encryption, receipt
verification, recovery URL restrictions, and the stdin-only restore boundary.

## Releases

The version has one source, the git tag; `make` and the Docker build stamp it
into the binary with `-X main.version`. Push a tag `vX.Y.Z` on `main` to
release: `release.yml` tests the tagged commit, builds the tarballs, `.deb`,
`.rpm` and Raspberry Pi image, attests them (identity
`release.yml@refs/tags/<tag>`), publishes the GitHub release and pushes the
`:<tag>` and `:<version>` images. A manual run of `release.yml` is a dry run
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
