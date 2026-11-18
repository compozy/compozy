package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// collapseWorkspaceScopedHooks keeps one entry per set of hooks that differ only in their
// workspace matcher and pipeline order, at the lowest order of the set.
func collapseWorkspaceScopedHooks(hooks []HookCatalogRecord) []HookCatalogRecord {
	collapsed := make([]HookCatalogRecord, 0, len(hooks))
	index := make(map[string]int, len(hooks))
	for _, hook := range hooks {
		key := workspaceScopedHookKey(hook)
		if at, ok := index[key]; ok {
			collapsed[at].Order = min(collapsed[at].Order, hook.Order)
			continue
		}
		index[key] = len(collapsed)
		collapsed = append(collapsed, hook)
	}
	return collapsed
}

func workspaceScopedHookKey(hook HookCatalogRecord) string {
	hook.Order = 0
	hook.Matcher.WorkspaceID = ""
	hook.Matcher.WorkspaceRoot = ""
	encoded, err := json.Marshal(hook)
	if err != nil {
		return fmt.Sprintf("%#v", hook)
	}
	return string(encoded)
}

func hookListBundle(hooks []HookCatalogRecord) outputBundle {
	return listBundle(
		hooks,
		hooks,
		"Hooks",
		[]string{
			"Order",
			automationNameValue,
			hooksEventValue,
			authoredContextSourceValue,
			taskModeValue,
			cliPriorityValue,
		},
		"hooks",
		[]string{
			hooksOrderKey,
			automationNameKey,
			hooksEventKey,
			automationSourceKey,
			"skill_source",
			hooksModeKey,
			configRequiredKey,
			"priority",
		},
		func(item HookCatalogRecord) []string {
			return []string{
				strconv.Itoa(item.Order),
				stringOrDash(item.Name),
				stringOrDash(item.Event),
				stringOrDash(item.Source),
				stringOrDash(item.Mode),
				strconv.Itoa(item.Priority),
			}
		},
		func(item HookCatalogRecord) []string {
			return []string{
				strconv.Itoa(item.Order),
				item.Name,
				item.Event,
				item.Source,
				item.SkillSource,
				item.Mode,
				strconv.FormatBool(item.Required),
				strconv.Itoa(item.Priority),
			}
		},
	)
}
