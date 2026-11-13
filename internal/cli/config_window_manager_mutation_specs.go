package cli

import (
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

type windowManagerMutationSpec struct {
	kind    configSetValueKind
	retired bool // Accepted and ignored at the CLI boundary until v0.8.0.
}

var windowManagerMutationSpecs = map[string]windowManagerMutationSpec{
	"new_window_policy":      {kind: configSetString},
	"small_viewport_policy":  {kind: configSetString},
	"focus_policy":           {kind: configSetString},
	"focus_wrap":             {kind: configSetBool},
	"focus_follows_pointer":  {kind: configSetBool},
	"swap_modifier":          {kind: configSetString},
	"history_limit":          {kind: configSetInt},
	"nav_stack_limit":        {kind: configSetInt},
	"closed_entry_limit":     {kind: configSetInt},
	"desktop_transition":     {kind: configSetString},
	"gaps.inner":             {kind: configSetInt},
	"gaps.top":               {kind: configSetInt},
	"gaps.right":             {kind: configSetInt},
	"gaps.bottom":            {kind: configSetInt},
	"gaps.left":              {kind: configSetInt},
	"raise_on_focus":         {kind: configSetBool, retired: true},
	"drag_away_policy":       {kind: configSetString, retired: true},
	"group_move_modifier":    {kind: configSetString, retired: true},
	"snap.edge_band":         {kind: configSetInt, retired: true},
	"snap.corner_reach":      {kind: configSetInt, retired: true},
	"snap.exit_slack":        {kind: configSetInt, retired: true},
	"snap.repeat_ratios":     {kind: configSetFloatSlice, retired: true},
	"bindings.top_center":    {kind: configSetString, retired: true},
	"bindings.bottom_center": {kind: configSetString, retired: true},
}

var windowManagerShortcutMutationSpec = windowManagerMutationSpec{kind: configSetStringOrStringSlice}
var windowManagerGlobalShortcutMutationSpec = windowManagerMutationSpec{kind: configSetString}

func lookupWindowManagerMutationSpec(path []string) (windowManagerMutationSpec, bool) {
	if len(path) < 2 || len(path) > 3 || path[0] != configWindowManagerKey {
		return windowManagerMutationSpec{}, false
	}
	if len(path) == 3 && path[1] == configWindowManagerShortcutsKey {
		return windowManagerShortcutMutationSpec, true
	}
	if len(path) == 3 && path[1] == configWindowManagerGlobalShortcutsKey {
		return windowManagerGlobalShortcutMutationSpec, true
	}
	spec, ok := windowManagerMutationSpecs[strings.Join(path[1:], ".")]
	return spec, ok
}

// Retired keys accept any value and never reach persistence until removal in v0.8.0.
func ignoreRetiredWindowManagerConfigSet(
	cmd *cobra.Command,
	target compozyconfig.WriteTarget,
	path []string,
	raw string,
	redacted bool,
) (bool, error) {
	spec, ok := lookupWindowManagerMutationSpec(path)
	if !ok || !spec.retired {
		return false, nil
	}
	if err := compozyconfig.ValidateConfigWriteScope(target.Scope(), path); err != nil {
		return true, fmt.Errorf("cli: %w", err)
	}
	value, err := parseConfigSetValue(spec.kind, raw)
	if err != nil {
		value = raw
	}
	cmd.PrintErrln(windowmanager.RetiredConfigWarning(strings.Join(path[1:], "."), false))
	record := configSetRecordForLocalWrite(path, value, target, redacted, classifyConfigSetLifecycle(path))
	record.Applied, record.RestartRequired, record.RestartScope = false, false, ""
	record.NextAction = "ignored retired setting; remove before v0.8.0."
	return true, writeCommandOutput(cmd, configSetBundle(record))
}

func translateRetiredWindowManagerConfigSetValue(cmd *cobra.Command, path []string, value any) any {
	if len(path) == 2 && path[0] == configWindowManagerKey && path[1] == "new_window_policy" &&
		value == compozyconfig.WindowNewPolicyFloating {
		cmd.PrintErrln(windowmanager.RetiredConfigWarning("new_window_policy", true))
		return compozyconfig.WindowNewPolicyTab
	}
	return value
}
