# Container-local installer setup

`kydns apply-setup --capabilities` advertises `ky.product-setup/1` without opening
configuration or state. `apply-setup --file /absolute/bundle.json [--check]` talks
to the running product through its owned mode-0600 UNIX socket at
`<data_dir>/installer/setup.sock`; the parent directory must be owned mode 0700.
`KYDNS_CONFIG` selects the existing YAML file (default `/etc/kydns/kydns.yaml`).
The runtime activates SSO live; no installer restart is required.

The protected bundle uses the shared `ky-primitives/applysetup` contract:

```json
{
  "schema": "ky.product-setup/1",
  "service": "kydns",
  "setupId": "271333e2-c221-4601-b249-563b3d828530",
  "sso": {
    "issuerUrl": "https://identity.example.com",
    "clientId": "dns",
    "clientSecretFile": "/var/lib/kydns/installer/client.secret",
    "directorySecretFile": "/var/lib/kydns/installer/directory.secret",
    "redirectUri": "https://dns.example.com/auth/sso/callback"
  },
  "admins": [
    {"issuer": "https://identity.example.com", "subject": "owner-subject", "username": "owner-admin"}
  ]
}
```

Bundle and credential files must be private, bounded, regular, owned by the
runtime user and referenced by absolute paths. Credentials never appear in the
report, installer configuration row or settings page; only the OIDC exchange reads
the client secret. The issuer is canonical HTTPS without a
trailing slash. The directory secret requires at least 16 bytes.

KyDNS has administrator accounts only. The bundle refuses `everyday` identities;
do not assign the everyday identity to this OIDC client. Exact verified
issuer/subject grants authorize management independently of a global provider
role. Existing local recovery passwords, DNS data and legacy tokens are preserved.
An instance with no local account receives an empty, disabled password slot;
no password is generated or reported. Explicit container-local `admin reset-password`
remains the operator's recovery mechanism.

Setup refuses existing foreign SSO settings and linked local recovery accounts.
Matching reruns are read-only and retain administrator passwords and audit history.
`--check` inspects the active product without migrations, key generation, new audit
rows, provider calls or authority reactivation. It refuses missing setup, changed
ownership and revoked administrators.

`POST /api/v1/installer/directory` authenticates `syncauth` signed SCIM desired
state. Only assigned subjects and the configured issuer are accepted. User-created
and updated events require an active flag, `kydns.admin` role and canonical positive
`meta.version` such as `W/"3"`. Deactivation and deletion remove authority. A durable
revision/digest fence refuses stale or conflicting events and accepts identical
retries without new audit rows. Directory authority is node-local on replicas too.
Sessions check current authority on every request and record the directory revision
at login; a session from before the latest revocation stays invalid after reactivation. Tokens created by a managed
administrator are issuer/subject-bound, including descendants issued through the
JSON API; offboarding deletes them atomically, and token creation checks that
authority in the same transaction as the insert. Later reactivation does not restore
old tokens. Legacy local recovery tokens retain their independent authority.

`GET /api/v1/installer/network` returns only `clientIp`, `proxyHeadersTrusted` and
`scheme`, with `Cache-Control: no-store`. `KYDNS_TRUSTED_PROXIES` accepts explicit
IP addresses or CIDRs and refuses `/0`. Browser cookies and OIDC callback scheme
use the same peer trust. Untrusted forwarded headers cannot alter identity or scheme.

Managed credentials are sealed in backups with a reference-to-destination map;
see [RESTORE.md](RESTORE.md). Managed administrators cannot enroll the first
recovery key by pinning or pairing; the local recovery administrator (`kydns admin
reset-password` in the container) or an unbound recovery token does, after which
managed administrators back up to that pin. Installer-driven backup enrollment is not yet supported
and a nonempty `backup` bundle is refused before writes. Existing native backup
configuration and operations remain available. Central client assignment/role and
directory delivery must still be composed and verified by KyQuickStart; the presence
of this interface alone is not a completed suite installation.

Verification: `go test -race ./internal/store ./internal/registry ./internal/adminapi
./internal/web ./internal/auth ./internal/config ./internal/app ./internal/backup
./cmd/kydns`; the SSH fixture in KyQuickStart uses `KYQ_TEST_DNS_BINARY`.
The shared setup package comes from ky-primitives v0.10.0.
