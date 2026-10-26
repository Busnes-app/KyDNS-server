# Supported strong-key configuration

[Overview](../README.md) · [Installation](../INSTALL.md) · [Security model](SECURITY-MODEL.md) · [Assurance case](ASSURANCE-CASE.md)

This supported configuration keeps DNS, pinned replication, local/SSO
administration and sealed recovery available while refusing undersized TLS
and OIDC signing keys. It is a process configuration, not FIPS certification
or evidence that any deployed instance has been checked.

## Preserve the installation

Keep the existing database, `node_key`, `backup_key`, recovery public-key pin,
sealed capsules, installer credential files and configuration. Do not generate
replacement keys to enable the profile. `node_key` is an exact 32-byte Ed25519
seed; `backup_key` contains an exact 32-byte AES key encoded as hex. Changing
either breaks its existing identity or encrypted credential bindings. Normal
startup retains the files and their enforced 0600 permissions.

For managed SSO, keep an independent random directory-signing secret with at
least 16 bytes; 32 random bytes encoded as hex is recommended. The file's string
bytes are the HMAC key, not hex-decoded bytes. Installer setup already refuses
short directory secrets. Coordinate any necessary rotation with the sender and
protect the original credential paths; do not print or commit their contents.
OAuth client secrets and bearer tokens are separate authentication credentials,
not raw HMAC keys.

Every RSA member in a newly fetched IdP JWKS must have at least 2048 modulus
bits. A weak member also refuses a mixed strong/weak set. Replace/retire weak
keys at the provider; do not pad keys or disable signature verification. This
signing-key rule applies even outside the optional TLS profile.

## Enable the process profile

Build with the toolchain specified by `go.mod` (the Dockerfile pins its builder).
Set `GODEBUG=fips140=on` before starting **every KyDNS process**, including its
remote CLI. If other GODEBUG settings are needed, keep them and set the `fips140`
entry to `on`. Use `on`, not diagnostic `only`: the former keeps Argon2id and the
suite's fixed X-Wing/AES-256-GCM recovery format available.

```sh
export GODEBUG=fips140=on
./bin/kydns serve --config /path/to/existing/kydns.yaml
```

For systemd, add `Environment=GODEBUG=fips140=on` to a service drop-in, preserving
any other required settings, then reload the unit and restart the service.
For Docker Compose, append `docker-compose.strong-crypto.yml` to the installation's
existing Compose file list. For example, if the LAN-DNS overlay is already used:

```sh
docker compose -f docker-compose.yml -f docker-compose.lan-dns.yml \
  -f docker-compose.strong-crypto.yml config --quiet
docker compose -f docker-compose.yml -f docker-compose.lan-dns.yml \
  -f docker-compose.strong-crypto.yml up -d --build
```

Preserve the same project, image/build selection, network, data/backup mounts,
DNS and other operator overlays. The new overlay adds only the process setting.
If using a saved COMPOSE_FILE chain, append the overlay there and keep using that
chain for all commands. Do not overwrite .env or an existing GODEBUG value
without retaining its needed settings. No data deletion, unpairing or re-enrollment
is required by enabling this profile.

## Enabled boundaries

| Boundary | Configuration and key policy |
| --- | --- |
| DoT/DoH upstreams | Use verified `tls://` or `https://` upstreams; do not enable plain UDP fallback. The process policy requires RSA-2048+ or approved P-256/P-384/P-521/fixed Ed25519 certificate keys across the verified chain, including trusted roots. Custom roots do not waive the floor. Certificate/hostname checks remain enabled. DoH refuses redirects. |
| Blacklist HTTPS | Existing fetches require verified HTTPS and refuse redirects away from HTTPS. The same process TLS key policy applies. Keep list integrity and source selection under administrator control. |
| Service health | For HTTPS checks, leave certificate verification enabled (`check_insecure=false`) and use a trusted strong certificate chain. Install a private CA in the process trust store where needed. The opt-out deliberately skips CA/hostname verification and is outside this profile. Plain HTTP/TCP probes test availability and provide no authenticated TLS claim. |
| OIDC | Use direct HTTPS issuer, authorization, token, JWKS and optional userinfo URLs without embedded URL credentials. Configured-mode discovery/exchange refuses plaintext endpoints and redirects; failed discovery refuses login instead of silently choosing fallback URLs. The private JWKS input boundary always refuses RSA moduli below 2048 bits, malformed/oversized bodies and redirects. Upstream signature, issuer, audience, expiry and nonce checks remain. |
| Linked nodes | Keep the operator-compared Ed25519 fingerprints and original node seeds. TLS 1.3 requires the pinned Ed25519 peer; its explicit pin replaces CA validation. It cannot accept an arbitrary RSA certificate. Pairing peeks first, then sends the one-time code only over the confirmed pinned connection. |
| KyRecovery | Existing fixed X-Wing HPKE/AES-256-GCM capsule format and 32-byte local sealing key remain. Recoveryclient requires HTTPS and refuses redirects, with private/LAN access only through the documented explicit opt-in; the process TLS key policy still applies. Preserve the write-once pin and receipt checks. |
| Admin UI/API/MCP and remote CLI | KyDNS serves internal HTTP; put a TLS proxy with strong certificates in front and restrict direct access to the admin port to that proxy. The remote CLI requires HTTPS outside loopback and must run with the profile setting too. Trust only actual proxy peers and configure the admin hostnames. |

At the browser-facing proxy, use TLS 1.2/1.3 and a verified RSA-2048+ or P-256+
certificate chain. For an Nginx/OpenSSL edge the relevant settings are:

```nginx
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers 'DEFAULT@SECLEVEL=2';
```

Configure certificate paths/routing and verify the actual edge before rollout;
Go's setting does not configure the proxy or a browser. Ordinary LAN DNS/DHCP
remain their documented protocols; this profile does not add encrypted DNS
service for LAN clients or authenticate DHCP packets.

## Verify before rollout

Replace incompatible endpoint certificates and IdP signing keys first. Verify
local administration, a real SSO sign-in, DoT/DoH resolution, list refreshes,
health checks, linked-node replication and a scratch restore with the retained
keys. Do not respond to a TLS refusal by disabling validation or replacing local
keys. An unavailable upstream or provider remains unavailable; credentials and
DNS configuration are retained for repair and retry.

Source checks cover real RSA-1024/2047 signed ID-token refusal and RSA-2048
acceptance; unsafe metadata/redirect refusal; and actual DoT/DoH requests to a
separate ordinary-mode TLS peer. They demonstrate the configured client refusing
weak leaves and trusted roots while the same endpoints work in ordinary mode.
Full ordinary and configured race suites remain required. Run
`python3 scripts/check-strong-crypto-compose.py` to verify both Compose chains
using synthetic configuration without starting containers. Docker CI also
reuses an existing data volume when enabling the profile. These are
local fixtures, not proof of a live deployment, private CA installation or
browser/proxy policy.

Replication TLS has forward secrecy. Durable custodian recovery intentionally
allows later decryption of archived capsules and does not have recipient-key
forward secrecy. Do not replace that recovery contract with an ephemeral key.

Primary reference: [Go FIPS runtime mode and platform limits](https://go.dev/doc/security/fips140).
