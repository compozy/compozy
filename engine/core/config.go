package core

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

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
	Validate(ctx context.Context) error
	ValidateInput(ctx context.Context, input *Input) error
	ValidateOutput(ctx context.Context, output *Output) error
	HasSchema() bool
	Merge(other any) error
	AsMap() (map[string]any, error)
	FromMap(any) error
}

type ConfigType string

const (
	ConfigProject       ConfigType = "project"
	ConfigWorkflow      ConfigType = "workflow"
	ConfigTask          ConfigType = "task"
	ConfigAgent         ConfigType = "agent"
	ConfigTool          ConfigType = "tool"
	ConfigMCP           ConfigType = "mcp"
	ConfigMemory        ConfigType = "memory" // Added for memory resources
	ConfigKnowledgeBase ConfigType = "knowledge_base"
	ConfigEmbedder      ConfigType = "embedder"
	ConfigVectorDB      ConfigType = "vector_db"
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

// FromMapDefault decodes a generic map-like value into a value of type T.
// It uses github.com/mitchellh/mapstructure with `WeaklyTypedInput` enabled
// and reads struct field tags named `mapstructure`.
// The input `data` is typically a map[string]any (e.g., from JSON/unstructured sources)
// but can be any value supported by mapstructure's decoding rules.
// Returns the decoded value or the zero value of T and an error if decoder creation
// or decoding fails.
func FromMapDefault[T any](data any) (T, error) {
	var config T
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &config,
		TagName:          "mapstructure", // Use mapstructure tags as per project standard
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(StringToMapAliasPtrHook),
	})
	if err != nil {
		return config, err
	}
	return config, decoder.Decode(data)
}

// stringToMapAliasPtrHook converts a scalar string into a pointer to any
// named map type whose underlying type is map[string]any. This enables
// mapstructure to accept string IDs for alias types like *schema.Schema
// when decoding from generic maps (e.g., router inline routes), aligning
// behavior with YAML unmarshalling which already supports scalar schema IDs.
//
// The hook creates a new map value and sets a reserved key "__schema_ref__"
// to the provided string. The schema linker later interprets this sentinel
// as a schema ID to resolve. This avoids an import cycle with the schema
// package while maintaining consistent semantics across decoders.
// StringToMapAliasPtrHook is exported for reuse.
func StringToMapAliasPtrHook(from reflect.Type, to reflect.Type, data any) (any, error) {
	if from.Kind() != reflect.String {
		return data, nil
	}
	// Expect pointer to a named map type with map[string]any underlying
	if to.Kind() != reflect.Ptr {
		return data, nil
	}
	elem := to.Elem()
	if elem.Kind() != reflect.Map {
		return data, nil
	}
	if elem.Key().Kind() != reflect.String || elem.Elem().Kind() != reflect.Interface {
		return data, nil
	}
	// Construct map and set sentinel reference
	id, ok := data.(string)
	if !ok {
		return data, nil
	}
	m := reflect.MakeMap(elem)
	m.SetMapIndex(reflect.ValueOf("__schema_ref__"), reflect.ValueOf(id))
	ptr := reflect.New(elem)
	ptr.Elem().Set(m)
	return ptr.Interface(), nil
}
