# Running

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Running it

```sh
kydns serve --config /etc/kydns/kydns.yaml
```

On first start with no admin account, KyDNS writes a setup token to
`<data_dir>/setup-token` (mode 0600). It is printed to the log too, except under
systemd, where the journal is readable by more than root. Open the admin listener, enter that
token, and choose a password. A bootstrap API token for the CLI is written to
`<data_dir>/bootstrap-token`.

The CLI reads `KYDNS_URL` (default `http://127.0.0.1:8053`) and `KYDNS_TOKEN`.
Everything except `serve` and `admin` goes through the JSON API, so the CLI
works on the host itself, or from any machine through the trusted HTTPS proxy
(`KYDNS_URL=https://<proxy>`). The CLI refuses to send the token over plain
`http://` to anything but loopback.

| Command | Does |
|---|---|
| `kydns serve` | Run the DNS and admin servers. |
| `kydns service add\|list\|update\|rm` | Manage services, their addresses and aliases (`update` only changes `--mac`). `--mac` reserves the address for a device in the built-in DHCP server. |
| `kydns record add\|list\|rm` | Manage manual A, AAAA and CNAME records. |
| `kydns view add\|list\|rm` | Manage per-subnet views. |
| `kydns token add\|list\|rm` | Manage API tokens. |
| `kydns blacklist status\|on\|off\|list\|add\|rm\|refresh\|allow\|deny\|rules\|unrule\|test` | Blackhole filtering: switch it on or off, manage block lists and allow/deny rules, refresh lists, and `test` which rule decides a name. |
| `kydns dhcp status\|leases` | Whether the built-in DHCP server is running, and the leases it has handed out. Read-only. |
| `kydns settings get` | Print the settings the server is running. |
| `kydns settings set k=v ...` | Change them. `--confirm-public <cidr>` for a public `allow_query` range. |
| `kydns replica invite\|list\|remove` | Manage the replicas this node serves. `invite` prints a pairing code and this node's fingerprint; confirm the fingerprint on the replica before entering the code. |
| `kydns export` / `kydns import` | Read or write the registry and settings as YAML or JSON. |
| `kydns backup-pin-key --public-key-file p --threshold k --total-shares n` | Pin the suite recovery public key by hand, for a suite with no KyRecovery. Write-once: a second, different key is refused. |
| `kydns backup-unpair` | Forget the KyRecovery URL and sealed token. The key pin stays. |
| `kydns backup-schedule --minutes n` | Set the automatic backup interval; `0` turns it off, the floor is 15. |
| `kydns deposit` | Run a backup now, to every configured destination. |
| `kydns export-capsule --out path` | Write a sealed capsule to a file. |
| `kydns backup-drill` | Seal and reopen a capsule with an ephemeral key; proves collection and SQLite integrity. |
| `kydns restore --capsule p --out dir` | Open a capsule with custodian shares read from stdin. See [docs/RESTORE.md](RESTORE.md). |
| `kydns admin reset-password` | Local recovery. Opens the database directly. |

### Forgot the admin password

```sh
kydns admin reset-password --config /etc/kydns/kydns.yaml
```

This is the one command that opens the database directly instead of going
through the API, because there is otherwise no way back in. It needs write
access to the database file — anyone who has that can already read every hash
and token in it, so it is not an extra way in. The reset immediately ends existing
sessions, revokes every API token, unlinks the local SSO identity, and disables
SSO unless it is installer-managed. Installer-managed SSO remains unchanged.
Sign in with the new password to create replacement tokens for integrations,
or restart KyDNS to generate a new bootstrap token.

In Docker, where the image has no shell:

```sh
docker compose run --rm -it kydns admin reset-password
```

### Docker

KyDNS uses a **LAN-attached Docker network**, giving the container its own LAN
address and port 53 without competing with host DNS listeners. `make setup`
detects interface/subnet/gateway; Compose creates the configured network if absent,
and `compose down` can remove a network it created. If the host already owns a
compatible shared network such as unRAID's `br0`, the existing-network path below
preserves it. This LAN DNS network is separate from the KyPost-owned suite bridge
`kypost-net`.

