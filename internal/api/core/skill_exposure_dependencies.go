package core

import "github.com/compozy/compozy/internal/store"

// SkillExposureDependencies projects the optional exposure stores implemented by a shared backend.
type SkillExposureDependencies struct {
	Repository store.SkillExposureRepository
	Events     store.EventSummaryStore
}

// ResolveSkillExposureDependencies keeps HTTP and UDS exposure wiring identical.
func ResolveSkillExposureDependencies(value any) SkillExposureDependencies {
	dependencies := SkillExposureDependencies{}
	if repository, ok := value.(store.SkillExposureRepository); ok {
		dependencies.Repository = repository
	}
	if events, ok := value.(store.EventSummaryStore); ok {
		dependencies.Events = events
	}
	return dependencies
}
