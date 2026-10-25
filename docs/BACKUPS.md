# Backups

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Backups

Backups are sealed to the suite recovery key and this server never holds what opens them.
The key arrives by pairing with KyRecovery, or is pasted from the ceremony page for a
server with no KyRecovery. **Settings → Disaster recovery** does both, and shows the
pinned key, the destinations, the schedule and the newest local copy. Every capsule is
sealed to that key and goes to each configured destination: KyRecovery when paired, and
`KYDNS_BACKUP_DIR` when set, as `KyDNS.<capsule-id>.kycap` at mode 0600. The newest
`KYDNS_BACKUP_KEEP` (default 7) with that prefix are kept; other files in the directory
are never touched. A local copy that fails never cancels the deposit, and a deposit that
fails never deletes the local copy.

A capsule carries `data/kydns.db`, `data/backup_key` and a record of the config, plus
`data/node_key` and `data/recovery.pub` when this node has them.

`KYDNS_BACKUP_DEPOSIT_INTERVAL` (default `24h`, 15 minute floor, `0` disables) seeds the
schedule on first start; after that the schedule is an admin setting you change on the
screen or with `kydns backup-schedule --minutes n`. Custodian cards come from the
KyRecovery ceremony; a restore needs k of them typed on stdin, see
[docs/RESTORE.md](RESTORE.md).

**KyRecovery must be reached over HTTPS, and by default at a public address.** TLS is not
for the capsule, which is already sealed. It protects the public key that arrives at
pairing (trust on first use), the deposit token, and the receipts. For a KyRecovery on your
own network behind a TLS proxy, set `KYDNS_BACKUP_ALLOW_PRIVATE_RECOVERY=true`;
HTTPS is still required and loopback stays refused. Whatever the wire, compare the key
fingerprint Settings shows with the one in the KyRecovery dashboard; a swapped key then fails at
pairing instead of at restore. In Docker, names that exist only on your LAN need
`docker-compose.lan-dns.yml` with `KYDNS_DNS` on the command line.

Without that, pairing fails with `lookup recovery.<lan domain> on 127.0.0.11:53: no such
host`: Docker's resolver inside the container asks the host's resolvers, and records you keep
in KyDNS itself do not help, because the host usually cannot reach a container on its own
`br0`/ipvlan address. Unraid has no Compose file. Add the equivalent under the container's
**Edit → Advanced View → Extra Parameters**, then Apply:

- `--dns=<LAN DNS server>`, the same as the overlay (never KyDNS's own address), or
- `--add-host=recovery.<lan domain>:<proxy address>` for KyRecovery's name alone.

**One recovery key for life.** The first pairing pins KyRecovery's public key. Pairing with
a KyRecovery that holds a different key fails with `already pinned to a different recovery
key`, and unpairing does not clear the pin: it is what stops a hijacked admin session from
redirecting backups. A node paired with a test or older KyRecovery can only pair with that
one again. The claim succeeds on KyRecovery's side before KyDNS refuses the key, so the new
KyRecovery lists KyDNS as paired: revoke that entry there. A supported move to a new key is
requested in Busnes-app/ky-primitives#40.