**The built-in DHCP server does not work on this install path.** It has to
hear client broadcasts, which is why it is supported only on a native install
or in Docker with `network_mode: host` (which needs a host sysctl, see "Four
things that bite") — not on a LAN-attached container network. The startup check refuses a bridge-mode container but not this one,
so enabling DHCP here gives you a listener that reports itself running and may
never hear a client. Leave DHCP to the router on this path, or install
natively.

```sh
cp .env.example .env                    # set KYDNS_IP, and KYDNS_NETWORK if not br0
cp kydns.example.yaml kydns.yaml
make setup                              # detects your interface, subnet, gateway
docker compose up -d
docker compose logs kydns | grep setup-token
```

`make setup` reads the host's default route and appends the interface, subnet
and gateway to `.env`. It only adds keys that are missing, so re-running it is
safe and it never overwrites a value you set by hand. Compose creates the
network itself on `up` — there is no separate network step.

Already have the network — unRAID's `br0`, or whatever AdGuard Home and Pi-hole
are on? Set `KYDNS_NETWORK` to its name. Compose adopts it, prints a warning
saying it did not create it, and leaves it in place on `docker compose down`.
Only a network compose created itself gets removed.

In `kydns.yaml` change two of the three file-owned settings from their
defaults: `data_dir: /var/lib/kydns`, and `admin.listen: "0.0.0.0:8053"`.
Leaving admin on `127.0.0.1` would bind the *container's* loopback, reachable
from nowhere. `dns.listen` is already right at `:53`.

`0.0.0.0` exposes plain HTTP on the container's LAN address. Before setup,
restrict port 8053 with network controls so only your TLS-terminating reverse
proxy can connect. Set `KYDNS_TRUSTED_PROXIES` in `.env` to the proxy's address
as KyDNS sees it (comma-separated IPs or CIDRs; `/0` is refused), and open
`/setup` through the proxy's `https://` address. This setting trusts forwarded
headers; it does not block direct access to `http://$KYDNS_IP:8053/`.

Set `KYDNS_ADMIN_HOSTNAMES` to the host names the admin UI is reached by
(comma-separated, no scheme or port, for example `kydns.home.arpa`). The UI,
the API and `/mcp` refuse any other `Host`, which stops a web page from
reaching them by re-pointing its own name at this server (DNS rebinding).
`localhost` and IP addresses always work. Left empty, a request through the
trusted HTTPS proxy is accepted on any name, because the proxy's certificate
already binds it; KyDNS warns about this at startup.

The image already has both set, in `kydns.docker.yaml` baked in at
`/etc/kydns/kydns.yaml`, so a `docker run` with no config mounted at all
starts and serves. The compose file mounts your `kydns.yaml` over it, which is
what makes the two settings above yours to get right. Nothing else needs to be
in that file: every other setting seeds the database on the first run and is
edited under Settings from then on.

Pick a `KYDNS_IP` inside that network's subnet, **outside the router's DHCP
pool**, and different from any address AdGuard Home or Pi-hole already holds.
Two DNS servers can each own port 53 at once when each has its own LAN
address, which is how you move one client at a time instead of the whole
house.

#### Unraid

On Unraid there is no YAML to edit at all. The three keys a single server needs
are the container template's job: one volume mapping for `data_dir`, and two
port mappings for `dns.listen` and `admin.listen`. The baked-in
`kydns.docker.yaml` already sets the two that need it, and the template's
mappings decide where they land on the host.

Replication is the fourth, and it has no port mapping to stand in for it. Add
it as a template **Variable** rather than mounting a config file: Edit the
container, *Add another Path, Port, Variable...*, Config Type **Variable**, Key
`KYDNS_REPLICATION_LISTEN`, Value `0.0.0.0:8443` for a primary — or
`KYDNS_REPLICATION_PRIMARY` with the primary's `host:port` for a replica.
Apply, which restarts the container, which is what picking up a file-owned key
takes anyway. On a `br0` address that port needs no mapping; it is already on
the container's own LAN address.

Set the container on your `br0` network so it gets its own LAN address, add
`KYDNS_TRUSTED_PROXIES` as a Variable holding your reverse proxy's address,
start it, read the setup token from the log, and do everything else —
upstreams, the private domain, filtering, `allow_query` — from the web UI,
through that proxy over HTTPS. Without that Variable the admin UI answers
every proxied request with 403; DNS itself keeps working.

The template's data path is a host directory under `appdata`, and Unraid
creates it owned by someone other than KyDNS's uid 65532, so the first start
exits with `permission denied`. Give the directory to KyDNS once, from the
Unraid terminal, with the container stopped:

```sh
chown -R 65532:65532 /mnt/user/appdata/kydns/data
```

Use the host path your template maps to `/var/lib/kydns` if it differs. The
same command fixes an upgrade from an image that ran as root.

#### Pre-creating it

Compose creates the network for you. Pre-create it only if you want it shared
with other containers or kept across `compose down` — compose adopts an
existing network rather than complaining:

```sh
docker network create -d ipvlan \
  -o parent=eth0 \
  --subnet 192.168.1.0/24 --gateway 192.168.1.1 \
  --ip-range 192.168.1.53/32 \
  br0
```

The `/32` range is deliberate. Give Docker the whole subnet and it will
eventually allocate an address your router later leases to a phone, which
presents as DNS failing at unpredictable hours.

`ipvlan` shares the host's MAC and only adds an address, so switch port
security and DHCP snooping have nothing to object to, and it is the driver
that survives WiFi. `macvlan` gives each container its own MAC, which matters
only if you want the router to hold a DHCP reservation for it — KyDNS uses a
static address, so it does not. Set `KYDNS_NET_DRIVER=macvlan` if you want it
anyway.

#### Four things that bite

- **The host cannot reach the container.** macvlan and ipvlan both deliberately
  block host-to-container traffic. Everything else on the LAN is fine; only the
  Docker host itself can't reach KyDNS. That matters if the host resolves
  through it. `docker-compose.yml` has the shim-interface fix.
- **Use the named volume for `data_dir`, not a host directory.** The container
  runs as uid 65532 with every capability dropped, so it cannot write into a
  directory owned by another user. A bind mount from your home directory fails
  with `unable to open database file`. Chown it to `65532:65532` if you need
  one.
- **Port 53 comes from a sysctl, not a capability.** A non-root process cannot
  use an added `NET_BIND_SERVICE`, so compose sets
  `net.ipv4.ip_unprivileged_port_start=0` in the container's own network
  namespace. Under `network_mode: host`, the DHCP path, there is no own
  namespace: Docker refuses the sysctl, so drop it, and the host's value
  applies. **An existing host-network install stops binding port 53 on
  upgrade** until you do one of two things: switch to the native package,
  which systemd gives `CAP_NET_BIND_SERVICE` alone, or lower the limit on the
  host, which lets every local user bind 53 and up:

  ```sh
  echo 'net.ipv4.ip_unprivileged_port_start=53' | sudo tee /etc/sysctl.d/60-kydns.conf
  sudo sysctl --system
  ```
- **The image has no shell.** It is distroless, about 29 MB. Read the first-run
  tokens from the volume rather than with `docker exec`:

```sh
docker run --rm -v kydns-server_kydns-data:/d busybox cat /d/setup-token
```

#### Upgrading from an image that ran as root

Images before the move to uid 65532 wrote the volume as root, and the new one
cannot open that database: it exits with `permission denied` and the `chown`
it needs. Chown it once, with KyDNS stopped; running it again is harmless. On
Unraid or any other host directory, chown that directory as in
[Unraid](#unraid); with the compose volume:

```sh
docker compose stop kydns
docker run --rm -v kydns-server_kydns-data:/d busybox chown -R 65532:65532 /d
docker compose up -d
```

Compose files of your own also need the `sysctls` entry from
`docker-compose.yml` and no longer need `cap_add: ["NET_BIND_SERVICE"]`.

#### WiFi

`ipvlan` works over WiFi, which is why it is the default. Some access points
with client isolation still interfere. Note that there is no `br0` over WiFi in
the literal sense — a station-mode interface cannot be enslaved to a Linux
bridge — but nothing here needs one: the network just has to exist under that
name, and an ipvlan network with `parent=wlan0` does.
