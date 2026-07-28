//go:build mage

package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

const providerEnvImportPath = "github.com/compozy/compozy/internal/providerenv"

var restrictedProviderHomeKeys = map[string]struct{}{
	"CODEX_HOME":          {},
	"HOME":                {},
	"PROVIDER_CODEX_HOME": {},
	"PROVIDER_HOME":       {},
	"XDG_CACHE_HOME":      {},
	"XDG_CONFIG_HOME":     {},
	"XDG_DATA_HOME":       {},
}

func inspectProviderHomeMutations(
	fileset *token.FileSet,
	file *ast.File,
	imports map[string]string,
	rel string,
) []sourcePolicyViolation {
	violations := make([]sourcePolicyViolation, 0)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		keyIndex := -1
		switch {
		case importedCall(call, imports, "os", "Setenv"), importedCall(call, imports, "os", "Unsetenv"):
			keyIndex = 0
		case importedCall(call, imports, providerEnvImportPath, "SetEnvValue"):
			keyIndex = 1
		}
		if keyIndex < 0 || len(call.Args) <= keyIndex {
			return true
		}
		key, ok := stringLiteral(call.Args[keyIndex])
		if !ok {
			return true
		}
		if _, restricted := restrictedProviderHomeKeys[key]; !restricted {
			return true
		}
		violations = append(violations, sourcePolicyViolation{
			path:    rel,
			line:    fileset.Position(call.Pos()).Line,
			message: fmt.Sprintf("provider home key %q may only be mutated by internal/providerenv", key),
		})
		return true
	})
	return violations
}
