package contract

// SettingsSkillSourceInheritancePayload reports workspace source-key inheritance.
type SettingsSkillSourceInheritancePayload struct {
	Sources       bool `json:"sources"`
	CustomSources bool `json:"custom_sources"`
}

// SettingsSkillSourcePayload describes one configured skill source convention.
type SettingsSkillSourcePayload struct {
	Slug          string                           `json:"slug"`
	Label         string                           `json:"label"`
	Kind          string                           `json:"kind"`
	Enabled       bool                             `json:"enabled"`
	AlwaysOn      bool                             `json:"always_on"`
	Default       bool                             `json:"default,omitempty"`
	WorkspacePath string                           `json:"workspace_path,omitempty"`
	GlobalPath    string                           `json:"global_path,omitempty"`
	Path          string                           `json:"path,omitempty"`
	Roots         []SettingsSkillSourceRootPayload `json:"roots"`
}

// SettingsSkillSourceRootPayload carries daemon-measured diagnostics for one root.
type SettingsSkillSourceRootPayload struct {
	RootID        string                                  `json:"root_id"`
	Path          string                                  `json:"path"`
	Exists        bool                                    `json:"exists"`
	Readable      bool                                    `json:"readable"`
	ScannedCount  *int                                    `json:"scanned_count,omitempty"`
	SkillCount    *int                                    `json:"skill_count,omitempty"`
	Truncated     bool                                    `json:"truncated"`
	SkippedLinks  []SettingsSkillSourceSkippedLinkPayload `json:"skipped_links"`
	Collisions    []SettingsSkillSourceCollisionPayload   `json:"collisions"`
	Verification  SettingsSkillSourceVerificationPayload  `json:"verification"`
	NativeReaders []string                                `json:"native_readers"`
}

// SettingsSkillSourceSkippedLinkPayload explains why a first-level link was excluded.
type SettingsSkillSourceSkippedLinkPayload struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// SettingsSkillSourceCollisionPayload reports a lower-precedence definition.
type SettingsSkillSourceCollisionPayload struct {
	Name          string `json:"name"`
	WinnerRootID  string `json:"winner_root_id"`
	QualifiedForm string `json:"qualified_form"`
}

// SettingsSkillSourceVerificationPayload summarizes verifier outcomes for one root.
type SettingsSkillSourceVerificationPayload struct {
	Blocked int `json:"blocked"`
	Warned  int `json:"warned"`
}
