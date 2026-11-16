# Local DNSSEC validation — design

Date: 2026-11-15. Owner approval: 2026-11-15 suite feature list. Status: building.

## Problem

KyDNS forwards to upstream resolvers and passes on their AD bit. "Authenticated"
from KyDNS means "the resolver we talked to says it validated". An upstream that lies,
a plaintext upstream on a hostile path, or a DoT/DoH upstream that does not validate
all leave clients trusting unchecked data. This change makes KyDNS check the DNSSEC
chain itself, from the root trust anchors, and decide AD and SERVFAIL on its own.

## Threat model

| Adversary | Today | With local validation |
|---|---|---|
| On-path attacker against a `udp://` upstream | Can forge any answer; AD is cleared, so clients see "unsigned" | Forged answers in signed zones are bogus and become SERVFAIL; unsigned zones stay forgeable (DNSSEC cannot help) |
| Upstream that lies or is compromised (any transport) | Fully trusted, AD included | Can only deny service: it cannot make bogus data look secure, and it cannot strip signatures from a signed zone without the answer becoming bogus |
| Attacker who wants to downgrade a signed zone to "insecure" | n/a | Needs a signed proof that the DS does not exist, which it cannot forge |
| KeyTrap-style CPU exhaustion (CVE-2023-50387, CVE-2023-50868) | Upstream's problem | Bounded: at most 8 signature checks per RRset, 64 per answer, and NSEC3 iteration counts above 150 are treated as insecure (RFC 9276) |
| Wrong system clock (Raspberry Pi has no RTC) | Upstream's problem | Signature time checks are skipped while the clock is provably unsafe; see "Clock safety" |
| LAN client | Asks questions | Unchanged: the ACL, views and policy run before validation |

Out of scope: an attacker who controls the root zone's keys, or who can replace the
KyDNS binary (the anchors ship inside it).

## Choice of implementation

Requirement: use miekg/dns's signature verification; write no cryptography.

| Option | Fit | Verdict |
|---|---|---|
| `github.com/nsmithuk/resolver` | A full iterative resolver from the root. It would replace KyDNS's forwarding (and its DoT/DoH privacy) instead of checking what the upstream returns. No tagged releases. | Rejected |
| `github.com/peterzen/goresolver` | Last release 2019; does its own recursion; no NSEC3 opt-out handling | Rejected: unmaintained |
| unbound / ldns via cgo | The build must stay cgo-free (distroless image) | Rejected |
| A small validator in `internal/dnssec` on miekg/dns primitives | `RRSIG.Verify`, `RRSIG.ValidityPeriod`, `DNSKEY.ToDS`, `NSEC3.Cover/Match` are all there | **Chosen** |

miekg/dns v1.1.73 verifies algorithms 8, 10, 13, 14 and 15. It does not verify 16
(Ed448). For 16, `internal/dnssec` rebuilds the RFC 4034 §6 signed data (canonical
names and RRset order, no hashing) and calls `github.com/cloudflare/circl/sign/ed448`.
circl is already in the module graph (BSD-3-Clause, maintained by Cloudflare); this
makes it a direct dependency, covered by the existing govulncheck and OSV gates. A
test proves the rebuilt signed data is byte-identical to what miekg/dns signs, using
an Ed25519 RRset, so the only new code is canonical ordering, not crypto.

## How it works

Validation sits inside `dnsserver.Forwarder`, behind the cache. With the setting on:

1. Every upstream query asks for DO=1 and sets CD=1. KyDNS is the validator, so the
   upstream returns the data and its signatures even if it would call them bogus.
2. The answer is checked by `dnssec.Validator.Check` before it is cached. The cached
   copy carries the verdict in its AD bit; a bogus answer becomes a SERVFAIL with an
   RFC 8914 Extended DNS Error and is not cached (unchanged rule).
3. The chain: root DNSKEY against the embedded anchors, then for each label from the
   top down, a DS query for that name. A signed DS set leads to the child's DNSKEY set,
   which must contain a key matching a DS and be signed by it. A signed denial of the DS
   at a delegation (NSEC/NSEC3 with NS and without SOA, or an NSEC3 opt-out span) makes
   everything below insecure. Results per name are cached; DNSKEY and DS fetches use
   singleflight, so concurrent queries do not multiply upstream traffic.
