package cli

func extensionConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"extensions.marketplace.registry":                         configSetString,
		"extensions.marketplace.base_url":                         configSetString,
		"extensions.marketplace.allow_unverified":                 configSetBool,
		"extensions.resources.allowed_kinds":                      configSetStringSlice,
		"extensions.resources.max_scope":                          configSetString,
		"extensions.resources.snapshot_rate_limit.requests":       configSetInt,
		"extensions.resources.snapshot_rate_limit.window":         configSetDuration,
		"extensions.resources.snapshot_rate_limit.queue":          configSetInt,
		"extensions.resources.operator_write_rate_limit.requests": configSetInt,
		"extensions.resources.operator_write_rate_limit.window":   configSetDuration,
		"extensions.resources.operator_write_rate_limit.queue":    configSetInt,
	}
}
