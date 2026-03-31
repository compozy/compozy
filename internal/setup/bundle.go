package setup

import (
	"embed"
	"io/fs"
)

//go:generate go run ./cmd/generatebundle
//go:embed assets/skills/**
var bundledSkills embed.FS

// ListBundledSkills returns the public skills bundled into the looper binary.
func ListBundledSkills() ([]Skill, error) {
	bundle, err := bundledSkillsRoot()
	if err != nil {
		return nil, err
	}
	return ListSkills(bundle)
}

// PreviewBundledSkillInstall resolves the on-disk install plan for bundled skills.
func PreviewBundledSkillInstall(cfg InstallConfig) ([]PreviewItem, error) {
	bundle, err := bundledSkillsRoot()
	if err != nil {
		return nil, err
	}

	cfg.Bundle = bundle
	return Preview(cfg)
}

// InstallBundledSkills materializes bundled public skills for the selected agents.
func InstallBundledSkills(cfg InstallConfig) (*Result, error) {
	bundle, err := bundledSkillsRoot()
	if err != nil {
		return nil, err
	}

	cfg.Bundle = bundle
	return Install(cfg)
}

func bundledSkillsRoot() (fs.FS, error) {
	return fs.Sub(bundledSkills, "assets/skills")
}
