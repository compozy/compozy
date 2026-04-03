package setup

import "io/fs"

// InstallMode determines how bundled skills are materialized for agents.
type InstallMode string

const (
	// InstallModeSymlink installs a canonical copy and symlinks agent paths to it.
	InstallModeSymlink InstallMode = "symlink"
	// InstallModeCopy copies the bundled skill into each target agent path.
	InstallModeCopy InstallMode = "copy"
)

// InstallScope identifies whether a skill installation is project-local or global.
type InstallScope string

const (
	// InstallScopeUnknown indicates that no existing installation scope could be resolved.
	InstallScopeUnknown InstallScope = "unknown"
	// InstallScopeProject identifies a project-local installation.
	InstallScopeProject InstallScope = "project"
	// InstallScopeGlobal identifies a global installation.
	InstallScopeGlobal InstallScope = "global"
)

// Skill describes one bundled skill available for installation.
type Skill struct {
	Name        string
	Description string
	Directory   string
}

// Agent describes one supported agent/editor destination.
type Agent struct {
	Name           string
	DisplayName    string
	ProjectRootDir string
	GlobalRootDir  string
	Universal      bool
	Detected       bool
}

// ResolverOptions configures environment-sensitive path resolution.
type ResolverOptions struct {
	CWD             string
	HomeDir         string
	XDGConfigHome   string
	CodeXHome       string
	ClaudeConfigDir string
}

// InstallConfig describes one bundled-skill installation run.
type InstallConfig struct {
	Bundle fs.FS

	ResolverOptions

	SkillNames []string
	AgentNames []string
	Global     bool
	Mode       InstallMode
}

// VerifyState describes the verification status for one installed skill.
type VerifyState string

const (
	// VerifyStateCurrent indicates the installed skill matches the bundled version.
	VerifyStateCurrent VerifyState = "current"
	// VerifyStateMissing indicates the installed skill is missing from the selected scope.
	VerifyStateMissing VerifyState = "missing"
	// VerifyStateDrifted indicates the installed skill differs from the bundled version.
	VerifyStateDrifted VerifyState = "drifted"
)

// VerifyConfig describes one bundled-skill verification run.
type VerifyConfig struct {
	Bundle fs.FS

	ResolverOptions

	AgentName  string
	SkillNames []string
}

// SkillDrift describes how an installed skill differs from the bundled version.
type SkillDrift struct {
	MissingFiles []string
	ExtraFiles   []string
	ChangedFiles []string
	Reason       string
}

// VerifiedSkill captures the verification result for one skill.
type VerifiedSkill struct {
	Skill         Skill
	CanonicalPath string
	TargetPath    string
	ResolvedPath  string
	State         VerifyState
	Drift         SkillDrift
}

// VerifyResult summarizes one bundled-skill verification run.
type VerifyResult struct {
	Agent  Agent
	Scope  InstallScope
	Mode   InstallMode
	Skills []VerifiedSkill
}

// MissingSkillNames returns every missing skill name in sorted order.
func (r VerifyResult) MissingSkillNames() []string {
	names := make([]string, 0, len(r.Skills))
	for i := range r.Skills {
		skill := &r.Skills[i]
		if skill.State != VerifyStateMissing {
			continue
		}
		names = append(names, skill.Skill.Name)
	}
	return names
}

// DriftedSkillNames returns every drifted skill name in sorted order.
func (r VerifyResult) DriftedSkillNames() []string {
	names := make([]string, 0, len(r.Skills))
	for i := range r.Skills {
		skill := &r.Skills[i]
		if skill.State != VerifyStateDrifted {
			continue
		}
		names = append(names, skill.Skill.Name)
	}
	return names
}

// HasMissing reports whether any skill is missing.
func (r VerifyResult) HasMissing() bool {
	for i := range r.Skills {
		if r.Skills[i].State == VerifyStateMissing {
			return true
		}
	}
	return false
}

// HasDrift reports whether any installed skill differs from the bundled version.
func (r VerifyResult) HasDrift() bool {
	for i := range r.Skills {
		if r.Skills[i].State == VerifyStateDrifted {
			return true
		}
	}
	return false
}

// PreviewItem describes the on-disk plan for one skill/agent install pair.
type PreviewItem struct {
	Skill         Skill
	Agent         Agent
	CanonicalPath string
	TargetPath    string
	WillOverwrite bool
}

// SuccessItem captures one successful installation mapping.
type SuccessItem struct {
	Skill         Skill
	Agent         Agent
	Path          string
	CanonicalPath string
	Mode          InstallMode
	SymlinkFailed bool
}

// FailureItem captures one failed installation mapping.
type FailureItem struct {
	Skill Skill
	Agent Agent
	Path  string
	Mode  InstallMode
	Error string
}

// Result summarizes one bundled-skill installation run.
type Result struct {
	Global     bool
	Mode       InstallMode
	Successful []SuccessItem
	Failed     []FailureItem
}
