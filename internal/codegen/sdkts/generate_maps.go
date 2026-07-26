package sdkts

import (
	"reflect"
	"sort"

	"strings"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/hooks"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (g *generator) emitHookMaps() {
	var payload strings.Builder
	payload.Grow(estimatedHookMapSize("export interface HookPayloadByEvent {\n", g.hookSpecs, true))
	payload.WriteString("export interface HookPayloadByEvent {\n")
	for _, spec := range g.hookSpecs {
		payload.WriteString("  ")
		writeQuotedString(&payload, string(spec.Event))
		payload.WriteString(": ")
		payload.WriteString(spec.Payload.Name)
		payload.WriteString(";\n")
	}
	payload.WriteString("}\n")
	g.blocks = append(g.blocks, payload.String())

	var patch strings.Builder
	patch.Grow(estimatedHookMapSize("export interface HookPatchByEvent {\n", g.hookSpecs, false))
	patch.WriteString("export interface HookPatchByEvent {\n")
	for _, spec := range g.hookSpecs {
		patch.WriteString("  ")
		writeQuotedString(&patch, string(spec.Event))
		patch.WriteString(": ")
		patch.WriteString(spec.Patch.Name)
		patch.WriteString(";\n")
	}
	patch.WriteString("}\n")
	g.blocks = append(g.blocks, patch.String())
}

func (g *generator) emitHostMethodMap() {
	var out strings.Builder
	out.Grow(estimatedHostMethodMapSize(g.hostSpecs))
	out.WriteString("export interface HostAPIMethodMap {\n")
	for _, spec := range g.hostSpecs {
		paramsType := spec.Params.Name
		if spec.OptionalParams {
			if spec.Params.Name == "EmptyResult" {
				paramsType = "undefined"
			} else {
				paramsType += " | undefined"
			}
		}
		resultType := spec.Result.Name
		if resultIsList(spec.Result.Value) {
			resultType += "[]"
		}
		out.WriteString("  ")
		writeQuotedString(&out, string(spec.Method))
		out.WriteString(": {\n")
		out.WriteString("    params: ")
		out.WriteString(paramsType)
		out.WriteString(";\n")
		out.WriteString("    result: ")
		out.WriteString(resultType)
		out.WriteString(";\n")
		out.WriteString("  };\n")
	}
	out.WriteString("}\n")
	g.blocks = append(g.blocks, out.String())
}

func resultIsList(value any) bool {
	if value == nil {
		return false
	}
	t := reflect.TypeOf(value)
	return t.Kind() == reflect.Slice || t.Kind() == reflect.Array
}

func hostAPIMethodValues() []string {
	specs := extensioncontract.HostAPIMethodSpecs()
	values := make([]string, 0, len(specs))
	for _, spec := range specs {
		values = append(values, string(spec.Method))
	}
	sort.Strings(values)
	return values
}

func hookEventValues() []string {
	events := hooks.AllHookEvents()
	values := make([]string, 0, len(events))
	for _, event := range events {
		values = append(values, string(event))
	}
	return values
}

func hookEventFamilyValues() []string {
	return hookEventFamilyValuesFromEvents(hooks.AllHookEvents())
}

func hookEventFamilyValuesFromEvents(events []hooks.HookEvent) []string {
	values := make([]string, 0, len(events))
	seen := map[hooks.HookEventFamily]struct{}{}
	for _, event := range events {
		family := event.Family()
		if family == "" {
			continue
		}
		if _, ok := seen[family]; ok {
			continue
		}
		seen[family] = struct{}{}
		values = append(values, string(family))
	}
	return values
}

func hookModeValues() []string {
	return []string{string(hooks.HookModeSync), string(hooks.HookModeAsync)}
}

func hookOutcomeValues() []string {
	return []string{
		string(hooks.HookRunOutcomeApplied),
		string(hooks.HookRunOutcomeDenied),
		string(hooks.HookRunOutcomeFailed),
		string(hooks.HookRunOutcomeSkipped),
		string(hooks.HookRunOutcomeDropped),
		string(hooks.HookRunOutcomeRejected),
	}
}

func hookSkillSourceValues() []string {
	return []string{
		string(hooks.HookSkillSourceBundled),
		string(hooks.HookSkillSourceMarketplace),
		string(hooks.HookSkillSourceUser),
		string(hooks.HookSkillSourceAdditional),
		string(hooks.HookSkillSourceWorkspace),
	}
}

func hookExecutorKindValues() []string {
	return []string{
		string(hooks.HookExecutorNative),
		string(hooks.HookExecutorSubprocess),
		string(hooks.HookExecutorWASM),
	}
}

func hookSourceValues() []string {
	return []string{"native", "config", "agent_definition", "skill"}
}

func memoryTypeValues() []string {
	return []string{
		string(memcontract.TypeUser),
		string(memcontract.TypeFeedback),
		string(memcontract.TypeProject),
		string(memcontract.TypeReference),
	}
}

func memoryScopeValues() []string {
	return []string{string(memcontract.ScopeGlobal), string(memcontract.ScopeWorkspace), string(memcontract.ScopeAgent)}
}

func sessionStateValues() []string {
	return []string{
		string(session.StateStarting),
		string(session.StateActive),
		string(session.StateStopping),
		string(session.StateStopped),
	}
}

func stopReasonValues() []string {
	return []string{
		string(store.StopCompleted),
		string(store.StopUserCanceled),
		string(store.StopMaxIterations),
		string(store.StopLoopDetected),
		string(store.StopTimeout),
		string(store.StopBudgetExceeded),
		string(store.StopError),
		string(store.StopAgentCrashed),
		string(store.StopHookStopped),
		string(store.StopShutdown),
	}
}

func toolSourceValues() []string {
	return []string{"builtin", "mcp", "extension", "dynamic"}
}

func shouldAutoEmitNamedType(t reflect.Type) bool {
	if t == nil || t.Name() == "" {
		return false
	}
	if !strings.HasPrefix(t.PkgPath(), "github.com/compozy/agh/internal/") {
		return false
	}
	return t.Kind() == reflect.Struct || isEnumType(t) || isPrimitiveAliasType(t)
}
