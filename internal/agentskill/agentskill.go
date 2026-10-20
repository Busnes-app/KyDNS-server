// Package agentskill holds the SKILL.md that teaches AI agents (OpenClaw, Hermes Agent) to use
// KyDNS's MCP tools. Each instance serves the copy matching its own tools.
package agentskill

import _ "embed"

// KyDNS is kydns/SKILL.md, served at /agent/skills/kydns/SKILL.md.
//
//go:embed kydns/SKILL.md
var KyDNS []byte
