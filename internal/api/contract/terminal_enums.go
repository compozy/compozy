package contract

type TerminalState string

const (
	TerminalStateRunning TerminalState = "running"
	TerminalStateExited  TerminalState = "exited"
)

func TerminalStateValues() []string {
	return []string{string(TerminalStateRunning), string(TerminalStateExited)}
}

type TerminalExitCause string

const (
	TerminalExitCauseExited   TerminalExitCause = "exited"
	TerminalExitCauseSignaled TerminalExitCause = "signaled"
	TerminalExitCauseUnknown  TerminalExitCause = "unknown"
)

func TerminalExitCauseValues() []string {
	return []string{
		string(TerminalExitCauseExited),
		string(TerminalExitCauseSignaled),
		string(TerminalExitCauseUnknown),
	}
}

type TerminalCommandDetection string

const (
	TerminalCommandDetectionExact  TerminalCommandDetection = "exact"
	TerminalCommandDetectionMarker TerminalCommandDetection = "marker"
	TerminalCommandDetectionIdle   TerminalCommandDetection = "idle"
)

func TerminalCommandDetectionValues() []string {
	return []string{
		string(TerminalCommandDetectionExact),
		string(TerminalCommandDetectionMarker),
		string(TerminalCommandDetectionIdle),
	}
}

type TerminalCommandApproval string

const (
	TerminalCommandApprovalOnce        TerminalCommandApproval = "approved_once"
	TerminalCommandApprovalAlways      TerminalCommandApproval = "approved_always"
	TerminalCommandApprovalAllowlisted TerminalCommandApproval = "allowlisted"
	TerminalCommandApprovalHuman       TerminalCommandApproval = "human"
	TerminalCommandApprovalNone        TerminalCommandApproval = "none"
)

func TerminalCommandApprovalValues() []string {
	return []string{
		string(TerminalCommandApprovalOnce),
		string(TerminalCommandApprovalAlways),
		string(TerminalCommandApprovalAllowlisted),
		string(TerminalCommandApprovalHuman),
		string(TerminalCommandApprovalNone),
	}
}

type TerminalAttachMode string

const (
	TerminalAttachModeRead  TerminalAttachMode = "read"
	TerminalAttachModeWrite TerminalAttachMode = "write"
)

func TerminalAttachModeValues() []string {
	return []string{string(TerminalAttachModeRead), string(TerminalAttachModeWrite)}
}

type TerminalRecordingAction string

const (
	TerminalRecordingActionStart TerminalRecordingAction = "start"
	TerminalRecordingActionStop  TerminalRecordingAction = "stop"
)

func TerminalRecordingActionValues() []string {
	return []string{string(TerminalRecordingActionStart), string(TerminalRecordingActionStop)}
}

type TerminalRecordingState string

const (
	TerminalRecordingStateRecording TerminalRecordingState = "recording"
	TerminalRecordingStateSaved     TerminalRecordingState = "saved"
)

func TerminalRecordingStateValues() []string {
	return []string{string(TerminalRecordingStateRecording), string(TerminalRecordingStateSaved)}
}

type TerminalInputRejectOutcome string

const TerminalInputRejectOutcomeRejected TerminalInputRejectOutcome = "rejected"

func TerminalInputRejectOutcomeValues() []string {
	return []string{string(TerminalInputRejectOutcomeRejected)}
}
