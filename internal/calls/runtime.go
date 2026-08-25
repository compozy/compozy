package calls

import (
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

type RuntimeSpec struct {
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speed.Speed
}

func (r RuntimeSpec) Normalize() RuntimeSpec {
	r.Provider = strings.TrimSpace(r.Provider)
	r.Model = strings.TrimSpace(r.Model)
	r.ReasoningEffort = strings.TrimSpace(r.ReasoningEffort)
	return r
}

type PermissionAtoms struct {
	Tools           []string
	Skills          []string
	MCPServers      []string
	WorkspacePaths  []string
	NetworkChannels []string
	SandboxProfiles []string
}

func (p PermissionAtoms) Policy() store.SessionPermissionPolicy {
	return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{
		Tools: append([]string(nil), p.Tools...), Skills: append([]string(nil), p.Skills...),
		MCPServers:      append([]string(nil), p.MCPServers...),
		WorkspacePaths:  append([]string(nil), p.WorkspacePaths...),
		NetworkChannels: append([]string(nil), p.NetworkChannels...),
		SandboxProfiles: append([]string(nil), p.SandboxProfiles...),
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
