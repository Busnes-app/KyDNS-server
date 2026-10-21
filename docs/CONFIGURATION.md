# Configuration

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Configuration

**Three keys live in a file, and two more if you replicate. Everything else
lives in the database and is edited in the UI.**

Copy [`kydns.example.yaml`](../kydns.example.yaml) to `/etc/kydns/kydns.yaml`, or
point somewhere else with `--config`. These three are read from it at every
start, because KyDNS needs them before it has a database or a web UI:

| Setting | Default | Notes |
|---|---|---|
| `data_dir` | *required* | Holds the database and the bootstrap/setup tokens. Back it up. |
| `dns.listen` | `:53` | UDP and TCP. Port 53 needs root or `CAP_NET_BIND_SERVICE`. |
| `admin.listen` | `127.0.0.1:8053` | Web UI and JSON API. Plain HTTP; use a TLS-terminating proxy and restrict network access when binding beyond loopback. |

Two more are read from the file only if you run more than one server:
`replication.listen` and `replication.primary`. Setting both is a startup
error. See [Replication](REPLICATION.md#replication).

All five can also come from the environment — `KYDNS_DATA_DIR`,
`KYDNS_DNS_LISTEN`, `KYDNS_ADMIN_LISTEN`, `KYDNS_REPLICATION_LISTEN`,
`KYDNS_REPLICATION_PRIMARY` — which is how a container is configured without
mounting a file. The variable wins over the file, and startup logs which
variables it applied. A variable set to nothing is ignored rather than treated
as empty, so a blank field in a container template cannot switch off something
the file turned on. Setting `replication.listen` in the file and
`KYDNS_REPLICATION_PRIMARY` in the environment is the same startup error as
setting both in the file, and the message says which source each came from.

Changing one of those means restarting KyDNS. The Settings screen shows them
read-only, so you can see what the running server actually has.

Every other setting — the private domain, reverse zones, upstreams,
`allow_query`, Tailscale, TTLs, the cache bounds, the two logging opt-ins,
lease discovery, the built-in DHCP server, and the health-check intervals —
lives in the database. `kydns.example.yaml` still lists them, but only as
**first-run seed values**:
they populate a fresh database and are ignored on every start after that.
Editing them in the file later does nothing.

Edit them in one of three places, all of which go through the same validation
and the same write path:

```sh
kydns settings get                       # what the server is running
kydns settings set ttl=120 log_queries=true
kydns settings set allow_query=192.168.1.0/24,10.0.0.0/8
```

```sh
curl -H "Authorization: Bearer $KYDNS_TOKEN" $KYDNS_URL/api/v1/settings
curl -X PATCH -H "Authorization: Bearer $KYDNS_TOKEN" \
  -d '{"ttl":120}' $KYDNS_URL/api/v1/settings
```

`PATCH` merges: keys you leave out keep their value, so two people editing
different settings do not clobber each other. Settings are part of
`kydns export` and are applied by `kydns import`.

Everything applies the moment it is saved, with no restart and no dropped
queries: upstreams (which also flushes the cache), the private domain, reverse
zones, `allow_query`, `allow_tailscale`, the TTL, all four cache settings, both
log flags, the three health settings, the discovery interval,
`dhcp_lease_file` — the discovery poller is repointed at the new file in place —
and every `dhcp_*` key, which reconciles the built-in server's listener: it
starts, stops, or rebinds to match what you saved.

### Renaming the private domain

Services are stored by short name and follow the zone on their own. Manual
records are stored as full names, so changing `private_domain` moves them:
`printer.home.arpa.` becomes `printer.lan.example.`, and a CNAME or PTR
pointing into the old zone is repointed with it. The Settings screen lists
every record it would move and does nothing until you save a second time. The
records and the setting are written in one transaction, so a rename cannot
half-apply, and the cache is flushed because every cached name under the old
zone is now wrong.

Records outside the private zone are left alone — they are yours, not ours to
rewrite. Clients hold the old names until their TTL expires.

### Opening `allow_query` beyond your LAN

Adding a range outside loopback, RFC1918, ULA, link-local and CGNAT would make
KyDNS an open resolver, so it takes a deliberate second step: retype the
prefix in its canonical masked form to confirm it. In the settings form that
is the confirmation box, over the API it is `confirm_public`, and on the CLI
it is `--confirm-public`:

```sh
kydns settings set allow_query=192.168.0.0/16,203.0.113.0/24 \
  --confirm-public 203.0.113.0/24
```

You are asked once, when the prefix is new. A prefix already stored counts as
confirmed, so later saves of unrelated settings never ask again. While any
public range is configured, the dashboard and the Settings screen carry a
standing banner and KyDNS logs a warning naming the prefix at every start.
See [SECURITY.md](../SECURITY.md) for the full rule.

### Upgrading from a config-file deployment

- **An existing `kydns.yaml` keeps working.** On the first start after the
  upgrade its values seed the database, and from then on the database owns
  them. That includes a `dns.allow_query` that reaches past your LAN: it is
  grandfathered rather than refused, because refusing to start would take a
  working household's DNS offline over a file that was legal when it was
  written. It does log a warning naming the prefix at every start, and shows a
  standing banner, until you remove it.
- **Confirmation is asked once.** Adding a public prefix through the UI, the
  API or the CLI asks you to retype it that one time. It does not re-ask on
  every later save.
- **A backup containing a public prefix cannot be restored.** There is no
  confirmation path through import, so a document whose `allow_query` carries
  a public prefix the server is not already serving is refused. In replace
  mode that check runs *before* the destructive part, so the existing registry
  and settings are left exactly as they were rather than half-replaced. Edit
  the prefix out of the document, import it, then add the prefix back with
  `kydns settings set ... --confirm-public`.

### Upstream encryption

The upstream list is edited under Settings, one entry per line, and applies as
soon as it is saved. The scheme decides how much you trust the answer:

```text
tls://1.1.1.1:853              # DNS-over-TLS
https://9.9.9.9/dns-query      # DNS-over-HTTPS
udp://192.168.1.1:53           # plain DNS, opted into per upstream
```

The host must be an **IP address**. A hostname would need DNS to resolve it,
and KyDNS may be the thing resolving — on a machine that points at itself that
is a loop. Cloudflare, Quad9 and Google all present certificates valid for
their IPs, so nothing further is needed. When a provider's certificate needs a
hostname, put it after a `#`:

```text
tls://45.90.28.0:853#abc123.dns.nextdns.io
```

That sets the TLS server name while still dialling the address you gave.
Certificate verification is always on and there is no option to turn it off.

**With only encrypted upstreams, a query fails rather than falling back.** If
every one is unreachable, clients get `SERVFAIL` and the Settings screen names
the upstream and the error — usually a firewall blocking port 853. That is the
intended behaviour: silently dropping to plain DNS is exactly what an attacker
who blocks 853 is hoping for.

The escape hatch is one line. Add a `udp://` upstream and KyDNS will use it
when the encrypted ones fail. Answers it serves have the authenticated-data
flag cleared, because nothing authenticated them, and the dashboard carries a
banner for as long as the entry is there.

KyDNS is not a validating resolver. It does not verify signatures, hold a trust
anchor, or walk a chain of trust. It makes the path to a resolver that does all
of that private and tamper-proof, and passes that resolver's verdict through.

**Every upstream should be a validating resolver, and a DNSSEC rejection from
any of them is final.** When an upstream answers `SERVFAIL` with a DNSSEC
Extended DNS Error (RFC 8914: DNSSEC Bogus, Signature Expired, DNSKEY Missing
and the like), KyDNS returns that `SERVFAIL` to the client, keeps the error for
clients that speak EDNS, and does not ask the next upstream. Cloudflare, Quad9
and Google all send one. A resolver that rejects an answer without sending the
error looks like an outage, so KyDNS moves on to the next upstream, which may
not validate.

### Tailscale

Tailscale addresses are CGNAT (`100.64.0.0/10`), not RFC1918, so they are **not**
covered by the default `allow_query`. Leave "Allow Tailscale addresses" off
unless you use Tailscale — some ISPs also assign CGNAT addresses, which would
make the range a standing exposure on a WAN-facing interface. It is a checkbox
under Settings, or `kydns settings set allow_tailscale=true`, and it takes
effect at once.

While it is off, tailnet clients get `REFUSED`. KyDNS says so rather than
failing silently: the dashboard shows a banner, the Settings screen flags any
view that can never match, and the startup log warns.

To give one name a LAN answer and a tailnet answer, add a view for
`100.64.0.0/10` and give the service a second address tagged with it.

### Putting a service behind a reverse proxy

A service speaking plain HTTP behind a proxy that terminates TLS wants two
different addresses: clients should reach the proxy, monitoring should reach
the service.

Register the service at its **real** address, set the proxy address, and tick
"Send DNS to the proxy":

```sh
kydns service add kypost \
  --address 192.168.1.30 \
  --proxy 192.168.1.20 --via-proxy \
  --check http://192.168.1.30:8080/health
```

`kypost.home.arpa` now answers `192.168.1.20`, and so does every alias. The
reverse record for `192.168.1.30` still names `kypost.home.arpa`, so several
services behind one proxy each keep a correct PTR.

Point the health check at the service, not the proxy. A proxy returning 502 for
a dead backend still answers a prober perfectly, so checking the proxy tells you
only that the proxy is up.

Routing can be turned on or off after the service is created, which is how you
tell whether a problem is the application or the proxy: leave the address in
place and just flip the switch. The Services screen has a routing form on
each row — a proxy address field, a checkbox, and a Save button — that changes
only those two fields.

The same is true over the API: `PATCH /api/v1/services/{id}` merges the body
onto the existing service, so any field you leave out keeps its value. A body
of

```json
{"name": "grafana", "addresses": [{"address": "192.168.1.60"}]}
```

changes the name and address and leaves `aliases`, `check_url`,
`check_insecure`, `proxy_address`, and `route_via_proxy` untouched. To clear a
field explicitly, include it with an empty value (`"aliases": []`,
`"check_url": ""`); an `addresses` or `aliases` array you do provide replaces
the whole list rather than merging into it, so a routed address never
inherits a view tag from the one it replaced.

A service with both an IPv4 and an IPv6 address behind one proxy answers `A`
only: `--proxy` takes one address, so the `AAAA` query gets `NODATA` instead of
the service's own IPv6 address. Give the service just the address family the
proxy needs, or leave routing off if both must keep resolving.
