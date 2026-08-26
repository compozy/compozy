package config

const (
	SkillSourceCompozy    = "compozy"
	SkillSourceAgents     = "agents"
	SkillSourceClaude     = "claude"
	agentsGlobalSkillPath = "~/.agents/skills"
)

// SkillSourcePreset describes one curated filesystem convention understood by CompozyOS.
type SkillSourcePreset struct {
	Slug                   string
	Label                  string
	WorkspaceRel           string
	GlobalPath             string
	WorkspaceNativeReaders []string
	GlobalNativeReaders    []string
	AlwaysOn               bool
	DefaultOn              bool
}

// SkillSourcePresets returns the closed preset table in public display order.
func SkillSourcePresets() []SkillSourcePreset {
	return []SkillSourcePreset{
		{
			Slug:                   SkillSourceCompozy,
			Label:                  "Compozy",
			AlwaysOn:               true,
			WorkspaceNativeReaders: []string{},
			GlobalNativeReaders:    []string{},
		},
		{
			Slug:                   SkillSourceAgents,
			Label:                  "Agents",
			WorkspaceRel:           ".agents/skills",
			GlobalPath:             agentsGlobalSkillPath,
			WorkspaceNativeReaders: []string{providerOpenclawKey, providerHermesKey},
			GlobalNativeReaders:    []string{providerOpenclawKey},
			DefaultOn:              true,
		},
		{
			Slug:                   SkillSourceClaude,
			Label:                  "Claude",
			WorkspaceRel:           ".claude/skills",
			GlobalPath:             "~/.claude/skills",
			WorkspaceNativeReaders: []string{providerClaudeKey},
			GlobalNativeReaders:    []string{providerClaudeKey},
		},
	}
}

func configurableSkillSourceSlugs() []string {
	return []string{SkillSourceAgents, SkillSourceClaude}
}

func skillSourcePreset(slug string) (SkillSourcePreset, bool) {
	for _, preset := range SkillSourcePresets() {
		if preset.Slug == slug {
			return preset, true
		}
	}
	return SkillSourcePreset{}, false
}
