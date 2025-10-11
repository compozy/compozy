package prompts

import "embed"

// TemplateFS exposes the prompt template filesystem for orchestrator prompts.
//
//go:embed templates/*.tmpl
var TemplateFS embed.FS
