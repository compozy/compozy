package project

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

type (
	LogLevel     string
	Dependencies []*core.PackageRef
	Environment  string
)

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentStaging     Environment = "staging"

	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

func IsValidLogLevel(level LogLevel) bool {
	switch level {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError:
		return true
	default:
		return false
	}
}

type EnvironmentConfig struct {
	LogLevel LogLevel `json:"log_level" yaml:"log_level"`
	EnvFile  string   `json:"env_file"  yaml:"env_file"`
}

type WorkflowSourceConfig struct {
	Source string `json:"source" yaml:"source"`
}

type Config struct {
	Name         string                        `json:"name"                   yaml:"name"`
	Version      string                        `json:"version"                yaml:"version"`
	Description  string                        `json:"description,omitempty"  yaml:"description,omitempty"`
	Author       core.Author                   `json:"author,omitempty"       yaml:"author,omitempty"`
	Dependencies *Dependencies                 `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Environments map[string]*EnvironmentConfig `json:"environments,omitempty" yaml:"environments,omitempty"`
	Workflows    []*WorkflowSourceConfig       `json:"workflows"              yaml:"workflows"`

	filePath string
	cwd      *core.CWD
	env      *core.EnvMap
}

func (p *Config) Component() core.ConfigType {
	return core.ConfigProject
}

func (p *Config) GetFilePath() string {
	return p.filePath
}

func (p *Config) SetFilePath(path string) {
	p.filePath = path
}

func (p *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	p.cwd = cwd
	return nil
}

func (p *Config) GetCWD() *core.CWD {
	return p.cwd
}

func (p *Config) Validate() error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(p.cwd, p.Name),
	)
	return validator.Validate()
}

func (p *Config) ValidateParams(_ context.Context, _ *core.Input) error {
	return nil
}

func (p *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge project configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(p, otherConfig, mergo.WithOverride)
}

func (p *Config) LoadID() (string, error) {
	return p.Name, nil
}

func (p *Config) loadEnv() (core.EnvMap, error) {
	env, err := core.NewEnvFromFile(p.cwd.PathStr())
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}
	return env, nil
}

func (p *Config) SetEnv(env core.EnvMap) {
	p.env = &env
}

func (p *Config) GetEnv() *core.EnvMap {
	return p.env
}

func (p *Config) GetInput() *core.Input {
	return &core.Input{}
}

func Load(cwd *core.CWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}

	env, err := config.loadEnv()
	if err != nil {
		return nil, err
	}
	config.SetEnv(env)
	return config, nil
}
