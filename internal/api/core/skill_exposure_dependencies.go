package core

import "github.com/compozy/compozy/internal/store"

// SkillExposureDependencies projects the optional exposure stores implemented by a shared backend.
type SkillExposureDependencies struct {
	Repository store.SkillExposureRepository
	Events     store.EventSummaryStore
}

// ResolveSkillExposureDependencies keeps HTTP and UDS exposure wiring identical.
func ResolveSkillExposureDependencies(value any) SkillExposureDependencies {
	repository, _ := value.(store.SkillExposureRepository)
	events, _ := value.(store.EventSummaryStore)
	return SkillExposureDependencies{Repository: repository, Events: events}
}
