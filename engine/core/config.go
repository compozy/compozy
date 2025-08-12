package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mitchellh/mapstructure"
)

type Config interface {
	Component() ConfigType
	SetFilePath(string)
	GetFilePath() string
	SetCWD(path string) error
	GetCWD() *PathCWD
	GetEnv() EnvMap
	GetInput() *Input
	Validate() error
	ValidateInput(ctx context.Context, input *Input) error
	ValidateOutput(ctx context.Context, output *Output) error
	HasSchema() bool
	Merge(other any) error
	AsMap() (map[string]any, error)
	FromMap(any) error
}

type ConfigType string

const (
	ConfigProject  ConfigType = "project"
	ConfigWorkflow ConfigType = "workflow"
	ConfigTask     ConfigType = "task"
	ConfigAgent    ConfigType = "agent"
	ConfigTool     ConfigType = "tool"
	ConfigMCP      ConfigType = "mcp"
	ConfigMemory   ConfigType = "memory" // Added for memory resources
)

func AsMapDefault(config any) (map[string]any, error) {
	bytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var configMap map[string]any
	if err := json.Unmarshal(bytes, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config to map: %w", err)
	}
	return configMap, nil
}

func FromMapDefault[T any](data any) (T, error) {
	var config T

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &config,
		TagName:          "mapstructure", // Use mapstructure tags as per project standard
	})
	if err != nil {
		return config, err
	}

	return config, decoder.Decode(data)
}
