# AI agents

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

KyDNS serves the Model Context Protocol (Streamable HTTP, stateless) at `/mcp` on the admin
listener, behind the same transport gate as the web UI: loopback, or an HTTPS reverse proxy in
`KYDNS_TRUSTED_PROXIES`. It is never served on the DNS listener. Design:
[the MCP spec](superpowers/specs/2026-10-20-kydns-mcp-agent-access-design.md).

## Browser origins

- `/mcp` refuses any request carrying an `Origin` header (403) and sends no CORS headers.
- Nothing on the admin listener sends `Access-Control-Allow-Origin`.
- Creating, changing or revoking a token in the UI needs the session and its CSRF token, and a
  cross-origin request is refused even with both. The API takes an admin bearer token, which
  browsers never attach on their own, and refuses cross-origin requests too.
- Requests must use a host name in `KYDNS_ADMIN_HOSTNAMES`, `localhost` or an IP address.
- The page that shows a new token names the agent and the access it grants, and an agent token
  must have a name.

## Tokens

An agent token is a row in the same table as admin API tokens, with an access level stored in
the row, never in the token. Create one under **Settings → API tokens**, with
`kydns token add <label> --agent-read|--agent-write [--query-log]`, or with
`POST /api/v1/tokens {"label", "scope": "mcp:read"|"mcp:write", "query_log": bool}`.
`PATCH /api/v1/tokens/{id}` changes an agent token between the two agent levels and its
query-log flag. An admin token can never become an agent token, or the reverse.

| Access | Tools |
|---|---|
| `mcp:read` | `explain_domain`, `get_stats`, `list_sources` |
| `mcp:write` | the above, plus `add_rule`, `remove_rule`, `refresh_lists`, `pause_blocking`, `resume_blocking` |
| query log flag, at either level | `search_queries`, `query_stats` |

Every tool is listed for every token; a tool the token may not use answers `read-only token`
or `this token cannot read the query log`, so the agent can tell the user what to change.

- `/mcp` accepts only `Authorization: Bearer`. A request with an `Origin` header is refused
  (403), so a browser page cannot use a token, and cookies never authenticate.
- An admin token at `/mcp`, an agent token at `/api/v1`, a wrong secret, an idle token and a
  revoked KyIdentity administrator's token all get the same 401.
- A token unused for 90 days is refused and deleted the next time it is presented, and a daily
  sweep removes the ones nobody presents. Both are audited as `agent_token.expired`.
- `kydns admin reset-password` and installer directory revocation delete agent tokens with admin tokens.
- Tokens are node-local, like admin tokens: point an agent at one node.

## The query log

`search_queries` and `query_stats` read an in-memory ring of the most recent 10,000 answered
queries on this node. It fills only while **Log queries** is on, records the client address
only while **Log client IP** is also on, and is emptied when either is turned off and on
restart. It is never written to disk. The opt-ins are checked under the ring's own lock, so a
query answered while one is being switched off cannot be stored after the clear. Results say whether each opt-in is on and how far back
the ring reaches, so an agent can explain an empty answer.

The query log is browsing history. That is why it is a separate grant: a read-only agent that
can explain blocks does not see who asked for what.

## Untrusted names

Domain names in query results, stats and block lists come from devices, websites and list
maintainers. A name such as `ignore-previous-instructions.example` is legal. Every result that
carries names has a `note` saying they are untrusted data, every such tool's description says
so, and the server's MCP instructions and the skill tell the agent never to act on them.

## Writes

- Rules go through the same validation as the web UI: a domain has one rule, allow or deny. An
  agent cannot make or remove a rule for a public suffix (`co.uk`, `github.io`): an allow there
  would end blocking for every site under it with no time limit, past the bounded pause, and a
  deny would break them. Make such rules in the web UI.
- `refresh_lists` downloads one list by name, or every enabled list, at most once per 5 minutes
  across all agents on a node. A failed download keeps the previous copy and is reported in
  that list's `last_error`.
- `pause_blocking` turns filtering off for 1 to 60 minutes. The deadline is held in memory and
  compared on every query, so blocking resumes on its own with nothing to run it; a restart
  also resumes it. The Blacklists page shows the pause with **Resume now**.
- On a replica, rules and refreshes are refused (the primary owns them). A pause is node-local
  and works on any node, but pausing the primary does not pause its replicas.
- Upstream resolvers, listeners and every other setting are out of reach.

Writes are audited as `mcp.rule_add`, `mcp.rule_remove`, `mcp.lists_refresh`,
`mcp.blocking_pause` and `mcp.blocking_resume`, with the token ID. Token creation and access
changes are `agent_token.create` and `agent_token.access_change`; failed authentication of a
real token is `mcp.auth_failed`. Unknown tokens are not audited, so strangers cannot grow the
audit table, and no secret is ever recorded.

## Agent setup

```json
"mcp": { "servers": { "kydns": {
  "url": "https://<host>/mcp",
  "transport": "streamable-http",
  "headers": { "Authorization": "Bearer ${KYDNS_AGENT_TOKEN}" }
}}}
```

```yaml
mcp_servers:
  kydns:
    url: "https://<host>/mcp"
    headers:
      Authorization: "Bearer ${KYDNS_AGENT_TOKEN}"
```

```sh
# Hermes Agent
hermes skills install https://<host>/agent/skills/kydns/SKILL.md --category homelab

# OpenClaw
mkdir -p ~/.agents/skills/kydns
curl -fsSo ~/.agents/skills/kydns/SKILL.md https://<host>/agent/skills/kydns/SKILL.md
```
