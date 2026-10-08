package cli

import compozyconfig "github.com/compozy/compozy/internal/config"

func roleConfigSetPathKinds() map[string]configSetValueKind {
	result := make(map[string]configSetValueKind)
	for path, kind := range compozyconfig.RoleMutableConfigKinds() {
		result[path] = configSetKindFromToolKind(kind)
	}
	return result
}

func configSetKindFromToolKind(kind compozyconfig.ValueKind) configSetValueKind {
	switch kind {
	case compozyconfig.ConfigValueString:
		return configSetString
	case compozyconfig.ConfigValueBool:
		return configSetBool
	case compozyconfig.ConfigValueInt:
		return configSetInt
	case compozyconfig.ConfigValueInt64:
		return configSetInt64
	case compozyconfig.ConfigValueUint64:
		return configSetUint64
	case compozyconfig.ConfigValueFloat:
		return configSetFloat
	case compozyconfig.ConfigValueDuration:
		return configSetDuration
	case compozyconfig.ConfigValueStringSlice:
		return configSetStringSlice
	case compozyconfig.ConfigValueTable:
		return configSetTable
	case compozyconfig.ConfigValueACPOptions:
		return configSetACPOptions
	default:
		panic("cli: unsupported role config value kind")
	}
}
