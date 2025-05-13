package workflow

func LoadAgentsRef(wc *WorkflowConfig) error {
	for i := range wc.Agents {
		cfg, err := wc.Agents[i].LoadFileRef(wc.GetCWD())
		if err != nil {
			return err
		}
		if cfg != nil {
			wc.Agents[i] = *cfg
		}
	}
	return nil
}
