# IPv6 DNS advertisement (RDNSS) — design

Date: 2026-11-15. Owner approval: 2026-11-15 suite feature list. Status: building.

## Problem

On a dual-stack network the router advertises its own DNS server in its IPv6 router
advertisements (RAs). Clients often prefer it, so queries bypass KyDNS even with the
built-in DHCP server on. Android learns DNS only from RAs: it has no DHCPv6 client, so
DHCPv4 option 6 alone does not stop it using the router's resolver over IPv6.

KyDNS will optionally send its own RAs carrying only DNS information: an RDNSS option
(its own address) and a DNSSL option (the private domain), RFC 8106.

## What is sent

One ICMPv6 Router Advertisement (RFC 4861 §4.2), to `ff02::1`, hop limit 255, from the
interface's link-local address:

| Field | Value | Why |
|---|---|---|
| Cur Hop Limit | 0 | unspecified, so hosts keep their own |
| M, O flags | 0 | KyDNS runs no DHCPv6 |
| Router Lifetime | **0** | KyDNS is never a default router (RFC 4861 §6.3.4: hosts still process the options) |
| Reachable Time, Retrans Timer | 0 | unspecified |
| Source link-layer address | the interface MAC | RFC 4861 SHOULD |
| RDNSS (type 25) | one address, lifetime 1800 s | RFC 8106 §5.1 |
| DNSSL (type 31) | the private domain, lifetime 1800 s | RFC 8106 §5.2; omitted if the domain cannot be encoded |
| Prefix information | **none** | KyDNS never hands out addresses |

Timing follows RFC 4861 §6.2.4: MaxRtrAdvInterval 600 s, MinRtrAdvInterval 200 s, a
uniformly random interval between them; the first three advertisements at most 16 s
apart. A Router Solicitation (hop limit 255, code 0, ICMP-filtered to type 133) is
answered by a multicast RA after a random 0–500 ms, and never sooner than 3 s after the
previous one (MIN_DELAY_BETWEEN_RAS), so a flood of solicitations yields at most one RA
every 3 s. The RDNSS lifetime is 3 × MaxRtrAdvInterval (RFC 8106 §5.1 guidance). When
the feature is turned off, the interface changes, or KyDNS stops cleanly, one final RA
goes out with lifetime 0 on both options, which tells hosts to drop the address at
once (RFC 8106 §5.1).

## Threat model

| Concern | Handling |
|---|---|
| KyDNS becomes a router by accident | Router Lifetime is a constant 0, no prefix option exists in the code, M/O are 0. A test pins the bytes. |
| A hostile LAN host sends RAs too | Out of scope: RAs are unauthenticated on any LAN (RA Guard on the switch is the fix). KyDNS does not make it worse. |
| Solicitation flood used to make KyDNS flood the link | Responses are coalesced: at most one RA per 3 s regardless of input. Solicitations are not parsed beyond type, code, hop limit and length. |
| Off-link solicitation | Hop limit must be 255 (RFC 4861 §6.1.1), and the receiving interface must be the chosen one. |
| Advertising an address that disappears | Only a stable address: a configured one, else a ULA, else a stable global. Temporary (privacy), deprecated, tentative and DAD-failed addresses are never chosen, read from the kernel's per-address flags in `/proc/net/if_inet6`. Link-local is refused. |
| Advertising an address KyDNS does not answer on | Refused when `dns.listen` binds IPv4 only or a different IPv6 address. A warning when `allow_query` would refuse clients whose source address is in the advertised address's /64 (a global address outside the default private ranges). |
| Raw socket privilege | `CAP_NET_RAW`, granted to the packaged unit (below). The socket is ICMPv6 only, filtered to Router Solicitations on receive. |
| Two KyDNS nodes advertising | A replica never sends, like DHCP; it sends once promoted. |
| Privacy | Nothing is logged about solicitations or who sent them. The log carries the interface and KyDNS's own advertised address, as the DHCP start line does. |

## Where it runs

The same rules as the built-in DHCP server:

- Native install (`.deb`, `.rpm`, Raspberry Pi image), or Docker with
  `network_mode: host` running as root. The container image runs as uid 65532 with every
  capability dropped, and a non-root process cannot use an added capability, so the
  default Compose install cannot open the raw socket; KyDNS refuses with a message naming
  the native package.
- Compose bridge networking: the interface is a veth, which is refused before any socket
  opens, with the same message DHCP uses.
- One interface, chosen by the operator.

Packaging: `packaging/kydns.service` gains `CAP_NET_RAW` in `AmbientCapabilities` and
`CapabilityBoundingSet`. The `.deb` and `.rpm` ship that unit, and the Raspberry Pi image
installs the `.deb`, so all three get it with no other change. `RestrictAddressFamilies`
already allows `AF_INET6`. The capability is granted even while the feature is off; the
alternative (an operator-added drop-in) is owner question 1.

## Data stored

Three node-local columns on the `settings` row, never replicated (no `cv_` trigger
names them; `ApplySnapshot` keeps the local row's values; `PutDHCPSettings`, the replica
write path, writes them):

| Column | Default | Meaning |
|---|---|---|
| `rdnss_enabled` | 0 | Send RAs |
| `rdnss_interface` | `''` | The one interface |
| `rdnss_address` | `''` | Advertised address; empty means choose a stable one automatically |

Validation at the boundary: an enabled sender needs an interface; an address, if given,
must be a unicast IPv6 address without a zone, not link-local, loopback, multicast,
unspecified or IPv4-mapped. Host-state checks (interface exists, address present and
stable) run when the sender starts, like DHCP's.

## Routes and auth

| Route | Auth | Purpose |
|---|---|---|
| `POST /dhcp/rdnss` | session + CSRF | Save the three settings |
| `POST /dhcp/rdnss/suggest` | session + CSRF | Wizard: read the interface, propose the address; saves nothing |
| `GET /api/v1/rdnss/status` | admin token | Running, last error, chosen address, warnings |
| `GET /api/v1/rdnss/suggest?interface=` | admin token | The wizard's proposal |
| `PATCH /api/v1/settings` (existing) | admin token | Gains `rdnss_enabled`, `rdnss_interface`, `rdnss_address` |

The two web routes are exempt from the replica write gate, like the DHCP form, because
the settings are node-local. No route is anonymous, none makes an outbound request, and
the suggestion reads only local interface state.

The wizard is a second card on the DHCP tab (no new navigation item). It explains, before
the save button, that the router's own IPv6 DNS advertisement must be turned off, or
clients will use both resolvers and filtering will look intermittent.

## Defaults

Off. No interface. Automatic address. DNSSL is the private domain.

## Not built

- Prefix information, M/O flags, or any default-router role.
- Unicast replies to solicitations (multicast is allowed and simpler).
- Sending on more than one interface.
- RA Guard detection, or detecting the router's own RDNSS automatically.
- Docker non-root support (needs a capability a non-root container process cannot use).

## Owner questions

1. The packaged unit now grants `CAP_NET_RAW` to every install, even with the feature
   off. Default chosen: grant it, because the Raspberry Pi image is the main audience and
   asking those users to write a systemd drop-in defeats the feature. The marginal risk is
   small next to what a compromised KyDNS already controls (DNS and DHCP for the LAN). The
   alternative is an opt-in drop-in documented in `docs/RUNNING.md`.
2. Should a replica also advertise itself as a second RDNSS? Default chosen: no, the same
   rule as DHCP.
