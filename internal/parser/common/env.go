package common

import (
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"github.com/joho/godotenv"
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
	env := make(EnvMap)
	if e == nil && other == nil {
		return env, nil
	}
	if e != nil {
		if err := mergo.Merge(&env, *e, mergo.WithOverride); err != nil {
			return nil, err
		}
	}
	if err := mergo.Merge(&env, other, mergo.WithOverride); err != nil {
		return nil, err
	}
	return env, nil
}
