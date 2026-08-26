package cli

func terminalConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"terminal.default_shell":            configSetString,
		"terminal.shell_integration":        configSetBool,
		"terminal.scrollback_bytes":         configSetInt,
		"terminal.detached_ttl":             configSetDuration,
		"terminal.exit_retention":           configSetDuration,
		"terminal.recording":                configSetBool,
		"terminal.recording_retention_days": configSetInt,
		"terminal.max_per_workspace":        configSetInt,
		"terminal.max_per_daemon":           configSetInt,
		"terminal.max_subscribers":          configSetInt,
	}
}
