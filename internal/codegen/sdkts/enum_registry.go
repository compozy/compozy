package sdkts

import (
	"reflect"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/tools"
)

const enumUnknownValue = "unknown"

var enumValuesRegistry = map[reflect.Type][]string{
	reflect.TypeFor[session.Badge](): {
		"running",
		"idle",
		"unhealthy",
		"hung",
		"waiting-for-auth",
		"waiting-for-input",
		"done",
		"stopped",
		"failed",
		enumUnknownValue,
		"needs-attention",
		"waiting",
	},
	reflect.TypeFor[session.BoardColumn]():      {"needs_you", "working", "waiting", "in_review", "done"},
	reflect.TypeFor[session.WaitingOn]():        {"sessions", "monitor"},
	reflect.TypeFor[session.PullRequestState](): {"open", "closed", "merged"},
	reflect.TypeFor[session.ChecksState]():      {"passing", "failing", "pending", "none", enumUnknownValue},
	reflect.TypeFor[session.ReviewDecision](): {
		"approved", "changes_requested", "review_required", "none", enumUnknownValue,
	},
	reflect.TypeFor[session.Mergeable]():                            {"mergeable", "conflicting", enumUnknownValue},
	reflect.TypeFor[session.LookupState]():                          {"ok", "pending", "error", "incomplete"},
	reflect.TypeFor[session.Disposition]():                          session.DispositionValues(),
	reflect.TypeFor[store.SteerDeliveryMode]():                      store.SteerDeliveryModeValues(),
	reflect.TypeFor[extensioncontract.CommandFlagType]():            extensioncontract.CommandFlagTypeValues(),
	reflect.TypeFor[apicontract.IssueSeverity]():                    apicontract.IssueSeverityValues(),
	reflect.TypeFor[apicontract.AuthoredValidationStatus]():         apicontract.AuthoredValidationStatusValues(),
	reflect.TypeFor[apicontract.AuthoredDiagnosticSeverity]():       apicontract.AuthoredDiagnosticSeverityValues(),
	reflect.TypeFor[apicontract.AgentSoulRevisionAction]():          apicontract.AgentSoulRevisionActionValues(),
	reflect.TypeFor[apicontract.HeartbeatRevisionOperation]():       apicontract.HeartbeatRevisionOperationValues(),
	reflect.TypeFor[apicontract.HeartbeatActorKind]():               apicontract.HeartbeatActorKindValues(),
	reflect.TypeFor[apicontract.SessionHealthState]():               apicontract.SessionHealthStateValues(),
	reflect.TypeFor[apicontract.SessionHealthStatus]():              apicontract.SessionHealthStatusValues(),
	reflect.TypeFor[apicontract.SessionHealthIneligibilityReason](): apicontract.SessionHealthIneligibilityReasonValues(),
	reflect.TypeFor[apicontract.HeartbeatWakeSource]():              apicontract.HeartbeatWakeSourceValues(),
	reflect.TypeFor[apicontract.HeartbeatWakeResult]():              apicontract.HeartbeatWakeResultValues(),
	reflect.TypeFor[apicontract.HeartbeatWakeReason]():              apicontract.HeartbeatWakeReasonValues(),
	reflect.TypeFor[apicontract.LoopProvenanceRole]():               apicontract.LoopProvenanceRoleValues(),
	reflect.TypeFor[extensionprotocol.HostAPIMethod]():              hostAPIMethodValues(),
	reflect.TypeFor[hooks.HookEvent]():                              hookEventValues(),
	reflect.TypeFor[hooks.HookEventFamily]():                        hookEventFamilyValues(),
	reflect.TypeFor[hooks.HookMode]():                               hookModeValues(),
	reflect.TypeFor[hooks.HookRunOutcome]():                         hookOutcomeValues(),
	reflect.TypeFor[hooks.LoopGenerationOrigin]():                   hooks.LoopGenerationOriginValues(),
	reflect.TypeFor[hooks.InputMentionKind]():                       hooks.InputMentionKindValues(),
	reflect.TypeFor[hooks.HookSkillSource]():                        hookSkillSourceValues(),
	reflect.TypeFor[hooks.HookExecutorKind]():                       hookExecutorKindValues(),
	reflect.TypeFor[hooks.HookSource]():                             hookSourceValues(),
	reflect.TypeFor[modelcatalog.ReasoningSource]():                 modelcatalog.ReasoningSourceValues(),
	reflect.TypeFor[modelcatalog.CostStatus]():                      modelcatalog.CostStatusValues(),
	reflect.TypeFor[modelcatalog.CostSource]():                      modelcatalog.CostSourceValues(),
	reflect.TypeFor[session.State]():                                sessionStateValues(),
	reflect.TypeFor[store.StopReason]():                             stopReasonValues(),
	reflect.TypeFor[tools.ToolSource]():                             toolSourceValues(),
}

var generatedTypeNameOverrides = map[reflect.Type]string{
	reflect.TypeFor[modelcatalog.ReasoningEffort](): "ReasoningEffort",
}

func generatedTypeName(t reflect.Type) string {
	if name, ok := generatedTypeNameOverrides[t]; ok {
		return name
	}
	return t.Name()
}
