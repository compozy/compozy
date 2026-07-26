package bridges

import bridgecontract "github.com/compozy/agh/internal/bridges/contract"

func progressConfigToContract(config ProgressConfig) bridgecontract.ProgressConfig {
	return bridgecontract.ProgressConfig{
		ToolProgress: bridgecontract.ProgressMode(config.ToolProgress),
		Grouping:     bridgecontract.ProgressGrouping(config.Grouping),
		Typing:       config.Typing,
		Reactions:    config.Reactions,
		PreviewLimit: config.PreviewLimit,
		EditInterval: config.EditInterval,
	}
}

func progressConfigFromContract(config bridgecontract.ProgressConfig) ProgressConfig {
	return ProgressConfig{
		ToolProgress: ProgressMode(config.ToolProgress),
		Grouping:     ProgressGrouping(config.Grouping),
		Typing:       config.Typing,
		Reactions:    config.Reactions,
		PreviewLimit: config.PreviewLimit,
		EditInterval: config.EditInterval,
	}
}
