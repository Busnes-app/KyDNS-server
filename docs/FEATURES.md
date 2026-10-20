# Features

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## What works today

The admin listener exposes public `GET /healthz` as `ky.health/1` JSON for this
KyDNS instance. `GET /api/v1/healthz` is a compatibility alias with the same
response and five-second cache. The local database check returns HTTP 200 when
healthy or 503 when unavailable; the response gives status codes and never the
database error text. This endpoint does not probe DNS or other services. The
service checks shown in the UI remain separate at authenticated `/api/v1/health`.

- First-class services, each with its own address records and aliases, plus
  derived reverse records. Manual A, AAAA, and CNAME records for anything a
  service doesn't cover.
- A configurable private domain, `home.arpa` by default.
- Per-subnet views: one name can answer differently on the LAN and over a VPN.
- A built-in DHCP server, off by default: one interface, one range,
  reservations that are services, and a setup wizard that fills the form in
  from the interface you pick. It advertises KyDNS as the DNS server, which
  is the point — plenty of routers will not let you change that. It refuses
  to start if another DHCP server is already answering, and it needs a
  native install or Docker with `network_mode: host`, because DHCP has to
  hear broadcasts. DHCP lease discovery from a dnsmasq lease file still
  works for anyone who already runs their own.
- Health checks over HTTP, HTTPS, and TCP, with status in the web interface.
- Upstream forwarding over DNS-over-TLS and DNS-over-HTTPS, with a cache,
  sequential failover, and single-flight.
- A web UI, a JSON API, and a CLI that talks to the API.
- Server settings — upstreams, the query ACL, TTLs, cache bounds, logging,
  discovery and health — edited from any of the three and applied without a
  restart. Only five keys are still in a config file.
