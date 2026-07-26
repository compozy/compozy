package spec

import (
	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func bridgeScopeValues() []string {
	return []string{string(bridgepkg.ScopeGlobal), string(bridgepkg.ScopeWorkspace)}
}

func bridgeInstanceSourceValues() []string {
	return []string{
		string(bridgepkg.BridgeInstanceSourceDynamic),
		string(bridgepkg.BridgeInstanceSourcePackage),
	}
}

func bridgeStatusValues() []string {
	return []string{
		string(bridgepkg.BridgeStatusAuthRequired),
		string(bridgepkg.BridgeStatusDegraded),
		string(bridgepkg.BridgeStatusDisabled),
		string(bridgepkg.BridgeStatusError),
		string(bridgepkg.BridgeStatusReady),
		string(bridgepkg.BridgeStatusStarting),
	}
}

func bridgeDMPolicyValues() []string {
	return []string{
		string(bridgepkg.BridgeDMPolicyOpen),
		string(bridgepkg.BridgeDMPolicyAllowlist),
		string(bridgepkg.BridgeDMPolicyPairing),
	}
}

func bridgeDegradationReasonValues() []string {
	return []string{
		string(bridgepkg.BridgeDegradationReasonAuthFailed),
		string(bridgepkg.BridgeDegradationReasonRateLimited),
		string(bridgepkg.BridgeDegradationReasonWebhookInvalid),
		string(bridgepkg.BridgeDegradationReasonProviderTimeout),
		string(bridgepkg.BridgeDegradationReasonTenantConfigInvalid),
	}
}

func bridgeDiagnosticKindValues() []string {
	return []string{
		string(bridgepkg.BridgeDiagnosticKindUnknownDestination),
		string(bridgepkg.BridgeDiagnosticKindMissingToken),
		string(bridgepkg.BridgeDiagnosticKindPermissionDenied),
		string(bridgepkg.BridgeDiagnosticKindUnsupportedCapability),
		string(bridgepkg.BridgeDiagnosticKindTransientDeliveryFailure),
	}
}

func bridgeDiagnosticSeverityValues() []string {
	return []string{
		string(bridgepkg.BridgeDiagnosticSeverityInfo),
		string(bridgepkg.BridgeDiagnosticSeverityWarning),
		string(bridgepkg.BridgeDiagnosticSeverityError),
	}
}

func deliveryModeValues() []string {
	return []string{
		string(bridgepkg.DeliveryModeDirectSend),
		string(bridgepkg.DeliveryModeReply),
	}
}

func sessionStateValues() []string {
	return []string{
		string(session.StateStarting),
		string(session.StateActive),
		string(session.StateStopping),
		string(session.StateStopped),
	}
}

func sessionTypeValues() []string {
	return []string{
		string(session.SessionTypeUser),
		string(session.SessionTypeDream),
		string(session.SessionTypeSystem),
		string(session.SessionTypeCoordinator),
		string(session.SessionTypeSpawned),
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
