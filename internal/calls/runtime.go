package calls

import (
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

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
) PermissionAtoms {
	parent = store.NormalizeSessionPermissionPolicy(parent)
	request := requested.Policy()
	return PermissionAtoms{
		Tools:           inheritPermissionCategory(parent.Tools, request.Tools),
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
		{"tools", parent.Tools, child.Tools}, {"skills", parent.Skills, child.Skills},
		{"mcp_servers", parent.MCPServers, child.MCPServers},
		{"workspace_paths", parent.WorkspacePaths, child.WorkspacePaths},
		{"network_channels", parent.NetworkChannels, child.NetworkChannels},
		{"sandbox_profiles", parent.SandboxProfiles, child.SandboxProfiles},
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
