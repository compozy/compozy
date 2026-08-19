package builtin

import (
	"slices"
	"testing"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestCmdPaletteDescriptors(t *testing.T) {
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
		!slices.Contains(expanded, toolspkg.ToolIDCmdPaletteInvoke) {
		t.Fatalf("catalog tools = %#v, want command palette list and invoke", expanded)
	}
}
