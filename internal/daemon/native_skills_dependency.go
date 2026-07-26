package daemon

import skillspkg "github.com/compozy/compozy/internal/skills"

func skillsRegistryAPI(registry *skillspkg.Registry) daemonNativeSkillsRegistry {
	if registry == nil {
		return nil
	}
	return registry
}
