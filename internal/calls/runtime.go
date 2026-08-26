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
