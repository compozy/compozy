package calls

import (
	"slices"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

const (
	boundChildToolAgentCall    = "compozy__agent_call"
	boundChildToolAgentMessage = "compozy__agent_message"
	boundChildToolCallAwait    = "compozy__call_await"
	boundChildToolCallCancel   = "compozy__call_cancel"
	boundChildToolCallResult   = "compozy__call_result"
	boundChildToolCallReturn   = "compozy__call_return"
)

func boundChildBaseTools() []string {
	return []string{boundChildToolAgentMessage, boundChildToolCallReturn}
}

func boundChildDelegationTools() []string {
	return []string{
		boundChildToolAgentCall,
		boundChildToolCallAwait,
		boundChildToolCallCancel,
		boundChildToolCallResult,
	}
}

// RuntimeSpec selects the provider runtime for a child activation.
type RuntimeSpec struct {
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speed.Speed
}

// Normalize trims authored runtime identifiers.
func (r RuntimeSpec) Normalize() RuntimeSpec {
	r.Provider = strings.TrimSpace(r.Provider)
	r.Model = strings.TrimSpace(r.Model)
	r.ReasoningEffort = strings.TrimSpace(r.ReasoningEffort)
	return r
}

// PermissionAtoms contains the caller-requested permission narrowing.
type PermissionAtoms struct {
	Tools           []string
	Skills          []string
	MCPServers      []string
	WorkspacePaths  []string
	NetworkChannels []string
	SandboxProfiles []string
}

// Policy converts authored atoms into the canonical normalized policy.
func (p PermissionAtoms) Policy() store.SessionPermissionPolicy {
	return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{
		Tools: p.Tools, Skills: p.Skills,
		MCPServers:      p.MCPServers,
		WorkspacePaths:  p.WorkspacePaths,
		NetworkChannels: p.NetworkChannels,
		SandboxProfiles: p.SandboxProfiles,
	})
}

func resolvePermissionNarrowing(
	parent store.SessionPermissionPolicy,
	requested PermissionAtoms,
	remainingDepth int,
) PermissionAtoms {
	parent = store.NormalizeSessionPermissionPolicy(parent)
	request := requested.Policy()
	tools := request.Tools
	if len(tools) == 0 {
		tools = boundChildBaseTools()
		if remainingDepth > 0 {
			tools = append(tools, boundChildDelegationTools()...)
		}
	} else if !slices.Contains(tools, boundChildToolCallReturn) {
		tools = append(tools, boundChildToolCallReturn)
	}
	if remainingDepth <= 0 {
		tools = slices.DeleteFunc(tools, func(tool string) bool {
			return slices.Contains(boundChildDelegationTools(), strings.TrimSpace(tool))
		})
	}
	return PermissionAtoms{
		Tools:           store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{Tools: tools}).Tools,
		Skills:          inheritPermissionCategory(parent.Skills, request.Skills),
		MCPServers:      inheritPermissionCategory(parent.MCPServers, request.MCPServers),
		WorkspacePaths:  inheritPermissionCategory(parent.WorkspacePaths, request.WorkspacePaths),
		NetworkChannels: inheritPermissionCategory(parent.NetworkChannels, request.NetworkChannels),
		SandboxProfiles: inheritPermissionCategory(parent.SandboxProfiles, request.SandboxProfiles),
	}
}

func inheritPermissionCategory(parent []string, requested []string) []string {
	if len(requested) > 0 {
		return append([]string(nil), requested...)
	}
	return append([]string(nil), parent...)
}

func wideningPermissionAtoms(parent store.SessionPermissionPolicy, child store.SessionPermissionPolicy) []string {
	types := []struct {
		name   string
		parent []string
		child  []string
	}{
		{permissionKindTools, parent.Tools, child.Tools}, {permissionKindSkills, parent.Skills, child.Skills},
		{permissionKindMCPServers, parent.MCPServers, child.MCPServers},
		{permissionKindWorkspacePaths, parent.WorkspacePaths, child.WorkspacePaths},
		{permissionKindNetwork, parent.NetworkChannels, child.NetworkChannels},
		{permissionKindSandbox, parent.SandboxProfiles, child.SandboxProfiles},
	}
	widening := make([]string, 0)
	for _, permissionType := range types {
		allowed := make(map[string]struct{}, len(permissionType.parent))
		for _, atom := range permissionType.parent {
			allowed[strings.TrimSpace(atom)] = struct{}{}
		}
		for _, atom := range permissionType.child {
			trimmed := strings.TrimSpace(atom)
			if _, ok := allowed[trimmed]; !ok {
				widening = append(widening, permissionType.name+":"+trimmed)
			}
		}
	}
	sort.Strings(widening)
	return widening
}
