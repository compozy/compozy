package project

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/schema"
)

type WorkflowSourceConfig struct {
	Source string `json:"source" yaml:"source" mapstructure:"source"`
}

type Opts struct {
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`
}

type Config struct {
	Name        string                  `json:"name"        yaml:"name"        mapstructure:"name"`
	Version     string                  `json:"version"     yaml:"version"     mapstructure:"version"`
	Description string                  `json:"description" yaml:"description" mapstructure:"description"`
	Author      core.Author             `json:"author"      yaml:"author"      mapstructure:"author"`
	Workflows   []*WorkflowSourceConfig `json:"workflows"   yaml:"workflows"   mapstructure:"workflows"`
	Models      []*llm.ProviderConfig   `json:"models"      yaml:"models"      mapstructure:"models"`
	Schemas     []schema.Schema         `json:"schemas"     yaml:"schemas"     mapstructure:"schemas"`
	Opts        Opts                    `json:"config"      yaml:"config"      mapstructure:"config"`

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

func (p *Config) GetEnv() core.EnvMap {
	return *p.env
}

func (p *Config) GetInput() *core.Input {
	return &core.Input{}
}

func (p *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(p)
}

func (p *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return p.Merge(config)
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
