package cli

import aghconfig "github.com/compozy/agh/internal/config"

func roleConfigSetPathKinds() map[string]configSetValueKind {
	result := make(map[string]configSetValueKind)
	for path, kind := range aghconfig.RoleMutableConfigKinds() {
		result[path] = configSetKindFromToolKind(kind)
	}
	return result
}

func configSetKindFromToolKind(kind aghconfig.ValueKind) configSetValueKind {
	switch kind {
	case aghconfig.ConfigValueString:
		return configSetString
	case aghconfig.ConfigValueBool:
		return configSetBool
	case aghconfig.ConfigValueInt:
		return configSetInt
	case aghconfig.ConfigValueInt64:
		return configSetInt64
	case aghconfig.ConfigValueFloat:
		return configSetFloat
	case aghconfig.ConfigValueDuration:
		return configSetDuration
	case aghconfig.ConfigValueStringSlice:
		return configSetStringSlice
	case aghconfig.ConfigValueTable:
		return configSetTable
	default:
		panic("cli: unsupported role config value kind")
	}
}
