// Package dsl defines the agh.loop/v1 authoring document.
package dsl

// NodeClass is the closed node class vocabulary.
type NodeClass string

const (
	// NodeClassAction is open and resolves against reserved kinds or ToolIDs.
	NodeClassAction NodeClass = "action"
	// NodeClassControl is a closed statically-verifiable class.
	NodeClassControl NodeClass = "control"
	// NodeClassSource is a closed statically-verifiable class.
	NodeClassSource NodeClass = "source"
)

// ActionKind is open except for reserved first-party kinds.
type ActionKind string

const (
	// ActionRunAgent executes an agent turn.
	ActionRunAgent ActionKind = "run-agent"
	// ActionRunLoop starts another loop definition.
	ActionRunLoop ActionKind = "run-loop"
	// ActionTransform reshapes data in daemon.
	ActionTransform ActionKind = "transform"
	// ActionGoal advances a durable convergence cycle.
	ActionGoal ActionKind = "goal"
)

// ReservedActionKinds returns the reserved first-party action kinds.
func ReservedActionKinds() []ActionKind {
	return []ActionKind{ActionRunAgent, ActionRunLoop, ActionTransform, ActionGoal}
}

// IsReservedActionKind reports whether kind is first-party and not a ToolID.
func IsReservedActionKind(kind string) bool {
	switch ActionKind(kind) {
	case ActionRunAgent, ActionRunLoop, ActionTransform, ActionGoal:
		return true
	default:
		return false
	}
}

// ControlKind is the closed control-node vocabulary.
type ControlKind string

const (
	// ControlFanOut materializes a finite collection into branches.
	ControlFanOut ControlKind = "fan-out"
	// ControlCollect joins fan-out branches.
	ControlCollect ControlKind = "collect"
	// ControlBranch routes by CEL condition.
	ControlBranch ControlKind = "branch"
	// ControlGate evaluates criteria and routing policy.
	ControlGate ControlKind = "gate"
	// ControlSubLoop embeds an inline nested loop body.
	ControlSubLoop ControlKind = "sub-loop"
)

// IsKnownControlKind validates the closed control enum.
func IsKnownControlKind(kind string) bool {
	switch ControlKind(kind) {
	case ControlFanOut, ControlCollect, ControlBranch, ControlGate, ControlSubLoop:
		return true
	default:
		return false
	}
}

// SourceKind is the closed source-node vocabulary.
type SourceKind string

const (
	// SourceInput exposes a declared input into the graph.
	SourceInput SourceKind = "input"
	// SourceFileImport imports workspace files.
	SourceFileImport SourceKind = "file-import"
	// SourceWatchSource polls an extension watch source.
	SourceWatchSource SourceKind = "watch-source"
	// SourceWatchEvents subscribes to committed internal AGH events.
	SourceWatchEvents SourceKind = "watch-events"
)

// IsKnownSourceKind validates the closed source enum.
func IsKnownSourceKind(kind string) bool {
	switch SourceKind(kind) {
	case SourceInput, SourceFileImport, SourceWatchSource, SourceWatchEvents:
		return true
	default:
		return false
	}
}
