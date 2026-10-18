// Remove the leftover-reference filter in v0.6.0; retain the retired ID tombstones.
package tools

import (
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// RetiredMemoryToolIDs are tombstones and must never be reused.
var RetiredMemoryToolIDs = []ToolID{
	"compozy__memory_admin_history",
	"compozy__memory_daily_list",
	"compozy__memory_decisions_list",
	"compozy__memory_decisions_revert",
	"compozy__memory_decisions_show",
	"compozy__memory_dream_list",
	"compozy__memory_dream_retry",
	"compozy__memory_dream_show",
	"compozy__memory_dream_status",
	"compozy__memory_dream_trigger",
	"compozy__memory_extractor_drain",
	"compozy__memory_extractor_failures",
	"compozy__memory_extractor_retry",
	"compozy__memory_extractor_status",
	"compozy__memory_health",
	"compozy__memory_list",
	"compozy__memory_note",
	"compozy__memory_promote",
	"compozy__memory_propose",
	"compozy__memory_provider_disable",
	"compozy__memory_provider_enable",
	"compozy__memory_provider_get",
	"compozy__memory_provider_list",
	"compozy__memory_provider_select",
	"compozy__memory_recall_trace",
	"compozy__memory_reindex",
	"compozy__memory_reload",
	"compozy__memory_reset",
	"compozy__memory_scope_show",
	"compozy__memory_search",
	"compozy__memory_session_ledger",
	"compozy__memory_session_replay",
	"compozy__memory_sessions_prune",
	"compozy__memory_sessions_repair",
	"compozy__memory_show",
}

var RetiredMemoryToolsetIDs = []ToolsetID{"compozy__memory", "compozy__memory_admin"}

// ToolPolicy retains authored references until boundary validation resolves them.
type ToolPolicy struct {
	Tools     []string
	Toolsets  []string
	DenyTools []string
}

// DropRetiredToolReferences ignores only explicitly retired references without mutating the input.
func DropRetiredToolReferences(policy ToolPolicy) (filtered ToolPolicy, dropped []string) {
	filter := func(values []string, toolsets bool) []string {
		if values == nil {
			return nil
		}
		result := make([]string, 0, len(values))
		for _, raw := range values {
			id := strings.TrimSpace(raw)
			retired := slices.Contains(RetiredMemoryToolIDs, ToolID(id))
			if toolsets {
				retired = slices.Contains(RetiredMemoryToolsetIDs, ToolsetID(id))
			}
			if retired {
				if !slices.Contains(dropped, id) {
					dropped = append(dropped, id)
				}
				continue
			}
			result = append(result, raw)
		}
		return result
	}
	filtered.Toolsets = filter(policy.Toolsets, true)
	filtered.Tools = filter(policy.Tools, false)
	filtered.DenyTools = filter(policy.DenyTools, false)
	return filtered, dropped
}

var retiredToolPolicyWarnings sync.Map

// WarnRetiredToolReferences reports leftovers once per policy owner per process.
func WarnRetiredToolReferences(owner string, dropped []string) {
	if len(dropped) == 0 {
		return
	}
	if _, loaded := retiredToolPolicyWarnings.LoadOrStore(owner, struct{}{}); loaded {
		return
	}
	slog.Warn("tools.retired_ids_ignored", "owner", owner, "ids", dropped)
}

func dropRetiredPolicyInputs(inputs PolicyInputs) PolicyInputs {
	filterPatterns := func(values []ToolPattern) []ToolPattern {
		return slices.DeleteFunc(slices.Clone(values), func(pattern ToolPattern) bool {
			id, exact := pattern.exactID()
			return exact && slices.Contains(RetiredMemoryToolIDs, id)
		})
	}
	filterToolsets := func(values []ToolsetID) []ToolsetID {
		return slices.DeleteFunc(slices.Clone(values), func(id ToolsetID) bool {
			return slices.Contains(RetiredMemoryToolsetIDs, id)
		})
	}
	inputs.AllowTools = filterPatterns(inputs.AllowTools)
	inputs.DenyTools = filterPatterns(inputs.DenyTools)
	inputs.AllowToolsets = filterToolsets(inputs.AllowToolsets)
	inputs.Agent.Enforced = inputs.Agent.Enforced || len(inputs.Agent.Tools) > 0 || len(inputs.Agent.Toolsets) > 0
	inputs.Agent.Tools = filterPatterns(inputs.Agent.Tools)
	inputs.Agent.DenyTools = filterPatterns(inputs.Agent.DenyTools)
	inputs.Agent.Toolsets = filterToolsets(inputs.Agent.Toolsets)
	inputs.Session.Tools = slices.DeleteFunc(slices.Clone(inputs.Session.Tools), func(id ToolID) bool {
		return slices.Contains(RetiredMemoryToolIDs, id)
	})
	return inputs
}
