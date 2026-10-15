# syntax=docker/dockerfile:1

# Build stage. CGO stays off so the pure-Go SQLite driver is used and the
# binary runs on a distroless base with no libc.
#
# It pins itself to the *builder's* platform and cross-compiles with GOARCH.
# Go does that natively; letting buildx emulate an arm64 toolchain under QEMU
# instead would build the same binary an order of magnitude slower.
#
# Both bases are pinned by digest; the tag is for the reader and Dependabot.
FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build

WORKDIR /src

# Dependencies first, so a source-only change does not re-download them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X main.version=$VERSION" -o /out/kydns ./cmd/kydns

# The runtime image has no shell to chown with, so the data directory is made
# here and copied over already owned by the nonroot user. 0700, as the
# package install leaves it. COPY of a directory copies its contents, so the
# parent is copied to carry kydns/ itself, mode and all.
RUN mkdir -p /out/lib && mkdir -m 700 /out/lib/kydns

# Runtime stage. distroless/static carries CA certificates, which HTTPS health
# checks need, and nothing else — no shell, no package manager.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/kydns /usr/local/bin/kydns
COPY --from=build --chown=65532:65532 /out/lib/ /var/lib/
COPY kydns.example.yaml /usr/share/kydns/kydns.example.yaml

# A working configuration, so a fresh container starts and serves with no
# files prepared on the host. The image has no shell and cannot write one at
# startup. Mount your own file over this path to replace it.
COPY kydns.docker.yaml /etc/kydns/kydns.yaml

# The database and the first-run tokens live here. Mount a volume over it.
VOLUME ["/var/lib/kydns"]

# Documentation only: with host networking these are not published, they are
# simply the ports the process binds.
EXPOSE 53/udp 53/tcp 8053/tcp

# The base already defaults to nonroot; spelled out so it cannot drift. Ports
# below 1024 need net.ipv4.ip_unprivileged_port_start=0 (see docker-compose.yml).
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/kydns"]
CMD ["serve", "--config", "/etc/kydns/kydns.yaml"]
