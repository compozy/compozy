package mcp

import (
	"maps"
	"slices"
	"strings"
	"sync"
)

// RuntimeHealthKey identifies one extension-owned MCP server generation.
type RuntimeHealthKey struct {
	ResourceID           string
	InstanceName         string
	WorkspaceID          string
	BundleGeneration     string
	ServerName           string
	resolvedEpoch        uint64
	resolvedPackageEpoch uint64
	resolved             bool
}

// normalize preserves lifecycle fences while canonicalizing server identity.
func (k RuntimeHealthKey) normalize() RuntimeHealthKey {
	return RuntimeHealthKey{
		ResourceID:       strings.TrimSpace(k.ResourceID),
		InstanceName:     strings.TrimSpace(k.InstanceName),
		WorkspaceID:      strings.TrimSpace(k.WorkspaceID),
		BundleGeneration: strings.TrimSpace(k.BundleGeneration),
		ServerName:       strings.TrimSpace(k.ServerName),
		resolvedEpoch:    k.resolvedEpoch, resolvedPackageEpoch: k.resolvedPackageEpoch, resolved: k.resolved,
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

// instanceKey groups all server generations belonging to the same workspace instance.
func (k RuntimeHealthKey) instanceKey() runtimeHealthInstanceKey {
	normalized := k.normalize()
	return runtimeHealthInstanceKey{name: normalized.InstanceName, workspaceID: normalized.WorkspaceID}
}

// RuntimeHealthObservation fences one asynchronous call-site outcome.
type RuntimeHealthObservation struct {
	key          RuntimeHealthKey
	sequence     uint64
	epoch        uint64
	packageEpoch uint64
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
	packageEpochs  map[string]uint64
}

// NewRuntimeHealthRegistry creates an empty restart-local registry.
func NewRuntimeHealthRegistry() *RuntimeHealthRegistry {
	return &RuntimeHealthRegistry{
		states:         make(map[RuntimeHealthKey]runtimeHealthState),
		instanceEpochs: make(map[runtimeHealthInstanceKey]uint64),
		packageEpochs:  make(map[string]uint64),
	}
}

// ResolveKey captures lifecycle ownership before a resolved server can start an exchange.
func (r *RuntimeHealthRegistry) ResolveKey(key RuntimeHealthKey) RuntimeHealthKey {
	key = key.normalize()
	if r == nil || key.InstanceName == "" {
		return key
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	key.resolved = true
	key.resolvedEpoch = r.instanceEpochs[key.instanceKey()]
	key.resolvedPackageEpoch = r.packageEpochs[key.InstanceName]
	return key
}

// stateKey removes the resolution fence from the stable server identity.
func (k RuntimeHealthKey) stateKey() RuntimeHealthKey {
	k = k.normalize()
	k.resolved, k.resolvedEpoch, k.resolvedPackageEpoch = false, 0, 0
	return k
}

// Begin reserves a monotonic observation sequence before one exchange starts.
func (r *RuntimeHealthRegistry) Begin(key RuntimeHealthKey) RuntimeHealthObservation {
	key = key.normalize()
	if r == nil || !key.valid() {
		return RuntimeHealthObservation{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if key.resolved && (key.resolvedEpoch != r.instanceEpochs[key.instanceKey()] ||
		key.resolvedPackageEpoch != r.packageEpochs[key.InstanceName]) {
		return RuntimeHealthObservation{}
	}
	key = key.stateKey()
	state := r.states[key]
	state.nextSequence++
	r.states[key] = state
	return RuntimeHealthObservation{
		key:          key,
		sequence:     state.nextSequence,
		epoch:        r.instanceEpochs[key.instanceKey()],
		packageEpoch: r.packageEpochs[key.InstanceName],
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

// apply rejects outcomes from retired lifecycles or older exchanges before changing live health.
func (r *RuntimeHealthRegistry) apply(observation RuntimeHealthObservation, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	instance := observation.key.instanceKey()
	if observation.epoch != r.instanceEpochs[instance] ||
		observation.packageEpoch != r.packageEpochs[observation.key.InstanceName] {
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

// Ready reports the latest completed exchange without launching or probing a server.
func (r *RuntimeHealthRegistry) Ready(key RuntimeHealthKey) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := r.states[key.stateKey()]
	return state.appliedSequence != 0 && state.unhealthyMessage == ""
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
		if order := strings.Compare(left.Key.ServerName, right.Key.ServerName); order != 0 {
			return order
		}
		return strings.Compare(left.Key.ResourceID, right.Key.ResourceID)
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
	maps.DeleteFunc(r.states, func(key RuntimeHealthKey, _ runtimeHealthState) bool {
		return key.instanceKey() == instance
	})
	r.mu.Unlock()
}

// EvictPackage clears every workspace and fences resolved and in-flight calls after package retirement.
func (r *RuntimeHealthRegistry) EvictPackage(name string) {
	if r == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.packageEpochs[name]++
	maps.DeleteFunc(r.states, func(key RuntimeHealthKey, _ runtimeHealthState) bool {
		return key.InstanceName == name
	})
}
