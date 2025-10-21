package config

import (
	"reflect"
	"strings"
	"sync"
)

// EnvMapping represents a mapping between environment variable and config path
type EnvMapping struct {
	EnvVar     string
	ConfigPath string
}

var (
	cachedMappings []EnvMapping
	mappingsOnce   sync.Once
)

// GenerateEnvMappings generates environment variable mappings from config struct tags
func GenerateEnvMappings() []EnvMapping {
	mappingsOnce.Do(func() {
		cfg := &Config{}
		cachedMappings = extractMappings(reflect.TypeOf(cfg).Elem(), "")
	})
	return cachedMappings
}

// extractMappings recursively extracts env mappings from struct fields
func extractMappings(t reflect.Type, prefix string) []EnvMapping {
	var mappings []EnvMapping
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		koanfTag := field.Tag.Get("koanf")
		if koanfTag == "" || koanfTag == "-" {
			continue
		}

		configPath := koanfTag
		if prefix != "" {
			configPath = prefix + "." + koanfTag
		}

		envTag := field.Tag.Get("env")
		if envTag != "" && envTag != "-" {
			mappings = append(mappings, EnvMapping{
				EnvVar:     envTag,
				ConfigPath: configPath,
			})
		}

		if field.Type.Kind() == reflect.Struct {
			if field.Type.PkgPath() == "time" {
				continue
			}
			submappings := extractMappings(field.Type, configPath)
			mappings = append(mappings, submappings...)
		}
	}
	return mappings
}

// GenerateEnvToConfigMap generates a map from env var to config path
func GenerateEnvToConfigMap() map[string]string {
	mappings := GenerateEnvMappings()
	result := make(map[string]string, len(mappings))
	for _, m := range mappings {
		result[m.EnvVar] = m.ConfigPath
	}
	return result
}

// GetEnvVarForConfigPath returns the environment variable for a given config path
func GetEnvVarForConfigPath(configPath string) string {
	mappings := GenerateEnvMappings()
	for _, m := range mappings {
		if m.ConfigPath == configPath {
			return m.EnvVar
		}
	}
	return ""
}

// IsSensitiveConfigPath checks if a config path is marked as sensitive
func IsSensitiveConfigPath(configPath string) bool {
	cfg := &Config{}
	return checkSensitiveField(reflect.TypeOf(cfg).Elem(), strings.Split(configPath, "."))
}

// checkSensitiveField recursively checks if a field is marked as sensitive
func checkSensitiveField(t reflect.Type, pathParts []string) bool {
	if len(pathParts) == 0 {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		koanfTag := field.Tag.Get("koanf")

		if koanfTag == pathParts[0] {
			if len(pathParts) == 1 {
				// NOTE: Treat SensitiveString or explicit sensitive tags as secrets for env mapping.
				if field.Type.Name() == "SensitiveString" {
					return true
				}
				sensitiveTag := field.Tag.Get("sensitive")
				return sensitiveTag == "true"
			}

			if field.Type.Kind() == reflect.Struct && field.Type.PkgPath() != "time" {
				return checkSensitiveField(field.Type, pathParts[1:])
			}
		}
	}
	return false
}
