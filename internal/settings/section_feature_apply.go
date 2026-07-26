package settings

import (
	"errors"
	"fmt"

	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func applySkillsSettings(editor *aghconfig.OverlayEditor, settings aghconfig.SkillsConfig) error {
	updates := []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionSkills), sectionsEnabledKey}, value: settings.Enabled},
		{
			path:  []string{string(SectionSkills), "disabled_skills"},
			value: append([]string(nil), settings.DisabledSkills...),
		},
		{path: []string{string(SectionSkills), "poll_interval"}, value: settings.PollInterval.String()},
		{
			path:  []string{string(SectionSkills), "allowed_marketplace_mcp"},
			value: append([]string(nil), settings.AllowedMarketplaceMCP...),
		},
		{
			path:  []string{string(SectionSkills), "allowed_marketplace_hooks"},
			value: append([]string(nil), settings.AllowedMarketplaceHooks...),
		},
		{
			path:  []string{string(SectionSkills), sectionsMarketplaceKey, "registry"},
			value: settings.Marketplace.Registry,
		},
		{
			path:  []string{string(SectionSkills), sectionsMarketplaceKey, "base_url"},
			value: settings.Marketplace.BaseURL,
		},
	}
	return applyValueUpdates(editor, updates)
}

func applyAutomationSettings(editor *aghconfig.OverlayEditor, settings AutomationSettings) error {
	updates := []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionAutomation), sectionsEnabledKey}, value: settings.Enabled},
		{path: []string{string(SectionAutomation), "timezone"}, value: settings.Timezone},
		{path: []string{string(SectionAutomation), "max_concurrent_jobs"}, value: settings.MaxConcurrentJobs},
		{
			path:  []string{string(SectionAutomation), "default_fire_limit", sectionsMaxKey},
			value: settings.DefaultFireLimit.Max,
		},
		{
			path:  []string{string(SectionAutomation), "default_fire_limit", sectionsWindowKey},
			value: settings.DefaultFireLimit.Window,
		},
	}
	return applyValueUpdates(editor, updates)
}

func applyObservabilitySettings(editor *aghconfig.OverlayEditor, settings aghconfig.ObservabilityConfig) error {
	updates := []struct {
		path  []string
		value any
	}{
		{path: []string{string(SectionObservability), sectionsEnabledKey}, value: settings.Enabled},
		{path: []string{string(SectionObservability), "retention_days"}, value: settings.RetentionDays},
		{path: []string{string(SectionObservability), "max_global_bytes"}, value: settings.MaxGlobalBytes},
		{
			path:  []string{string(SectionObservability), sectionsTranscriptsKey, sectionsEnabledKey},
			value: settings.Transcripts.Enabled,
		},
		{
			path:  []string{string(SectionObservability), sectionsTranscriptsKey, "segment_bytes"},
			value: settings.Transcripts.SegmentBytes,
		},
		{
			path:  []string{string(SectionObservability), sectionsTranscriptsKey, "max_bytes_per_session"},
			value: settings.Transcripts.MaxBytesPerSession,
		},
	}
	return applyValueUpdates(editor, updates)
}

func applyValueUpdates(editor *aghconfig.OverlayEditor, updates []struct {
	path  []string
	value any
}) error {
	for _, update := range updates {
		if err := editor.SetValue(update.path, update.value); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) applySkillsDisabledChanges(current []string, next []string) error {
	if s.skillsRuntime == nil {
		return errors.New("settings: skills runtime is required to apply skills.disabled_skills")
	}

	currentSet := sliceToSet(current)
	nextSet := sliceToSet(next)
	for _, skill := range s.skillsRuntime.List() {
		if skill == nil {
			continue
		}
		name := strings.TrimSpace(skill.Meta.Name)
		if name == "" {
			continue
		}
		_, wasDisabled := currentSet[name]
		_, nowDisabled := nextSet[name]
		if wasDisabled == nowDisabled {
			continue
		}
		if err := s.skillsRuntime.SetEnabled(name, nil, !nowDisabled); err != nil {
			return fmt.Errorf("settings: apply skills.disabled_skills for %q: %w", name, err)
		}
	}
	return nil
}

func (s *service) applyAgentSkillsDisabledChanges(
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	current []string,
	next []string,
) error {
	if s.skillsRuntime == nil {
		return errors.New("settings: skills runtime is required to apply agent skills.disabled_skills")
	}

	currentSet := sliceToSet(current)
	nextSet := sliceToSet(next)
	names := make([]string, 0, len(currentSet)+len(nextSet))
	seen := make(map[string]struct{}, len(currentSet)+len(nextSet))
	for name := range currentSet {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range nextSet {
		if _, ok := seen[name]; ok {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		_, wasDisabled := currentSet[name]
		_, nowDisabled := nextSet[name]
		if wasDisabled == nowDisabled {
			continue
		}
		if err := s.skillsRuntime.SetEnabledForAgent(name, resolved, agentName, !nowDisabled); err != nil {
			return mapSkillsSettingsError(
				fmt.Errorf("settings: apply agent skills.disabled_skills for %q on %q: %w", name, agentName, err),
			)
		}
	}

	return nil
}
