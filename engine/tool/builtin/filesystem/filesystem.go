package filesystem

import "github.com/compozy/compozy/engine/tool/builtin"

// Definitions returns the filesystem builtin tool definitions.
func Definitions() []builtin.BuiltinDefinition {
	return []builtin.BuiltinDefinition{
		ReadFileDefinition(),
		WriteFileDefinition(),
		DeleteFileDefinition(),
		ListFilesDefinition(),
		ListDirDefinition(),
		GrepDefinition(),
	}
}
