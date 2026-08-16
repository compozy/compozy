package mcp

import (
	"slices"
	"strings"
	"sync"
)

// RuntimeHealthKey identifies one extension-owned MCP server generation.
type RuntimeHealthKey struct {
	InstanceName     string
	WorkspaceID      string
	BundleGeneration string
	ServerName       string
}

func (k RuntimeHealthKey) normalize() RuntimeHealthKey {
	return RuntimeHealthKey{
		InstanceName:     strings.TrimSpace(k.InstanceName),
		WorkspaceID:      strings.TrimSpace(k.WorkspaceID),
		BundleGeneration: strings.TrimSpace(k.BundleGeneration),
		ServerName:       strings.TrimSpace(k.ServerName),
	}
}

func (k RuntimeHealthKey) valid() bool {
	normalized := k.normalize()
	return normalized.InstanceName != "" && normalized.BundleGeneration != "" && normalized.ServerName != ""
}

type runtimeHealthInstanceKey struct {
	name        string
	workspaceID string
}

func (k RuntimeHealthKey) instanceKey() runtimeHealthInstanceKey {
	normalized := k.normalize()
	return runtimeHealthInstanceKey{name: normalized.InstanceName, workspaceID: normalized.WorkspaceID}
}

// RuntimeHealthObservation fences one asynchronous call-site outcome.
type RuntimeHealthObservation struct {
	key      RuntimeHealthKey
	sequence uint64
	epoch    uint64
}

// RuntimeHealthEntry is one redacted live failure.
type RuntimeHealthEntry struct {
	Key      RuntimeHealthKey
	Sequence uint64
	Message  string
}

type runtimeHealthState struct {
	nextSequence     uint64
	appliedSequence  uint64
	unhealthyMessage string
}

// RuntimeHealthRegistry owns restart-local extension MCP availability state.
type RuntimeHealthRegistry struct {
	mu             sync.RWMutex
	states         map[RuntimeHealthKey]runtimeHealthState
	instanceEpochs map[runtimeHealthInstanceKey]uint64
}

// NewRuntimeHealthRegistry creates an empty restart-local registry.
func NewRuntimeHealthRegistry() *RuntimeHealthRegistry {
	return &RuntimeHealthRegistry{
		states:         make(map[RuntimeHealthKey]runtimeHealthState),
		instanceEpochs: make(map[runtimeHealthInstanceKey]uint64),
	}
}

// Begin reserves a monotonic observation sequence before one exchange starts.
func (r *RuntimeHealthRegistry) Begin(key RuntimeHealthKey) RuntimeHealthObservation {
	key = key.normalize()
	if r == nil || !key.valid() {
		return RuntimeHealthObservation{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.states[key]
	state.nextSequence++
	r.states[key] = state
	return RuntimeHealthObservation{
		key:      key,
		sequence: state.nextSequence,
		epoch:    r.instanceEpochs[key.instanceKey()],
	}
}

// RecordFailure records a redacted failure unless a newer outcome or lifecycle epoch won.
func (r *RuntimeHealthRegistry) RecordFailure(observation RuntimeHealthObservation, err error) {
	if r == nil || err == nil || !observation.key.valid() || observation.sequence == 0 {
		return
	}
	message := strings.TrimSpace(redactMCPError(err).Error())
	if message == "" {
		message = "MCP server exchange failed"
	}
	r.apply(observation, message)
}

// RecordSuccess clears a live failure unless a newer outcome or lifecycle epoch won.
func (r *RuntimeHealthRegistry) RecordSuccess(observation RuntimeHealthObservation) {
	if r == nil || !observation.key.valid() || observation.sequence == 0 {
		return
	}
	r.apply(observation, "")
}

func (r *RuntimeHealthRegistry) apply(observation RuntimeHealthObservation, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	instance := observation.key.instanceKey()
	if observation.epoch != r.instanceEpochs[instance] {
		return
	}
	state := r.states[observation.key]
	if observation.sequence < state.appliedSequence {
		return
	}
	state.appliedSequence = observation.sequence
	state.unhealthyMessage = strings.TrimSpace(message)
	r.states[observation.key] = state
}

// Entries returns deterministic live failures for one instance generation.
func (r *RuntimeHealthRegistry) Entries(
	instanceName string,
	workspaceID string,
	bundleGeneration string,
) []RuntimeHealthEntry {
	if r == nil {
		return nil
	}
	name := strings.TrimSpace(instanceName)
	workspace := strings.TrimSpace(workspaceID)
	generation := strings.TrimSpace(bundleGeneration)
	r.mu.RLock()
	entries := make([]RuntimeHealthEntry, 0)
	for key, state := range r.states {
		if key.InstanceName != name || key.WorkspaceID != workspace || key.BundleGeneration != generation ||
			state.unhealthyMessage == "" {
			continue
		}
		entries = append(entries, RuntimeHealthEntry{
			Key: key, Sequence: state.appliedSequence, Message: state.unhealthyMessage,
		})
	}
	r.mu.RUnlock()
	slices.SortFunc(entries, func(left, right RuntimeHealthEntry) int {
		return strings.Compare(left.Key.ServerName, right.Key.ServerName)
	})
	return entries
}

// EvictInstance clears all generations and fences in-flight observations for one instance.
func (r *RuntimeHealthRegistry) EvictInstance(instanceName string, workspaceID string) {
	if r == nil {
		return
	}
	instance := runtimeHealthInstanceKey{
		name: strings.TrimSpace(instanceName), workspaceID: strings.TrimSpace(workspaceID),
	}
	if instance.name == "" {
		return
	}
	r.mu.Lock()
	r.instanceEpochs[instance]++
	for key := range r.states {
		if key.instanceKey() == instance {
			delete(r.states, key)
		}
	}
	r.mu.Unlock()
}
