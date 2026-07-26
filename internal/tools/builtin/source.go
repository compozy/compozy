package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

// Source returns the provenance shared by daemon-compiled AGH tools.
func Source() toolspkg.SourceRef {
	return toolspkg.BuiltinSource()
}
