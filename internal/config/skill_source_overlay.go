package config

// SkillSourceOverridePresence reports whether one overlay explicitly owns each source list.
type SkillSourceOverridePresence struct {
	Sources       bool
	CustomSources bool
}

// ReadSkillSourceOverridePresence inspects one config overlay without applying inherited values.
func ReadSkillSourceOverridePresence(path string) (SkillSourceOverridePresence, error) {
	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		return SkillSourceOverridePresence{}, err
	}
	return SkillSourceOverridePresence{
		Sources:       overlay.Skills.Sources != nil,
		CustomSources: overlay.Skills.CustomSources != nil,
	}, nil
}
