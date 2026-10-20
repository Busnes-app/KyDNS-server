# KyDNS MCP access for AI agents — design

Status: implemented on `feat/mcp-endpoint` (2026-10-20). Follows the KyCalendar pattern
(`KyCalendar/docs/superpowers/specs/2026-10-14-kycalendar-mcp-agent-access-design.md`) so the
Ky suite exposes MCP the same way.

## Goal

An operator connects an AI agent (OpenClaw, Hermes Agent) to KyDNS so it can answer "why is
this blocked?" and manage block rules, with a token the operator creates, scopes and can revoke.

## Decisions

| Question | Decision |
|---|---|
| Transport | MCP Streamable HTTP at `/mcp`, stateless, JSON responses, official Go SDK `github.com/modelcontextprotocol/go-sdk` (as KyCalendar) |
| Where | The admin listener, behind `TransportGate`. Never the DNS listener |
| Credential | A row in `tokens` with new `scope` (`admin` default, `mcp:read`, `mcp:write`) and `query_log` columns. Same plaintext format; the access lives in the row |
| Separation | `AuthenticateTokenPrincipal` skips non-admin rows; `AuthenticateAgent` refuses admin rows. `PATCH` moves only between the two MCP scopes |
| Query log | Its own grant (`query_log`), at either level, off by default. Browsing history is not bundled into read |
| Lifetime | No expiry; unused for 90 days (from `last_used_at`, or `created_at`) is revoked at lookup and by a daily sweep |
| Revocation | Reusing `tokens` means `ResetAdmin` and installer directory revocation cover agent tokens with no new code; identity-bound agent tokens re-check their KyIdentity authority per request |
| Untrusted text | Results are structured JSON; every result carrying names has `note`; tool descriptions, `instructions` and the skill say names are data |
| Skill | `internal/agentskill/kydns/SKILL.md`, embedded, served publicly at `/agent/skills/kydns/SKILL.md` |

Out of scope for v1: upstream resolvers, listeners and other settings; list add/delete or
enable/disable; OAuth; MCP resources and prompts; stdio.

## Tools

| Tool | Needs | Reuses |
|---|---|---|
| `explain_domain` | read | `policy.Service.Explain`: the decision plus every rule and list entry that matches (`Set.MatchedBy`) |
| `get_stats` | read | `/api/v1/stats` counters; top blocked names carry no client |
| `list_sources` | read | list metadata (`last_ok_at`, `last_error`) and rules |
| `search_queries` | query log | `dnsserver.QueryLog.Each`: client (exact IP), domain (suffix at label boundary), window, `blocked_only`, `limit` ≤ 500 |
| `query_stats` | query log | per-client counts and top allowed/blocked names over the ring |
| `add_rule`, `remove_rule` | write | `policy.Service.AddRule` / `DeleteRule` |
| `refresh_lists` | write | `policy.Service.Refresh` |
| `pause_blocking`, `resume_blocking` | write | `policy.Holder` pause deadline |

Each tool re-checks access itself. Rules and refreshes are refused on a replica, whose primary
would overwrite them; a pause is node-local and allowed.

## The query log

`dnsserver.QueryLog` is a 10,000-entry ring in memory. `logQuery` adds to it exactly when it
writes the stdout query line, with the client only under `log_client_ip`. `SetLogging` clears
it when either opt-in turns off, and a restart clears it. This is the one exception to
LOGGING.md's "no embedded log database": bounded, volatile, and no wider than the stdout log
the operator already opted into. History beyond the ring belongs in the operator's platform.

## Pause

`Holder.pausedUntil` is a unix-nanosecond deadline; `Decide` returns `paused` (not blocked,
not counted) before it. Resuming needs no timer or goroutine: the comparison against the clock
on every query is the auto-resume, and the expired deadline is cleared on first read. Bounds
1–60 minutes; a new pause replaces the old; memory only, so a restart resumes blocking. The
policy runs before the cache, so a name forwarded during a pause is blocked again as soon as
it ends. The Blacklists page shows a pause with **Resume now**.

## Authentication at `/mcp`

1. Any `Origin` header: 403. No CORS; the preflight is refused.
2. Only `Authorization: Bearer`; cookies never authenticate. Missing: 401 with
   `WWW-Authenticate: Bearer`.
3. Admin token, idle token, revoked identity or wrong secret: the same 401.
4. Audit `mcp.auth_failed` only for a real token (by ID), so unauthenticated callers cannot
   grow the audit table. KyDNS's admin API has no failure limiter either; tokens are 256-bit.
5. 1 MiB request body cap.

## Browser origins

Added after the Joplin Web Clipper advisory (GHSA-9728-v7ww-mxjv): a website minted a
full-access token through an endpoint with wildcard CORS, no Origin check and an anonymous
consent prompt. `web.TransportGate` (outermost, covering UI, `/api/v1` and `/mcp`) refuses a
Host outside `localhost`/IP literals/`KYDNS_ADMIN_HOSTNAMES` (proxied requests pass when none is
configured: TLS binds the name), any cross-origin preflight, and any cross-origin state change
(`Origin` mismatch or `null`, `Sec-Fetch-Site` cross-site or same-site). No route sends CORS
headers. `withMCPAuth` refuses any `Origin`. The token banner and CLI name the agent and spell
out the access (`registry.AccessText`); agent tokens require a label. Tests:
`internal/web/origin_test.go`, one per requirement, run through the full handler stack.

## Testing

`internal/adminapi/mcp_test.go`: authentication (Origin, admin token, Basic, cookie, wrong
secret, agent token on `/api/v1`, audit without secrets); the access matrix (every tool ×
read/write × query-log flag); idle revocation at 89 and 91 days, the sweep, admin tokens
untouched; pause bounds and auto-resume with an injected clock; explain naming rule, list and
entry; query search filters and limits; token API create/list/patch; replica refusals; skill
drift. `internal/dnsserver/querylog_test.go`: the ring follows the opt-ins. `internal/store`:
migration keeps old tokens admin; `ResetAdmin` removes agent tokens. `internal/web`: the token
form and pause banner.
