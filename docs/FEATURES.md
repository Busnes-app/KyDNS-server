# Features

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## What works today

The admin listener exposes public `GET /healthz` as `ky.health/1` JSON for this
KyDNS instance. `GET /api/v1/healthz` is a compatibility alias with the same
response and five-second cache. The local database check returns HTTP 200 when
healthy or 503 when unavailable; the response gives status codes and never the
database error text. This endpoint does not probe DNS or other services. The
service checks remain separate at authenticated `/api/v1/health`.

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
- An IPv6 DNS advertisement, off by default: router advertisements that carry
  only KyDNS's address and the private domain (RFC 8106 RDNSS and DNSSL), so
  IPv6 clients, and Android in particular, use KyDNS too. KyDNS never becomes
  a router. See [below](#the-ipv6-dns-advertisement).
- Optional health checks over HTTP, HTTPS, and TCP, configured through the CLI
  or API, with status at `/api/v1/health`. The Services page focuses on names,
  addresses, aliases and proxy routing.
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
tab rather than being guessed at. A MAC belongs to one service. A reservation
for an address another device still holds takes it over, but the address is
probed first: while that device still answers for it, the reserved device gets
a dynamic address and moves at a later renewal.

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

### The IPv6 DNS advertisement

Android learns its DNS server only from IPv6 router advertisements, and other
clients often prefer the IPv6 resolver the router advertises, so on a
dual-stack network DHCP alone does not bring every query to KyDNS. Turn this on
in the **IPv6 DNS advertisement** card on the DHCP tab: type the interface,
press **Fill in from this interface** to see the address KyDNS would advertise,
then save. **Turn off your router's own IPv6 DNS advertisement (RDNSS) first,
keeping IPv6 addressing on**: if both advertise, clients use both resolvers and
names the router answers bypass KyDNS and its filtering.

What KyDNS sends, on the one interface you chose, to `ff02::1` from its
link-local address with hop limit 255:

- Router lifetime 0, no prefix information, M and O flags clear: it is never a
  default router and never numbers anyone.
- One RDNSS option with KyDNS's address and one DNSSL option with the private
  domain, both with a 1800-second lifetime (three times the 600-second maximum
  interval, per RFC 8106).
- An advertisement every 200–600 seconds, the first three at most 16 seconds
  apart (RFC 4861). A router solicitation is answered within half a second, but
  never sooner than 3 seconds after the previous advertisement, so a flood of
  solicitations cannot make KyDNS flood the link.
- When you turn it off, change the interface, or stop KyDNS cleanly, a last
  advertisement with lifetime 0, so clients drop the address at once.

**The address is a stable one.** An address you enter must be on the
interface and must not be temporary, deprecated or tentative. Left empty,
KyDNS picks a ULA (`fd00::/8`) first, because it survives your ISP changing
the global prefix, then a stable global address. Temporary (privacy) and
link-local addresses are never used. If `dns.listen` answers on IPv4 only or on
a different address, the advertisement refuses to start and says so. If the
address is a global one whose /64 `allow_query` does not cover, the card warns:
clients would query from that /64 and be refused.

**Where it runs:** the same places as the built-in DHCP server, and it also
needs `CAP_NET_RAW` for its ICMPv6 socket. The `.deb`, the `.rpm` and the
Raspberry Pi image grant it in their systemd unit. In Docker it needs
`network_mode: host` and a container running as root; the default Compose
install runs as uid 65532 with every capability dropped, and a bridge-mode
container (a veth interface) is refused with a message naming the native
package. A replica never advertises, like DHCP; the three `rdnss_*` settings
are node-local, so a standby is configured in advance and promotion starts it.

From the API, `GET /api/v1/rdnss/status` says whether it is running, the
address and why not, and `GET /api/v1/rdnss/suggest?interface=eth0` is the
wizard's proposal. The settings are `rdnss_enabled`, `rdnss_interface` and
`rdnss_address`.

Not verified on real devices: that every Android, iOS, Windows and Linux
release honours RDNSS from an advertisement with router lifetime 0. RFC 4861
§6.3.4 says hosts process the options regardless, and the CI test receives the
advertisement over a veth pair, but phones have not been tested.

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

An AI agent can explain why a name is blocked, manage allow and deny rules,
refresh lists and pause blocking for up to an hour over MCP; see
[AI agents](AI-AGENTS.md).

## Not yet

- **Automatic failover.** Replication is one primary, chosen by you. Nothing
  elects a new one, nothing discovers peers, and a replica never promotes
  itself. Losing the primary costs administration, not resolution: give clients
  both servers and they keep resolving.
- **IPv6 DNS advertisement in a non-root container.** The advertisement needs
  a raw ICMPv6 socket, which the default Compose install (uid 65532, every
  capability dropped) cannot open. Use the native package, or host networking
  as root. DHCPv6 is not offered either.
- **DNSSEC trust-anchor rollover (RFC 5011).** KyDNS validates DNSSEC itself
  ([Configuration](CONFIGURATION.md#validate-dnssec-locally)), from root keys
  built into the release. A new root key arrives with an update, not by itself.
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
