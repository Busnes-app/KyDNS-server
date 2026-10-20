# KyDNS

**Start here:** [Standalone installation](INSTALL.md). Manage private DNS records, service names, filtering and optional replication. A native install can also serve DHCP. KyIdentity and KyRecovery are optional. Verify DNS over both UDP and TCP before changing client resolver settings; the web health page does not test DNS resolution.

KyDNS is a self-hosted local DNS server and service directory for homes,
homelabs, and small teams.

Its primary job is to make private services easy to name and reach. It also
provides opt-out DNS blackhole filtering with built-in and operator-managed
lists.

KyDNS can pair with KyRecovery using a one-time six-digit code and back itself
up automatically; see [Backups](docs/BACKUPS.md#backups) below.

Container-local installer setup and exact KyIdentity administrator grants are documented in
[INSTALLER_SETUP.md](docs/INSTALLER_SETUP.md).

## Example

Register a service once:

```sh
kydns service add kypost \
  --address 192.168.1.20 \
  --alias webmail \
  --check https://kypost.home.arpa/health
```

Clients can then use:

```text
kypost.home.arpa
webmail.home.arpa
```

without manually maintaining hosts files or opaque DNS rewrite rules.

## License

KyDNS is licensed under the [MIT License](LICENSE.txt). The web UI carries the
notice and serves the full license text from its About popover, so an operator
can always see the terms of what they are running.

## What it can do

- Private service names, DNS records and per-network views.
- Blocklist filtering and encrypted upstream resolution.
- Service discovery and optional linked-node replication.
- Optional DHCPv4 on supported native or host-network installations.

## Documentation

- [Backups](docs/BACKUPS.md) — Backups. <a id="backups"></a>
- [Features](docs/FEATURES.md) — What works today, Not yet, Scope, Repository status. <a id="what-works-today"></a><a id="the-built-in-dhcp-server"></a><a id="blacklist-filtering"></a><a id="not-yet"></a><a id="scope"></a><a id="repository-status"></a>
- [Configuration](docs/CONFIGURATION.md) — Configuration. <a id="configuration"></a><a id="renaming-the-private-domain"></a><a id="opening-allow_query-beyond-your-lan"></a><a id="upgrading-from-a-config-file-deployment"></a><a id="upstream-encryption"></a><a id="tailscale"></a><a id="putting-a-service-behind-a-reverse-proxy"></a>
- [Replication](docs/REPLICATION.md) — Replication. <a id="replication"></a><a id="when-the-primary-is-down"></a>
- [Packages](docs/PACKAGES.md) — Install from a package. <a id="install-from-a-package"></a>
- [Raspberry pi](docs/RASPBERRY-PI.md) — Raspberry Pi SD image. <a id="raspberry-pi-sd-image"></a>
- [Running](docs/RUNNING.md) — Running it. <a id="running-it"></a><a id="forgot-the-admin-password"></a><a id="docker"></a><a id="unraid"></a><a id="pre-creating-it"></a><a id="four-things-that-bite"></a><a id="upgrading-from-an-image-that-ran-as-root"></a><a id="wifi"></a>
- [Security model](docs/SECURITY-MODEL.md) — Security model. <a id="security-model"></a>
