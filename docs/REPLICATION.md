# Replication

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Replication

A second KyDNS keeps resolving while the first one is down. One node is the
primary and takes every change; the other follows it and serves the same
answers read-only. Give your clients both addresses.

Both `replication` keys are owned by the config file, so each step here that
sets one needs a restart. On the primary:

```yaml
replication:
  listen: "192.168.1.10:8443"   # its own TLS listener, separate from DNS and the UI
```

On the node that will follow it:

```yaml
replication:
  primary: "192.168.1.10:8443"
```

Restart both, then pair them. Pairing works like an SSH host key: the primary
prints a code and its fingerprint, and the other node reports the fingerprint
of whoever answered before the code is ever sent.

```sh
kydns replica invite                  # on the primary: a code and a fingerprint
kydns replica join 192.168.1.10:8443 <code> --fingerprint <fingerprint>
kydns replica list                    # on the primary: lag and last sync per replica
kydns replica status                  # on a replica: what it follows, and why a poll failed
```

Compare the two fingerprints yourself. Without `--fingerprint`, `join` shows
you the one it was presented and waits for a yes; if it does not match, say no.
A mismatch means something is answering in the primary's place, and the code is
not sent, so it stays good.

Restart the replica once more after joining — the pull loop is built at
startup. From then on it polls every five seconds, and a change on the primary
is answered by the replica a few seconds later. Writes on the replica are
refused with the address of the primary to make them on: in the API, in the
CLI, and in the UI, whose editing controls are disabled with the same reason.
Its own DHCP settings are the exception, because they are node-local and no
pull can discard them; see the DHCP section above.

The Replication screen in the web UI does all of this too: minting an invite,
pairing a replica with the primary its config names, listing replicas with
their lag, removing one, and promoting. The screen pairs with
`replication.primary` and never an address you type, because that is the
address the pull loop dials.

### When the primary is down

The replica keeps answering every name it last pulled, for as long as it takes.
It shows how long since its last sync, and puts up a standing banner once that
passes a minute. Service health reads as unknown, not as the last thing it
heard: the node that runs the checks is the one that is missing.

Promote it once you have decided the primary is not coming back:

```sh
kydns replica promote
```

It accepts writes immediately. Two things are then yours to do:

- Set `replication.listen` on it and restart, or nothing can follow it.
  Promotion opens no listener and does not edit your config file.
- Do not switch the old primary back on until it has been demoted or rebuilt.
  Demote it with `kydns replica join` against the new primary, which discards
  its configuration on the first pull. Two primaries serving the same replicas
  is the one state KyDNS can neither detect nor undo.

Demoting an old primary means editing both keys in its config: set
`replication.primary` to the new primary, and remove the `replication.listen`
it still has from when it was one. A node with both keys refuses to start, so
setting only the first leaves that box with no DNS at all.

The promotion is recorded in the database, so a promoted node comes back a
primary at every later start even though `replication.primary` is still in its
file. It logs that the key is being ignored; remove it when convenient.

There is no automatic failover, no leader election, and no peer discovery.
Promotion is a decision you make, and one primary is the only supported shape.
