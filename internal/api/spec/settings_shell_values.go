package spec

import "github.com/compozy/compozy/internal/api/contract"

func settingsShellSessionSortValues() []string {
	return []string{
		string(contract.SettingsShellSessionSortCreated),
		string(contract.SettingsShellSessionSortName),
		string(contract.SettingsShellSessionSortLastActivity),
		string(contract.SettingsShellSessionSortAttention),
		string(contract.SettingsShellSessionSortManual),
	}
}

func settingsShellSessionScopeValues() []string {
	return []string{
		string(contract.SettingsShellSessionScopeWorkspace),
		string(contract.SettingsShellSessionScopeAllWorkspaces),
	}
}

func settingsShellTextSizeValues() []string {
	return []string{
		string(contract.SettingsShellTextSizeSmall),
		string(contract.SettingsShellTextSizeDefault),
		string(contract.SettingsShellTextSizeLarge),
		string(contract.SettingsShellTextSizeLarger),
	}
}

func settingsShellLetterSpacingValues() []string {
	return []string{
		string(contract.SettingsShellLetterSpacingTight),
		string(contract.SettingsShellLetterSpacingDefault),
		string(contract.SettingsShellLetterSpacingRelaxed),
		string(contract.SettingsShellLetterSpacingWide),
	}
}
