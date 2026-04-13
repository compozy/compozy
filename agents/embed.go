package agents

import "embed"

// FS holds the bundled reusable-agent fixtures installed by `compozy setup`.
//
//go:embed */AGENT.md
var FS embed.FS
