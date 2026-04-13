package setup

// ExtensionSkillPackSources converts effective extension skills back into declarative skill-pack sources.
func ExtensionSkillPackSources(skills []Skill) []SkillPackSource {
	if len(skills) == 0 {
		return nil
	}

	sources := make([]SkillPackSource, 0, len(skills))
	for i := range skills {
		if skills[i].Origin != AssetOriginExtension {
			continue
		}
		sources = append(sources, SkillPackSource{
			ExtensionName:   skills[i].ExtensionName,
			ExtensionSource: skills[i].ExtensionSource,
			ManifestPath:    skills[i].ManifestPath,
			ResolvedPath:    skills[i].ResolvedPath,
			SourceFS:        skills[i].SourceFS,
			SourceDir:       skills[i].SourceDir,
		})
	}
	return sources
}
