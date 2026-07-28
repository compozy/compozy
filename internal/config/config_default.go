package config

func defaultConfig() (Config, error) {
	homePaths, err := ResolveHomePaths()
	if err != nil {
		return Config{}, err
	}
	return DefaultWithHome(homePaths), nil
}
