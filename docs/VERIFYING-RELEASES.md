# Verifying releases

[Product overview](../README.md) · [Packages](PACKAGES.md) · [Assurance case](ASSURANCE-CASE.md)

A KyDNS release `vX.Y.Z` (or `vX.Y.Z.N` for a dependency-only release) is a GitHub release with
these files, all built by `release.yml` from the tag:

- `kydns_X.Y.Z_linux_amd64.tar.gz` and `kydns_X.Y.Z_linux_arm64.tar.gz`: the binary, the example
  configuration and the licence;
- `kydns_X.Y.Z_amd64.deb`, `kydns_X.Y.Z_arm64.deb`, `kydns-X.Y.Z-1.x86_64.rpm` and
  `kydns-X.Y.Z-1.aarch64.rpm`: the same binaries as the tarballs, packaged;
- `kydns_X.Y.Z_rpi64.img.xz`: a Raspberry Pi OS Lite image with the arm64 package staged;
- `SHA256SUMS`: the SHA-256 of every file above.

The same job pushes the container image `ghcr.io/busnes-app/kydns-server`, tagged both `vX.Y.Z` and
`X.Y.Z`, for `linux/amd64` and `linux/arm64`. This page shows how to check that a file or image was
built by this repository's release workflow from the tagged commit, and how to rebuild the binary
yourself and get the same bytes.

## What is signed, and by whom

- `release.yml` publishes a SLSA build-provenance attestation, through GitHub's Sigstore
  integration, for every release file listed above, including `SHA256SUMS`, and for the image.
- There is no long-lived signing key to download. The signature is tied to a workflow identity,
  and that identity is the thing you check:
  `https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v<version>`.
  `gh` fetches Sigstore's public trust root itself.
- The `:latest` and `:<commit sha>` images that `ci.yml` pushes from `main` carry **no**
  attestation. Run a release tag if you want to verify what you run.
- No release publishes a software bill of materials. Every Go binary records the modules it was
  built from, with their checksums; see [Read the dependency list](#read-the-dependency-list).

## Verify a release file

You need `gh` (logged in). Name the version yourself. Verifying `SHA256SUMS` and then checking each
file against it covers every file in the release; the check also requires the attestation to name
the commit the release tag points at.

```bash
v=<release version, e.g. 0.4.3>
gh release download v$v --repo Busnes-app/KyDNS-server --pattern SHA256SUMS --pattern "kydns_${v}_linux_amd64.tar.gz"
sha=$(gh api repos/Busnes-app/KyDNS-server/commits/refs/tags/v$v --jq .sha) \
  && gh attestation verify SHA256SUMS --repo Busnes-app/KyDNS-server \
       --cert-identity https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v$v \
  && [ "$(gh attestation verify SHA256SUMS --repo Busnes-app/KyDNS-server \
       --cert-identity https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v$v \
       --format json --jq '.[0].verificationResult.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit')" = "$sha" ] \
  && sha256sum --check --ignore-missing SHA256SUMS \
  && echo "OK: release v$v files, built from $sha"
```

Download whichever files you need with more `--pattern` arguments; `--ignore-missing` checks only
the ones present. Each file is also attested on its own, so
`gh attestation verify <file> --repo … --cert-identity …` works for a single package too, as
[PACKAGES.md](PACKAGES.md) shows.

## Verify a release image

You also need `docker` with buildx (`imagetools` talks to the registry; it does not need a running
daemon). The check resolves the tag to a digest first, then verifies that digest.

```bash
v=<release version, e.g. 0.4.3>
sha=$(gh api repos/Busnes-app/KyDNS-server/commits/refs/tags/v$v --jq .sha) \
  && d=$(docker buildx imagetools inspect ghcr.io/busnes-app/kydns-server:$v --format '{{.Manifest.Digest}}') \
  && gh attestation verify "oci://ghcr.io/busnes-app/kydns-server@$d" --repo Busnes-app/KyDNS-server \
       --cert-identity https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v$v \
  && [ "$(gh attestation verify "oci://ghcr.io/busnes-app/kydns-server@$d" --repo Busnes-app/KyDNS-server \
       --cert-identity https://github.com/Busnes-app/KyDNS-server/.github/workflows/release.yml@refs/tags/v$v \
       --format json --jq '.[0].verificationResult.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit')" = "$sha" ] \
  && echo "OK: ghcr.io/busnes-app/kydns-server@$d is release v$v, built from $sha"
```

Then run that digest, not the tag: `ghcr.io/busnes-app/kydns-server@$d`. The digest names the
multi-architecture index, so it works on both `amd64` and `arm64`.

Any failure means: do not install that file or run that image. A different workflow, a branch
instead of the tag, or another repository all fail the `--cert-identity` or `--repo` check.

## Read the dependency list

```bash
tar -xzf kydns_${v}_linux_amd64.tar.gz && go version -m kydns_${v}_linux_amd64/kydns
```

This prints the Go version, every module compiled in with its `go.sum` checksum, the build
settings, and, for the tarball and package binaries, the commit they were built from
(`vcs.revision`).

## Rebuild the binary and compare

KyDNS builds with `-trimpath`, so the same commit, Go version and version string give the same
binary on any machine and in any directory. CI checks this on every push, for both release paths.
The two paths give different binaries on purpose: the tarballs and packages are built from a git
checkout, so Go records the commit in them, while the image is built from a context with no
`.git`. Rebuild each one the way it was built.

**The tarball and package binary.** From the directory holding the tarball you downloaded above:

```bash
git clone --branch v$v https://github.com/Busnes-app/KyDNS-server.git kydns-v$v && cd kydns-v$v
GOTOOLCHAIN=$(sed -n 's/^toolchain //p' go.mod) make dist VERSION=$v
sha256sum dist/kydns_${v}_linux_amd64/kydns ../kydns_${v}_linux_amd64/kydns
```

The two hashes must match. `make dist` builds `arm64` too, to compare against the other tarball.
The `.deb` and `.rpm` carry the tarball's binary as `/usr/bin/kydns`. The packages and the
Raspberry Pi image themselves are not rebuilt bit for bit (archive timestamps, the Raspberry Pi OS
base); their attestation is what vouches for them.

**The image binary.** From the clone, in the same shell (`$d` from the image check):

```bash
mkdir ../src && git archive v$v | tar -x -C ../src
go_version=$(sed -n 's/^FROM .*golang:\([0-9.]*\)-.*/\1/p' Dockerfile)
(cd ../src && GOTOOLCHAIN=go$go_version CGO_ENABLED=0 GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X main.version=$v" -o ../kydns-image-from-source ./cmd/kydns)
cid=$(podman create --platform linux/amd64 ghcr.io/busnes-app/kydns-server@$d) \
  && podman cp "$cid:/usr/local/bin/kydns" ../kydns-from-image && podman rm "$cid" >/dev/null
sha256sum ../kydns-image-from-source ../kydns-from-image
```

The two hashes must match. For `arm64`, set `GOARCH=arm64` and `--platform linux/arm64`. The
image is built with the Go version the Dockerfile names, the tarballs with the `toolchain` line in
`go.mod`; the commands read each from its source. They use Podman; with Docker, write `docker` for
`podman`. The distroless layers around the binary are not rebuilt; the binary is what you are
checking.
