# KyDNS standalone installation

## Before you start

These steps are for a **new, independent installation** from this repository. Use the
README’s upgrade/restore procedure for an existing instance. Keep its volumes, keys,
configuration and Compose overlays; do not replace its environment files with examples.

Use Linux with Docker Engine and Docker Compose 2.24 or newer, Git, and permission to use Docker.
Choose a release/revision and use its matching Compose files and documentation. Image tags
can move: follow the README/Compose attestation and digest-pinning procedure before relying
on a published image. Source builds use the separate build overlay where provided.

Clone this repository, enter its directory, then run the product steps below:

```sh
git clone https://github.com/Busnes-app/KyDNS-server.git
cd KyDNS-server
```

Each service needs its own directory and persistent storage. Several products use host
port 8080 by default; install them on separate hosts or configure distinct host ports and
matching public origins. For remote users, provide a TLS reverse proxy and a hostname you
control. Trust only the proxy’s actual peer address, and forward the original scheme/client
address according to the product’s proxy guide.

## Choose native or container installation

KyDNS runs without any other Ky product. You need an unused address on your LAN, permission
to serve **UDP and TCP port 53**, and a TLS proxy or SSH tunnel for its administration UI.
Keep your current resolver until a test client has verified the replacement. Enable DHCP
only on a native or appropriately configured host-network install, with one deliberate
DHCP authority for the LAN.

**Native package:** choose a release `.deb` or `.rpm` for your architecture and follow
[package installation](docs/PACKAGES.md#install-from-a-package), including attestation verification
and checking port 53 before starting systemd. Read `/var/lib/kydns/setup-token` privately,
tunnel port 8053 over SSH, and create the administrator at `/setup`. The native UI defaults
to loopback; publishing it requires TLS and network access controls.

**Docker:** the repository builds the image locally. Prepare your configuration:

```sh
[ -e .env ] || (umask 077; cp .env.example .env)
[ -e kydns.yaml ] || cp kydns.example.yaml kydns.yaml
chmod 600 .env
```

Set `KYDNS_IP` inside your LAN subnet but outside DHCP allocation, and review
`KYDNS_NETWORK`, interface, gateway and subnet. In `kydns.yaml` set
`data_dir: /var/lib/kydns` and `admin.listen: "0.0.0.0:8053"`. Set
`KYDNS_TRUSTED_PROXIES` to your proxy peer address. Restrict direct admin-port access to
that proxy **before** setup; the trust setting alone does not restrict connections.

```sh
make setup
docker compose config --quiet
make up
docker compose logs kydns
```

Read the setup token privately, open the proxy’s HTTPS `/setup`, and create an admin.
Compose creates a missing LAN network or adopts one with the configured name. An existing
shared network is not deleted on `down`; pre-create it if its lifetime must be independent
of this stack. The named `kydns-data` volume is owned by UID 65532.

From a **different LAN machine** (host-to-ipvlan/macvlan traffic needs the documented shim),
substitute the chosen address and an actual registered name:

```sh
dig @192.168.1.53 service.home.arpa A
dig +tcp @192.168.1.53 service.home.arpa A
```

Also verify upstream resolution/filtering and access restrictions before changing a
client’s DNS settings. DHCP is not supported on this LAN-attached Compose path. See
[Docker networking and pitfalls](docs/RUNNING.md#docker), [configuration](kydns.example.yaml)
and [restore](docs/RESTORE.md). The prior nine-app installation campaign did not exercise
KyDNS; its screenshots/results are not DNS acceptance evidence.

## Keep it recoverable

Preserve the database **and deployment keys**, not just the image. Configure recovery using
the linked restore runbook, retain an independent copy, and complete a scratch restore with
custodian cards before treating recovery as verified. A synthetic drill uses a throwaway
key and does not prove that the real cards are available or correct.

For upgrades, back up first, read the chosen release’s migration notes, and reuse the same
Compose project, volume mounts and overlay selection for every command. `docker compose
pull` followed by `docker compose up -d` updates a published-image installation; a digest pin
must be deliberately changed after verification. A source install must keep its build
overlay and rebuild. `docker compose down -v` deletes managed volumes: it is not an upgrade.

## What verification means

The startup checks above prove only the checks they perform. Use the README’s development
section and the repository CI workflow for source/build tests; a passing health response or
screenshot is not a substitute. Record the revision, commands, environment and skips when
reporting results. Design plans and dated evidence remain records of their stated scope.
