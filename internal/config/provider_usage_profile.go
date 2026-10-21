package config

// ApplyProviderUsageProfile applies only the validated profile layer to an active
// daemon config snapshot, without reloading unrelated desired machine settings.
func ApplyProviderUsageProfile(home HomePaths, profileName string, cfg *Config) error {
	if err := ValidateResourceProfileName(profileName); err != nil {
		return err
	}
	return applyProfileConfigOverlayFile(profileConfigFile(home, profileName), cfg, RoleFieldSourceProfile)
}
