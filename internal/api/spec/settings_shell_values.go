package spec

import "github.com/compozy/compozy/internal/api/contract"

func settingsShellSessionSortValues() []string {
	return []string{
		string(contract.SettingsShellSessionSortLastActivity),
		string(contract.SettingsShellSessionSortAttention),
	}
}

func settingsShellSessionScopeValues() []string {
	return []string{
		string(contract.SettingsShellSessionScopeRecent),
		string(contract.SettingsShellSessionScopeAll),
		string(contract.SettingsShellSessionScopeAllWorkspaces),
	}
}
