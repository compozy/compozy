package native

import (
	"sync"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/builtin/exec"
	"github.com/compozy/compozy/engine/tool/builtin/fetch"
	"github.com/compozy/compozy/engine/tool/builtin/filesystem"
)

var (
	definitionsOnce sync.Once
	definitions     []builtin.BuiltinDefinition
	definitionIndex map[string]builtin.BuiltinDefinition
)

func Definitions() []builtin.BuiltinDefinition {
	definitionsOnce.Do(initDefinitions)
	return append([]builtin.BuiltinDefinition(nil), definitions...)
}

func DefinitionByID(id string) (builtin.BuiltinDefinition, bool) {
	definitionsOnce.Do(initDefinitions)
	def, ok := definitionIndex[id]
	return def, ok
}

func initDefinitions() {
	fsDefs := filesystem.Definitions()
	fetchDefs := fetch.Definitions()
	defs := make([]builtin.BuiltinDefinition, 0, len(fsDefs)+1+len(fetchDefs))
	defs = append(defs, fsDefs...)
	defs = append(defs, exec.Definition())
	defs = append(defs, fetchDefs...)
	definitions = defs
	definitionIndex = make(map[string]builtin.BuiltinDefinition, len(defs))
	for _, def := range defs {
		definitionIndex[def.ID] = def
	}
}
