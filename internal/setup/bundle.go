package setup

import (
	"fmt"
	"io/fs"

	"github.com/compozy/compozy/agents"
	"github.com/compozy/compozy/skills"
)

var installBundledReusableAgents = InstallBundledReusableAgents

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

// InstallBundledSetupAssets materializes bundled skills and bundled council reusable agents.
func InstallBundledSetupAssets(cfg InstallConfig) (*Result, error) {
	cfg.Bundle = skills.FS
	result, err := Install(cfg)
	if err != nil {
		return nil, err
	}

	successes, failures, err := installBundledReusableAgents(cfg.ResolverOptions)
	if err != nil {
		return result, fmt.Errorf("install bundled reusable agents: %w", err)
	}
	result.ReusableAgentsSuccessful = append(result.ReusableAgentsSuccessful, successes...)
	result.ReusableAgentsFailed = append(result.ReusableAgentsFailed, failures...)
	return result, nil
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

// bundledReusableAgentsRoot returns the embedded reusable-agent filesystem for tests.
func bundledReusableAgentsRoot() (fs.FS, error) {
	return agents.FS, nil
}
