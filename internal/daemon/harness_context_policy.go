package daemon

func (r *HarnessContextResolver) resolveSections(sessionCtx HarnessSessionContext) []HarnessPromptSection {
	sections := make([]HarnessPromptSection, 0, 6)
	if r.runtime.RuntimeIdentityPromptSectionEnabled {
		sections = append(sections, HarnessPromptSectionRuntimeIdentity)
	}
	if r.runtime.SituationPromptSectionEnabled {
		sections = append(sections, HarnessPromptSectionSituation)
	}
	if r.runtime.MemoryPromptSectionEnabled {
		sections = append(sections, HarnessPromptSectionMemory)
	}
	if r.runtime.SkillsPromptSectionEnabled {
		sections = append(sections, HarnessPromptSectionSkills)
	}
	if r.runtime.ToolsPromptSectionEnabled {
		sections = append(sections, HarnessPromptSectionTools)
	}
	if sessionCtx.NetworkLive {
		sections = append(sections, HarnessPromptSectionNetwork)
	}
	return sections
}

func (r *HarnessContextResolver) resolveAugmenters(
	surface ResolutionSurface,
	turnCtx HarnessTurnContext,
) []HarnessAugmenter {
	if surface != ResolutionSurfaceTurn {
		return nil
	}
	augmenters := make([]HarnessAugmenter, 0, 3)
	if r.runtime.SkillsAugmenter {
		augmenters = append(augmenters, HarnessAugmenterSkills)
	}
	if turnCtx.Origin == TurnOriginNetwork {
		return augmenters
	}
	if turnCtx.Origin != TurnOriginUser {
		return augmenters
	}
	if r.runtime.SituationAugmenter {
		augmenters = append(augmenters, HarnessAugmenterSituation)
	}
	if r.runtime.DurableMemoryAugmenter {
		augmenters = append(augmenters, HarnessAugmenterDurableMemory)
	}
	return augmenters
}

func (r *HarnessContextResolver) resolveReentry(turnCtx HarnessTurnContext) ReentryMode {
	if turnCtx.Origin == TurnOriginSynthetic {
		return ReentryModeSynthetic
	}
	return ReentryModeNone
}

func (r *HarnessContextResolver) resolveDetachedRunMode(
	sessionCtx HarnessSessionContext,
	turnCtx HarnessTurnContext,
) DetachedRunMode {
	if !r.runtime.DetachedTaskRuntimeEnabled {
		return DetachedRunModeNone
	}
	if turnCtx.Detached == nil {
		return DetachedRunModeNone
	}
	if sessionCtx.SessionClass == SessionClassSystem {
		return DetachedRunModeTaskRuntime
	}
	return DetachedRunModeNone
}
