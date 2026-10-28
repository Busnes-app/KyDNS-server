# KyDNS

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/15924/badge)](https://www.bestpractices.dev/projects/15924)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Busnes-app/KyDNS-server/badge)](https://scorecard.dev/viewer/?uri=github.com/Busnes-app/KyDNS-server)
[![OctoCounts](https://api.octocounts.com/badge/Busnes-app/KyDNS-server/branch/main)](https://octocounts.com/github/Busnes-app/KyDNS-server/tree/main)

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
- An MCP endpoint so AI agents can explain blocks and manage rules.
- Optional DHCPv4 on supported native or host-network installations.

## Connect an AI agent

AI agents such as OpenClaw and Hermes Agent reach KyDNS over MCP at
`https://<your host>/mcp`, on the admin listener and never on the DNS port. Under
**Settings → API tokens**, create a token with the access **AI agent** and an access level:

- **Read only** (the default): the agent can explain why a name is blocked ("which list or
  rule matched"), read the totals and see the block lists with when each last updated.
- **Read and write**: it can also add or remove allow and deny rules, refresh the lists, and
  pause blocking for up to 60 minutes. Blocking resumes on its own.
- **Can read the query log** is a separate box, off by default, at either level. The query log
  is your network's browsing history: who looked up what, and when. It also covers the
  most-blocked names. Tick it only for an agent you would show that to.

The page shows the token once, with config for both agents that reads it from
`KYDNS_AGENT_TOKEN`, and the command to install the KyDNS skill (served at
`/agent/skills/kydns/SKILL.md`), which teaches the agent the tools and when to ask you first.
You can change an agent's access there later. An agent token unused for 90 days is revoked; an
agent token never works on the admin API, nor an admin token over MCP. No agent can change
upstream resolvers or listener settings. The admin UI, API and `/mcp` refuse browser
cross-origin requests and answer only to `localhost`, IP addresses and the names in
`KYDNS_ADMIN_HOSTNAMES`; see [Running](docs/RUNNING.md).

Domain names in the query log and in block lists are chosen by devices, websites and list
maintainers, and an agent may treat them as instructions. KyDNS labels them as untrusted data,
but give an agent write access only if you trust everyone who can message it. Details:
[AI agents](docs/AI-AGENTS.md).

## Documentation

- [Backups](docs/BACKUPS.md) — Backups. <a id="backups"></a>
- [Features](docs/FEATURES.md) — What works today, Not yet, Scope, Repository status. <a id="what-works-today"></a><a id="the-built-in-dhcp-server"></a><a id="blacklist-filtering"></a><a id="not-yet"></a><a id="scope"></a><a id="repository-status"></a>
- [Configuration](docs/CONFIGURATION.md) — Configuration. <a id="configuration"></a><a id="renaming-the-private-domain"></a><a id="opening-allow_query-beyond-your-lan"></a><a id="upgrading-from-a-config-file-deployment"></a><a id="upstream-encryption"></a><a id="tailscale"></a><a id="putting-a-service-behind-a-reverse-proxy"></a>
- [Replication](docs/REPLICATION.md) — Replication. <a id="replication"></a><a id="when-the-primary-is-down"></a>
- [Packages](docs/PACKAGES.md) — Install from a package. <a id="install-from-a-package"></a>
- [Raspberry pi](docs/RASPBERRY-PI.md) — Raspberry Pi SD image. <a id="raspberry-pi-sd-image"></a>
- [Running](docs/RUNNING.md) — Running it. <a id="running-it"></a><a id="forgot-the-admin-password"></a><a id="docker"></a><a id="unraid"></a><a id="pre-creating-it"></a><a id="four-things-that-bite"></a><a id="upgrading-from-an-image-that-ran-as-root"></a><a id="wifi"></a>
- [AI agents](docs/AI-AGENTS.md) — MCP tools, access levels, the query log, pausing. <a id="ai-agents"></a>
- [Strong-key configuration](docs/STRONG-CRYPTO.md) — Supported TLS/key policy and existing-installation migration.
- [Security model](docs/SECURITY-MODEL.md) — Security model. <a id="security-model"></a>
- [Verifying releases](docs/VERIFYING-RELEASES.md) — Check a release file or image's signature and source commit, and rebuild the binary to the same bytes.
- [Assurance case](docs/ASSURANCE-CASE.md) — Threat model, trust boundaries, and why the security requirements are met.