- Linked-server replication, operable from the CLI and the web UI: pair two
  nodes by confirming a fingerprint the way you would an SSH host key, watch
  each replica's lag on the Replication screen, and promote a replica with one
  command when the primary is gone. A replica follows its primary over its own
  pinned TLS listener, keeps answering DNS when the primary is down, refuses
  administrative writes with the address of the node to make them on, and
  reports service health as unknown rather than showing you what it last heard.
  See [Replication](REPLICATION.md#replication).
- Built-in and custom DNS blacklists, one-off allow/deny rules, and a one-button
  filtering toggle. Filtering is enabled by default and never overrides local
  records.
- YAML and JSON import/export for backup and Git-based configuration.

### The built-in DHCP server

It is off until you turn it on, under **DHCP** in the web UI. Type the
interface, press "Fill in from this interface", and the range, the gateway and
a 24-hour lease time are proposed from what that interface actually has — the
upper half of its subnet, the host's default route. Nothing is saved until you
press Save. A range wider than 65536 addresses is refused on save, because the
lease table is sized by it; a range outside that interface's subnet is caught a
step later, when the listener refuses to start and the tab says why.

The DNS server it hands out is KyDNS itself. A second one is an optional field,
for a resolver to fall back on if this box is down.

It and `dhcp_lease_file` are mutually exclusive — a save that has both on is
refused, naming the one to clear.

**It refuses to start if it hears another DHCP server.** Before binding it
sends its own DISCOVER and listens two seconds for an answer, and a reply
refuses the start naming that server and the address it offered. A probe that
could not run refuses too: "we could not check" is not "all clear". One
checkbox gets past both, for the operator who deliberately runs two scopes; it
is off by default, and the tab names what was detected. Once running, the same
probe repeats every fifteen minutes and raises a banner, but never takes the
listener down — pulling DHCP out from under a working network is worse than
the conflict it would be reacting to.

**Reservations are services.** Give a service a MAC and the device holding that
MAC always gets that service's address:

```sh
kydns service add printer --address 192.168.1.40 --mac aa:bb:cc:dd:ee:ff
kydns service update 7 --mac aa:bb:cc:dd:ee:ff
kydns service update 7 --mac ""                     # give the reservation up
```

The address reserved is the service's one address inside the DHCP subnet, so a
service that also answers on a VPN keeps its per-view addresses. No address in
that subnet, or two, and the reservation is inactive and says so on the DHCP
tab rather than being guessed at. A MAC belongs to one service.

The lease table's **Reserve** button does the same thing in one click, but it
*creates* a service — it never updates one. A lease whose hostname already
names a service is refused with `name already exists`. Set the MAC on that
service instead, with `kydns service update` above or a `PATCH` to
`/api/v1/services/{id}`: the Services screen has no MAC field, so that is the
one reservation the web UI cannot make for you.

From the CLI, `kydns dhcp status` says whether the listener is running and why
not if it is not, and `kydns dhcp leases` prints the table.

**A standby replica can be configured in advance.** The `dhcp_*` settings are
node-local and never replicate, so a replica may set them — through the DHCP
tab, `PATCH /api/v1/settings`, or `kydns settings set` — even though every
other setting on a replica is still refused with the address of its primary. A
write that names a DHCP key and any other key is refused whole rather than
applied in part. The listener still does not start: a replica never serves
DHCP, whatever its settings say, and promotion starts it with no restart. A
promoted node comes up with an empty lease table and re-allocates from scratch,
which is what the ARP probe before each address new to it is for.

### Blacklist filtering

The web UI's Blacklists tab has a global on/off toggle, the built-in list
(StevenBlack's unified hosts file, MIT licensed), any custom lists an
operator adds by URL, and one-off allow/deny rules. Lists come in three
formats: plain domains, hosts-file syntax, and Adblock Plus-style rules.

Precedence is deny beats allow beats lists: a deny rule always blocks, an
allow rule always overrides a list, and lists only apply where no rule says
otherwise. Matching is on label boundaries, so a rule or list entry for
`ads.example` also blocks `sub.ads.example` but never `badads.example`.

A blocked name gets a local NXDOMAIN with the authoritative (AA) bit clear
and never reaches an upstream resolver. A KyDNS service or manual record can
never be blocked: the policy is only consulted after the authoritative
lookup declines to answer.

This is domain filtering, not traffic inspection, parental control, or a
malware guarantee.

## Not yet

- **Automatic failover.** Replication is one primary, chosen by you. Nothing
  elects a new one, nothing discovers peers, and a replica never promotes
  itself. Losing the primary costs administration, not resolution: give clients
  both servers and they keep resolving.
- **Local DNSSEC validation.** KyDNS trusts the upstream's verdict over an
  encrypted channel; it does not verify signatures itself. An `AD` bit from
  KyDNS means "the resolver we talked to privately said it validated," not
  "KyDNS checked the chain."
- **IPv6 DNS advertisement.** On a dual-stack network your router advertises
  itself as a DNS server over IPv6, and clients often prefer it — so some
  queries bypass KyDNS even with its DHCP server running. Turning off the
  router's IPv6 DNS advertisement fixes it. Doing it properly means KyDNS
  sending router advertisements itself, which it does not.
- Docker Compose discovery, and lease formats other than dnsmasq.
- Local TLS certificate issuance for private service names.

## Scope

KyDNS is a small DNS server, a service registry, a web UI, and opt-out DNS
blackhole filtering. It does not provide parental controls, device posture, or
traffic inspection.

## Repository status

v1 is implemented, and a single node is still the normal deployment. See
[the design spec](superpowers/specs/2026-08-11-kydns-v1-design.md) for the
architecture and the reasoning behind each decision, and
[the replication spec](superpowers/specs/2026-08-13-kydns-linked-server-replication-design.md)
for the second node.

```sh
make test    # every package
make build   # bin/kydns
```

CI runs `gofmt`, `go vet`, `go mod tidy`, the race-enabled test suite, a
cgo-free build, and a Docker job that resolves both a local service name and a
public name over the default DoT upstream.
