package workflow

func LoadToolsRef(wc *Config) error {
	for i := range wc.Tools {
		cfg, err := wc.Tools[i].LoadFileRef(wc.GetCWD())
		if err != nil {
			return err
		}
		if cfg != nil {
			wc.Tools[i] = *cfg
		}
	}
	return nil
}
