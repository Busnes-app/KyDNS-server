# Logging

KyDNS must emit structured, privacy-safe application logs to standard output
and standard error. It must not build or require a KySecurity-specific log
database, log search system, or long-term retention service.

Operators may route container logs to an existing platform such as Loki,
OpenSearch, Elasticsearch, Graylog, or another OpenTelemetry-compatible
collector.

Log service registration changes, record changes, blacklist policy and list
refresh changes, DHCP and discovery failures, health-check results, policy
changes, and administrative actions. When replication ships, peer enrollment,
replication failures, and replication conflicts join that list. Query logs must
be configurable, minimized by default, and retained by the operator's logging
platform rather than KyDNS.

Query logging is off by default (`log_queries`), and recording the client IP is
a separate opt-in (`log_client_ip`), so turning on query logging does not by
itself record who asked. Both are server settings, edited under Settings in the
web UI, with `kydns settings set`, or over the API, and each takes effect on the
next query — there is no restart, and turning one on never turns on the other.

Settings changes are administrative actions and are logged: `settings applied`
when a change takes effect, `upstreams changed, cache flushed` when the upstream
list moved, `seeded settings from the config file` on a first run, and a warning
naming any `allow_query` prefix that reaches beyond the operator's network, at
every start for as long as it is configured. None of these carry credentials.

Blacklist lifecycle events are `blacklist list refreshed`, `blacklist list
refresh failed` (with a reason, no response body), and `blacklist list
unchanged`, plus events for settings changes and rule additions/removals. When
query logging is enabled, each entry gains a `policy` field: `local` (the
authoritative lookup answered), `allow`, `deny`, the name of the list that
matched, or `forwarded` (no policy match).

Never log upstream credentials, private keys, full DNS answers when they reveal
sensitive private services, or raw request bodies. Do not send query history to
KySecurity services by default. A blacklist list URL may never embed
credentials; the field is rejected outright rather than accepted and redacted
later. Downloaded list content, list URLs, client IPs, and query names are
never logged by default. Blacklist counters (blocked-query totals and
per-list match counts) never carry client identity.

Do not add an embedded log database or product-specific log viewer. Operators
should use their existing logging platform for search, alerting, retention, and
access control.

The one exception is the in-memory query ring the MCP tools search
(`search_queries`, `query_stats`; see [docs/AI-AGENTS.md](docs/AI-AGENTS.md)). It
holds the most recent 10,000 answered queries on one node, never touches disk,
and follows the two opt-ins exactly: it fills only while `log_queries` is on,
records the client address only while `log_client_ip` is also on, and is
emptied when either is turned off and on restart. It holds nothing the stdout
query log does not already carry. Reading it takes an agent token with the
separate query-log grant, because it is browsing history. It is not a retention
store: history beyond the ring belongs in the operator's logging platform.

AI agent activity is recorded in the local audit table: `mcp.auth_failed` (a
real token refused, by token ID), `mcp.rule_add`, `mcp.rule_remove`,
`mcp.lists_refresh`, `mcp.blocking_pause`, `mcp.blocking_resume`,
`agent_token.create`, `agent_token.access_change` and `agent_token.expired`. No
token secret is recorded, nor any query-log content. A pause and a resume are
also logged as `blacklist blocking paused` (with its end) and `blacklist
blocking resumed`.

Each accepted SSO back-channel logout is audited as `auth.sso_backchannel_logout`
with the provider's address; the token, subject and `sid` are not recorded.

Backup audit events record bounded action, outcome, capsule ID, remote address, and a
printable error summary. The actions are `backup.paired`, `backup.key_pinned`,
`backup.unpaired`, `backup.schedule`, `backup.exported`, `backup.export_failed`,
`backup.drill`, and `admin.backup_run`, which is written by the library for every backup
run whatever started it and whose details column is a JSON object rather than a sentence.
They never record pairing codes, bearer tokens, sealed token ciphertext, capsule contents,
recovery shares, or recovery private keys.
