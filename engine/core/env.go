package core

import (
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"github.com/joho/godotenv"
	"google.golang.org/protobuf/types/known/structpb"
)

type EnvMap map[string]string

func NewEnvFromFile(cwd string) (EnvMap, error) {
	envPath := filepath.Join(cwd, ".env")
	envMap, err := godotenv.Read(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(EnvMap), nil
		}
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}
	return EnvMap(envMap), nil
}

func (e *EnvMap) Merge(other EnvMap) (EnvMap, error) {
	result := make(EnvMap)
	if e != nil {
		result = *e
	}
	if err := mergo.Merge(&result, other, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	return result, nil
}

func (e EnvMap) Prop(key string) string {
	if e == nil {
		return ""
	}
	return e[key]
}

func (e *EnvMap) Set(key, value string) {
	if e == nil {
		return
	}
	(*e)[key] = value
}

func (e *EnvMap) ToProtoBufMap() (map[string]any, error) {
	return DefaultToProtoMap(*e)
}

func (e *EnvMap) ToStruct() (*structpb.Struct, error) {
	m, err := e.ToProtoBufMap()
	if err != nil {
		return nil, err
	}
	return structpb.NewStruct(m)
}

func (e *EnvMap) AsMap() map[string]any {
	if e == nil {
		return make(map[string]any)
	}
	result := make(map[string]any)
	for k, v := range *e {
		result[k] = v
	}
	return result
}

// EnvMerger handles environment merging logic
type EnvMerger struct{}

// Merge combines multiple environment maps in order, with later maps overriding earlier ones
func (e *EnvMerger) Merge(envs ...EnvMap) (EnvMap, error) {
	result := make(EnvMap)
	for _, env := range envs {
		if env == nil {
			continue
		}
		merged, err := result.Merge(env)
		if err != nil {
			return nil, fmt.Errorf("failed to merge environments: %w", err)
		}
		result = merged
	}
	return result, nil
}

// MergeWithDefaults merges environments, ensuring non-nil maps
func (e *EnvMerger) MergeWithDefaults(envs ...EnvMap) (EnvMap, error) {
	safeEnvs := make([]EnvMap, 0, len(envs))
	for _, env := range envs {
		if env == nil {
			safeEnvs = append(safeEnvs, make(EnvMap))
		} else {
			safeEnvs = append(safeEnvs, env)
		}
	}
	return e.Merge(safeEnvs...)
}