4. Positive answers: each RRset on the CNAME chain must be signed by the zone the walk
   found for its owner. A wildcard expansion (RRSIG labels below the owner's) also needs
   the NSEC/NSEC3 proof that the query name does not exist. DNAME-synthesised CNAMEs are
   accepted only when they match the signed DNAME.
5. Negative answers: NXDOMAIN needs proof that the name and the wildcard at its closest
   encloser do not exist; NODATA needs a matching NSEC/NSEC3 without the type (or the
   wildcard NODATA form). NSEC3 opt-out on a DS query is insecure, per RFC 5155.
   NXDOMAIN applies to the end of the alias chain (RFC 6604): data at that end under
   NXDOMAIN is bogus, and a chain ending without data needs the denial proof.
6. The answer leaves with only what was validated: on a secure positive answer the
   authority section keeps only the NSEC/NSEC3 proof RRsets that verified (and their
   RRSIGs); on a negative one, the verified SOA and proof. Every TTL in a secure answer
   is capped by the Original TTLs and the time left on the signatures (RFC 4035
   §5.3.3), before it is cached or sent. The additional section is passed on, as
   resolvers do; AD does not cover it.
7. Types that are not one RRset: ANY cannot be validated, so while validating the
   Forwarder never forwards it and answers with the RFC 8482 §4.2 HINFO ("RFC8482",
   AD=0, nothing cached); `Check` calls ANY bogus (EDE 21) as a backstop. An RRSIG
   query is not validated either: it keeps only RRSIGs owned by the query name, AD=0.
   Other meta types (AXFR, IXFR, MAILA, MAILB) go through the normal path, where
   records not of the asked type need a denial proof and are trimmed.

Verdicts:

| Verdict | Client sees |
|---|---|
| Secure | The answer; AD=1 only if the client sent DO=1 or AD=1 (RFC 6840 §5.7) |
| Insecure (unsigned zone, unsupported algorithm or digest, NTA, NSEC3 iterations > 150) | The answer, AD=0 |
| Bogus | SERVFAIL with EDE 6 (DNSSEC Bogus), 7 (Signature Expired), 8 (Signature Not Yet Valid), 9 (DNSKEY Missing), 10 (RRSIGs Missing) or 12 (NSEC Missing) |

A client that did not send DO gets no RRSIG/NSEC/NSEC3 records (RFC 4035 §3.2.1),
unless it asked for that type.

Supported algorithms: 8, 10, 13, 14, 15, 16. Anything else, including 5 and 7
(RSA/SHA-1), counts as insecure, as RFC 6840 §5.2 requires, not as bogus. DS digest
types 2 and 4; type 1 only when no stronger digest is present for that key (RFC 4509 §3).

### Trust anchors

`internal/dnssec/anchors.go` embeds the DS records of KSK-2017 (tag 20326) and
KSK-2024 (tag 38696). The source is IANA's
<https://data.iana.org/root-anchors/root-anchors.xml>; a copy is committed as
`internal/dnssec/testdata/root-anchors.xml` (SHA-256 `3ccaab38…5278b908`, fetched
2026-11-15) and a test checks the embedded records against it. The expired KSK-2010
entry is not embedded. RFC 5011 automated rollover is not built: a new root KSK reaches
KyDNS in a release, and the daily dependency releases make that fast.

### What is never validated

- Local directory names, manual records and per-network views are answered by
  `Authoritative` before forwarding, exactly as today, and never carry AD. The forwarded
  tail of a local CNAME chain is validated (a bogus tail is dropped), but the reply stays
  AD=0 because its head is local.
- Negative trust anchors, applied automatically: the private domain, every configured
  reverse zone, `home.arpa`, `internal`, `local`, `lan`, `localdomain`, and the RFC 6303
  private reverse zones (`10.in-addr.arpa`, `16.172` … `31.172.in-addr.arpa`,
  `168.192.in-addr.arpa`, `254.169.in-addr.arpa`, `d.f.ip6.arpa`, `8.e.f.ip6.arpa` …
  `b.e.f.ip6.arpa`). KyDNS has no conditional forwarding today, but a router used as a
  `udp://` upstream commonly answers these names itself, unsigned, while the root proves
  they do not exist. Under an NTA the answer is insecure, never bogus, never AD.

### Clock safety

A Raspberry Pi has no RTC; it boots in the past until NTP syncs, and NTP needs DNS. With
a wrong clock every signature looks expired and nothing resolves, so NTP never recovers.

KyDNS skips the signature time checks (inception/expiration) only, like dnsmasq's
`dnssec-no-timecheck`, while either holds:

- the system clock is earlier than the build floor: the commit time the Go toolchain
  embeds (`vcs.time`), or a constant in `internal/dnssec/clock.go` when the binary was
  built without VCS data, whichever is later; or
- on Linux, `adjtimex(2)` reports the kernel clock as unsynchronised (`TIME_ERROR`).
  That is read-only and needs no capability. Where the call fails or is unavailable,
  KyDNS cannot tell, and only the build floor applies.

Everything else (the chain, the keys, the signatures themselves) is still checked.
Entering the state logs once (`dnssec time checks skipped`, with the reason) and the
dashboard shows a banner until the clock is safe again; leaving it logs once too. The
state is re-read at most every 10 seconds.

The cost: while skipped, an attacker who can serve an old, expired, correctly signed
answer can replay it. That is the same trade dnsmasq and systemd-resolved make on
clockless devices, and it is visible on the dashboard. A host that runs no NTP client at
all reads as unsynchronised forever; the banner tells the operator so.

## Data stored

Two columns on the `settings` row:

| Column | Type | Meaning |
|---|---|---|
| `dnssec_validate` | INTEGER, default 0 | The "Validate DNSSEC locally" setting |
| `dnssec_notice` | INTEGER | 1 while the one-time "turn it on" notice is pending |

- New install: the first-run seed (`dns.dnssec_validate`, default `true`) writes 1;
  the notice column starts at 0.
- Upgrade: the migration adds `dnssec_validate` as 0 (behaviour unchanged) and
  `dnssec_notice` as 1, so the notice shows once.
- Both are node-local, like `log_queries`: no `cv_` trigger names them, `ApplySnapshot`
  keeps the replica's own value, and a replica may change them (`PutDHCPSettings`, the
  replica write path, now also writes `dnssec_validate`). Reason: validation depends on
  this node's clock and upstreams, and changing the replicated set needs a trigger
  migration this store cannot run safely (the trigger is created by the schema, before
  migrations).
