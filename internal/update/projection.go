package update

import (
	"slices"
	"time"
)

// MultiState is the live host-global projection shared by settings and app status.
type MultiState struct {
	Aggregate Status            `json:"aggregate"`
	Operation *OperationView    `json:"operation"`
	Runtime   RuntimeTrackState `json:"runtime"`
	App       *AppTrackState    `json:"app"`
}

// RuntimeTrackState is the complete runtime update projection.
type RuntimeTrackState struct {
	Status          Status `json:"status"`
	InstallMethod   string `json:"install_method"`
	Managed         bool   `json:"managed"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	ReleaseURL      string `json:"release_url,omitempty"`
	Recommendation  string `json:"recommendation,omitempty"`
	RestoredVersion string `json:"restored_version,omitempty"`
	DaemonRestarted bool   `json:"daemon_restarted"`
	Message         string `json:"message"`
	LastError       string `json:"last_error,omitempty"`
}

// AppTrackState is present exactly when a desktop app is installed.
type AppTrackState struct {
	Status         Status `json:"status"`
	Running        bool   `json:"running"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	ReleaseURL     string `json:"release_url,omitempty"`
	AttemptID      string `json:"attempt_id,omitempty"`
	LastError      string `json:"last_error,omitempty"`
	Message        string `json:"message"`
}

// OperationView is the transport-safe read projection of the durable journal.
type OperationView struct {
	ID           string         `json:"id"`
	Revision     int64          `json:"revision"`
	Targets      []Target       `json:"targets"`
	ActiveTarget Target         `json:"active_target,omitempty"`
	Phase        OperationPhase `json:"phase,omitempty"`
	Percent      int            `json:"percent"`
	Holder       *Holder        `json:"holder"`
	Waiting      WaitingState   `json:"waiting"`
	StartedAt    time.Time      `json:"started_at"`
	LastError    string         `json:"last_error,omitempty"`
}

// UIPhase is the fixed user-facing lifecycle vocabulary.
type UIPhase string

const (
	UIPhaseDownload   UIPhase = "download"
	UIPhaseVerify     UIPhase = "verify"
	UIPhaseInstall    UIPhase = "install"
	UIPhaseStart      UIPhase = "start"
	UIPhaseReadyCheck UIPhase = "ready-check"
	UIPhaseReady      UIPhase = "ready"
)

// ProjectMultiState derives every live track and operation field from one computation.
func ProjectMultiState(runtime State, app *AppTrackState, operation *Operation) MultiState {
	runtimeTrack := RuntimeTrackState{
		Status:          runtime.Status,
		InstallMethod:   runtime.InstallMethod,
		Managed:         runtime.Managed,
		CurrentVersion:  runtime.CurrentVersion,
		LatestVersion:   runtime.LatestVersion,
		ReleaseURL:      runtime.ReleaseURL,
		Recommendation:  runtime.Recommendation,
		RestoredVersion: runtime.RestoredVersion,
		DaemonRestarted: runtime.DaemonRestarted,
		Message:         runtime.Message,
		LastError:       runtime.LastError,
	}
	state := MultiState{Runtime: runtimeTrack, App: cloneAppTrackState(app)}
	if operation != nil {
		state.Operation = operationView(operation)
		applyOperationProjection(&state, operation)
	}
	state.Aggregate = AggregateLiveStatus(state.Runtime.Status, appStatus(state.App))
	return state
}

// AggregateLiveStatus applies the frozen live status precedence.
func AggregateLiveStatus(statuses ...Status) Status {
	return aggregateStatus(
		[]Status{
			StatusFailed,
			StatusBlocked,
			StatusApplying,
			StatusAccepted,
			StatusStaged,
			StatusUpdated,
			StatusAvailable,
		},
		statuses,
	)
}

// AggregateTerminalStatus applies the frozen CLI terminal precedence.
func AggregateTerminalStatus(statuses ...Status) Status {
	return aggregateStatus(
		[]Status{StatusFailed, StatusBlocked, StatusStaged, StatusUpdated, StatusAvailable},
		statuses,
	)
}

func aggregateStatus(precedence []Status, statuses []Status) Status {
	for _, candidate := range precedence {
		if slices.Contains(statuses, candidate) {
			return candidate
		}
	}
	return StatusUpToDate
}

func operationView(operation *Operation) *OperationView {
	return &OperationView{
		ID:           operation.ID,
		Revision:     operation.Revision,
		Targets:      append([]Target(nil), operation.Targets...),
		ActiveTarget: operation.ActiveTarget,
		Phase:        phaseForTarget(operation, operation.ActiveTarget),
		Percent:      operation.Percent,
		Holder:       cloneHolder(operation.Holder),
		Waiting:      operation.Waiting,
		StartedAt:    operation.StartedAt,
		LastError:    operation.LastError,
	}
}

func applyOperationProjection(state *MultiState, operation *Operation) {
	if operation.Waiting == WaitingForApp && state.App != nil {
		state.App.Status = StatusStaged
		state.App.AttemptID = operation.App.AttemptID
		return
	}
	switch operation.ActiveTarget {
	case TargetRuntime:
		switch operation.Runtime.Phase {
		case PhaseFailed, PhaseRolledBack:
			state.Runtime.Status = StatusFailed
		case PhasePending:
			state.Runtime.Status = StatusAccepted
		default:
			state.Runtime.Status = StatusApplying
		}
	case TargetApp:
		if state.App == nil {
			return
		}
		state.App.AttemptID = operation.App.AttemptID
		switch operation.App.Phase {
		case PhaseFailed:
			state.App.Status = StatusFailed
		case PhasePending:
			state.App.Status = StatusAccepted
		default:
			state.App.Status = StatusApplying
		}
	}
}

func cloneAppTrackState(state *AppTrackState) *AppTrackState {
	if state == nil {
		return nil
	}
	cloned := *state
	return &cloned
}

func appStatus(app *AppTrackState) Status {
	if app == nil {
		return StatusUpToDate
	}
	return app.Status
}

// PhaseForUI maps every measurable operation edge to the fixed UI phase vocabulary.
func PhaseForUI(target Target, phase OperationPhase, percent int) (UIPhase, bool) {
	switch target {
	case TargetRuntime:
		switch phase {
		case PhaseDownloading:
			return UIPhaseDownload, true
		case PhaseVerifying:
			return UIPhaseVerify, true
		case PhaseSwapping:
			return UIPhaseInstall, true
		case PhaseRestarting:
			return UIPhaseStart, true
		case PhaseHealthChecking:
			return UIPhaseReadyCheck, true
		case PhaseFinalized:
			return UIPhaseReady, true
		}
	case TargetApp:
		switch phase {
		case PhaseApplying:
			if percent >= 100 {
				return UIPhaseVerify, true
			}
			return UIPhaseDownload, true
		case PhaseInstallerHandoff:
			return UIPhaseInstall, true
		case PhaseRestarted:
			return UIPhaseStart, true
		case PhaseVerified:
			return UIPhaseReady, true
		}
	}
	return "", false
}
