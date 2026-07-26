package sdkts

import (
	"reflect"

	apicontract "github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/hooks"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/tools"
)

var enumValuesRegistry = map[reflect.Type][]string{
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
	reflect.TypeFor[bridgepkg.DeliveryAckOutcome]():                 bridgepkg.DeliveryAckOutcomeValues(),
	reflect.TypeFor[bridgepkg.DeliveryEventType]():                  bridgepkg.DeliveryEventTypeValues(),
	reflect.TypeFor[bridgepkg.InboundEventFamily]():                 bridgepkg.InboundEventFamilyValues(),
	reflect.TypeFor[bridgepkg.InboundEditOperation]():               bridgepkg.InboundEditOperationValues(),
	reflect.TypeFor[bridgepkg.ToolProgressPhase]():                  bridgepkg.ToolProgressPhaseValues(),
	reflect.TypeFor[bridgepkg.ControlMethod]():                      bridgepkg.ControlMethodValues(),
	reflect.TypeFor[bridgepkg.BridgeCheckStatus]():                  bridgepkg.BridgeCheckStatusValues(),
	reflect.TypeFor[subprocess.BridgeRuntimePurpose]():              subprocess.BridgeRuntimePurposeValues(),
	reflect.TypeFor[extensionprotocol.HostAPIMethod]():              hostAPIMethodValues(),
	reflect.TypeFor[hooks.HookEvent]():                              hookEventValues(),
	reflect.TypeFor[hooks.HookEventFamily]():                        hookEventFamilyValues(),
	reflect.TypeFor[hooks.HookMode]():                               hookModeValues(),
	reflect.TypeFor[hooks.HookRunOutcome]():                         hookOutcomeValues(),
	reflect.TypeFor[hooks.HookSkillSource]():                        hookSkillSourceValues(),
	reflect.TypeFor[hooks.HookExecutorKind]():                       hookExecutorKindValues(),
	reflect.TypeFor[hooks.HookSource]():                             hookSourceValues(),
	reflect.TypeFor[memcontract.Type]():                             memoryTypeValues(),
	reflect.TypeFor[memcontract.Scope]():                            memoryScopeValues(),
	reflect.TypeFor[modelcatalog.ReasoningEffort]():                 modelcatalog.ReasoningEffortValues(),
	reflect.TypeFor[modelcatalog.ReasoningSource]():                 modelcatalog.ReasoningSourceValues(),
	reflect.TypeFor[modelcatalog.CostStatus]():                      modelcatalog.CostStatusValues(),
	reflect.TypeFor[modelcatalog.CostSource]():                      modelcatalog.CostSourceValues(),
	reflect.TypeFor[participation.Mode]():                           participationModeValues(),
	reflect.TypeFor[participation.ChannelStrategy]():                participationChannelStrategyValues(),
	reflect.TypeFor[participation.Source]():                         participationSourceValues(),
	reflect.TypeFor[participation.OwnerKind]():                      participationOwnerKindValues(),
	reflect.TypeFor[session.State]():                                sessionStateValues(),
	reflect.TypeFor[store.StopReason]():                             stopReasonValues(),
	reflect.TypeFor[tools.ToolSource]():                             toolSourceValues(),
}

var generatedTypeNameOverrides = map[reflect.Type]string{
	reflect.TypeFor[modelcatalog.ReasoningEffort]():  "ReasoningEffort",
	reflect.TypeFor[participation.Request]():         "NetworkParticipationRequest",
	reflect.TypeFor[participation.Spec]():            "NetworkParticipationSpec",
	reflect.TypeFor[participation.BoundsRequest]():   "NetworkParticipationBoundsRequest",
	reflect.TypeFor[participation.Bounds]():          "NetworkParticipationBounds",
	reflect.TypeFor[participation.Mode]():            "NetworkParticipationMode",
	reflect.TypeFor[participation.ChannelStrategy](): "NetworkParticipationChannelStrategy",
	reflect.TypeFor[participation.Source]():          "NetworkParticipationSource",
	reflect.TypeFor[participation.OwnerKind]():       "NetworkParticipationOwnerKind",
	reflect.TypeFor[participation.OwnerRef]():        "NetworkParticipationOwnerRef",
	reflect.TypeFor[participation.Status]():          "NetworkParticipationStatus",
}

func generatedTypeName(t reflect.Type) string {
	if name, ok := generatedTypeNameOverrides[t]; ok {
		return name
	}
	return t.Name()
}
