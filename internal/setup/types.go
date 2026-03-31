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
