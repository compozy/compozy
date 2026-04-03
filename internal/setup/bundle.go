package setup

import (
	"io/fs"

	"github.com/compozy/compozy/skills"
)

// ListBundledSkills returns the public skills bundled into the compozy binary.
func ListBundledSkills() ([]Skill, error) {
	return ListSkills(skills.FS)
}

// PreviewBundledSkillInstall resolves the on-disk install plan for bundled skills.
func PreviewBundledSkillInstall(cfg InstallConfig) ([]PreviewItem, error) {
	cfg.Bundle = skills.FS
	return Preview(cfg)
}

// InstallBundledSkills materializes bundled public skills for the selected agents.
func InstallBundledSkills(cfg InstallConfig) (*Result, error) {
	cfg.Bundle = skills.FS
	return Install(cfg)
}

// VerifyBundledSkills checks whether bundled public skills are installed and current.
func VerifyBundledSkills(cfg VerifyConfig) (VerifyResult, error) {
	cfg.Bundle = skills.FS
	return Verify(cfg)
}

// bundledSkillsRoot returns the embedded skill filesystem for tests.
func bundledSkillsRoot() (fs.FS, error) {
	return skills.FS, nil
}