- Nothing about queries is stored. The validator's caches are in memory, bounded
  (4,096 names; full means cleared), and hold only public DNS data.

## Routes and auth

| Route | Auth | Purpose |
|---|---|---|
| `POST /settings/server` (existing) | session + CSRF | Gains the "Validate DNSSEC locally" checkbox |
| `PATCH /api/v1/settings` (existing) | admin token | Gains `dnssec_validate` |
| `POST /settings/dnssec-notice` (new) | session + CSRF | `action=enable` turns validation on and clears the notice; `action=dismiss` only clears it. Exempt from the replica write gate, since both columns are node-local |

No route is anonymous, no route takes a URL or makes an outbound request from user
input, and nothing new is logged beyond the two clock-state lines. Query names are never
logged by the validator; a bogus verdict is logged only through the existing query log,
which is off by default.

## Defaults

- New installs: on. Upgrades: off, with the one-time notice.
- Validation applies live; turning it on or off flushes the answer cache, since cached
  AD bits mean different things in the two modes.
- Off keeps today's behaviour exactly (trust a secure upstream's AD).

## Not built

- RFC 5011 trust anchor rollover; anchors come with releases.
- Honouring a client's CD=1 by returning bogus data: KyDNS always validates, and a
  bogus answer is SERVFAIL for everyone. A downstream validator still gets the RRSIGs it
  needs for secure and insecure answers.
- Aggressive NSEC caching (RFC 8198), a bogus-answer cache (RFC 9520 asks for one;
  the key and DS failures are cached for 60 s, which is what stops the hammering).
- Operator-defined negative trust anchors and a per-name "explain" view.
- Validating the additional section.
- Algorithms 5 and 7 (RSA/SHA-1): insecure, matching current Fedora/RHEL policy.

## Owner questions

1. Should the automatic NTAs include more private-use TLDs (`home`, `corp`, `fritz.box`)?
   Default chosen: only `home.arpa`, `internal`, `local`, `lan`, `localdomain` and the
   private domain, since every NTA is a name an upstream can forge.
2. Should `dnssec_validate` replicate from the primary instead of being node-local?
   Default chosen: node-local (reason above).
