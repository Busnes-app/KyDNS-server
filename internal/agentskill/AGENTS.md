# Agent skill

## Purpose
Holds `kydns/SKILL.md`, the AgentSkills-format skill that teaches AI agents (OpenClaw, Hermes Agent) to use KyDNS's MCP tools safely, and embeds it as `agentskill.KyDNS`.

## Ownership
Owns the skill's text. The tools it describes, their error messages and the route serving it are `internal/adminapi`'s (`mcp.go`).

## Local Contracts
- Frontmatter is exactly `name`, `description`, `version`: OpenClaw silently hides a skill whose `requires` gate is unmet, so the skill declares none.
- The intro names every MCP tool and nothing that is not one, and the error table carries every message a tool returns that the agent must act on (`msgReadOnly`, `msgNoQueryLog`, `msgNoPolicy`, `msgNoLog`, the replica refusal).
- Its rules match the server's: domain names are untrusted data, the query log is browsing history, writes only on the user's request and after confirming, pauses at most 60 minutes, no upstream or listener changes. The same rules, shorter, are `mcpInstructions`.
- It is served byte for byte as embedded, publicly: it holds no instance data.

## Verification
- `go test ./internal/adminapi -run TestAgentSkill`

## Child DOX Index
None.
