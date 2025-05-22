package common

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

func (e *EnvMap) Prop(key string) string {
	if e == nil {
		return ""
	}
	return (*e)[key]
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
