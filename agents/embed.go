package agents

import "embed"

// FS holds the bundled reusable-agent fixtures installed by `compozy setup`.
//
//go:embed *
var FS embed.FS
