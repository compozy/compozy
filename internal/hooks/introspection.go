package hooks

import "time"

const (
	introspectionAuthoredContextObservationPatchValue = "AuthoredContextObservationPatch"
	introspectionAutomationFirePatchValue             = "AutomationFirePatch"
	introspectionAutomationObservationPatchValue      = "AutomationObservationPatch"
	introspectionCoordinatorObservationPatchValue     = "CoordinatorObservationPatch"
	introspectionLoopObservationPatchValue            = "LoopObservationPatch"
	introspectionSpawnObservationPatchValue           = "SpawnObservationPatch"
	introspectionTaskObservationPatchValue            = "TaskObservationPatch"
	introspectionTaskRunObservationPatchValue         = "TaskRunObservationPatch"
)

// CatalogFilter narrows the resolved hook catalog for one workspace/agent view.
type CatalogFilter struct {
	WorkspaceID   string
	WorkspaceRoot string
	AgentName     string
	Event         HookEvent
	Source        *HookSource
	Mode          HookMode
}

// CatalogEntry describes one resolved hook in pipeline order.
type CatalogEntry struct {
	Order        int
	Name         string
	Event        HookEvent
	Source       HookSource
	SkillSource  HookSkillSource
	Mode         HookMode
	Required     bool
	Priority     int32
	Timeout      time.Duration
	ExecutorKind HookExecutorKind
	Matcher      HookMatcher
	Metadata     map[string]string
}

// EventFilter narrows the supported hook taxonomy for introspection APIs.
type EventFilter struct {
	Family   HookEventFamily
	SyncOnly bool
}

// EventDescriptor describes one supported hook event for introspection APIs.
type EventDescriptor struct {
	Event         HookEvent
	Family        HookEventFamily
	SyncEligible  bool
	PayloadSchema string
	PatchSchema   string
}

// Catalog returns the currently resolved hook catalog in deterministic pipeline order.
func (h *Hooks) Catalog(filter CatalogFilter) ([]CatalogEntry, error) {
	if h == nil {
		return nil, nil
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := make([]CatalogEntry, 0)
	for _, event := range AllHookEvents() {
		order := 0
		for _, hook := range h.snapshot[event] {
			if !catalogHookMatchesFilter(hook, filter) {
				continue
			}
			executorKind := HookExecutorKind("")
			if hook.Executor != nil {
				executorKind = hook.Executor.Kind()
			}
			order++
			entries = append(entries, CatalogEntry{
				Order:        order,
				Name:         hook.Name,
				Event:        hook.Event,
				Source:       hook.Source,
				SkillSource:  hook.Decl.SkillSource,
				Mode:         hook.Mode,
				Required:     hook.Required,
				Priority:     hook.Priority,
				Timeout:      hook.Timeout,
				ExecutorKind: executorKind,
				Matcher:      cloneHookMatcher(hook.Matcher),
				Metadata:     cloneStringMap(hook.Metadata),
			})
		}
	}

	return entries, nil
}

// AllEventDescriptors returns the hook taxonomy metadata in deterministic order.
func AllEventDescriptors() []EventDescriptor {
	return FilterEventDescriptors(EventFilter{})
}

// FilterEventDescriptors returns the hook taxonomy metadata in deterministic order.
func FilterEventDescriptors(filter EventFilter) []EventDescriptor {
	descriptors := make([]EventDescriptor, 0, len(allHookEvents))
	for _, event := range AllHookEvents() {
		if descriptor, ok := hookEventDescriptors[event]; ok {
			if filter.Family != "" && descriptor.Family != filter.Family {
				continue
			}
			if filter.SyncOnly && !descriptor.SyncEligible {
				continue
			}
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors
}

func catalogHookMatchesFilter(hook *ResolvedHook, filter CatalogFilter) bool {
	if hook == nil {
		return false
	}
	if filter.Event != "" && hook.Event != filter.Event {
		return false
	}
	if filter.Source != nil && hook.Source != *filter.Source {
		return false
	}
	if filter.Mode != "" && hook.Mode != filter.Mode {
		return false
	}
	if !catalogStringMatches(filter.AgentName, hook.Matcher.AgentName) {
		return false
	}
	if !catalogStringMatches(filter.WorkspaceID, hook.Matcher.WorkspaceID) {
		return false
	}
	if !catalogStringMatches(filter.WorkspaceRoot, hook.Matcher.WorkspaceRoot) {
		return false
	}
	return true
}

func catalogStringMatches(filter string, value string) bool {
	if filter == "" || value == "" {
		return true
	}
	return matchStringField(value, filter)
}

func cloneHookMatcher(src HookMatcher) HookMatcher {
	cloned := src
	if src.ToolReadOnly != nil {
		value := *src.ToolReadOnly
		cloned.ToolReadOnly = &value
	}
	if src.NetworkMatcher != nil {
		value := *src.NetworkMatcher
		cloned.NetworkMatcher = &value
	}
	if src.CompactionMatcher != nil {
		value := *src.CompactionMatcher
		cloned.CompactionMatcher = &value
	}
	if src.Autonomy != nil {
		value := *src.Autonomy
		cloned.Autonomy = &value
	}
	return cloned
}
