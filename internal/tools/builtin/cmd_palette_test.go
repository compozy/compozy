package builtin

import (
	"bytes"
	"slices"
	"testing"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestCmdPaletteDescriptors(t *testing.T) {
	t.Parallel()

	t.Run("Should publish list and invoke descriptors on the catalog toolset", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		list := descriptors[toolspkg.ToolIDCmdPaletteList]
		invoke := descriptors[toolspkg.ToolIDCmdPaletteInvoke]
		if !list.ReadOnly || list.Risk != toolspkg.RiskRead || list.Backend.NativeName != "cmd_palette_list" {
			t.Fatalf("list descriptor = %#v", list)
		}
		if invoke.ReadOnly || invoke.Destructive || invoke.Risk != toolspkg.RiskMutating ||
			invoke.Backend.NativeName != "cmd_palette_invoke" {
			t.Fatalf("invoke descriptor = %#v", invoke)
		}
		catalog, err := ToolsetCatalog()
		if err != nil {
			t.Fatalf("ToolsetCatalog() error = %v", err)
		}
		universe := make([]toolspkg.ToolID, 0, len(descriptors))
		for id := range descriptors {
			universe = append(universe, id)
		}
		expanded, err := catalog.Expand(toolspkg.ToolsetIDCatalog, universe)
		if err != nil {
			t.Fatalf("Expand(catalog) error = %v", err)
		}
		if !slices.Contains(expanded, toolspkg.ToolIDCmdPaletteList) ||
			!slices.Contains(expanded, toolspkg.ToolIDCmdPaletteInvoke) ||
			!slices.Contains(expanded, toolspkg.ToolIDProfileList) ||
			!slices.Contains(expanded, toolspkg.ToolIDProfileCurrent) {
			t.Fatalf("catalog tools = %#v, want command palette and profile tools", expanded)
		}
		if !bytes.Contains(list.OutputSchema, []byte(`"commands"`)) {
			t.Fatalf("list output schema = %s, want commands", list.OutputSchema)
		}
		if !bytes.Contains(invoke.OutputSchema, []byte(`"status"`)) ||
			!bytes.Contains(invoke.OutputSchema, []byte(`"invocation_id"`)) {
			t.Fatalf("invoke output schema = %s, want status and invocation_id", invoke.OutputSchema)
		}
	})
}
