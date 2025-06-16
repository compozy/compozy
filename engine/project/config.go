package project

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/schema"
)

type WorkflowSourceConfig struct {
	Source string `json:"source" yaml:"source" mapstructure:"source"`
}

type RuntimeConfig struct {
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty" mapstructure:"permissions"`
}

type Opts struct {
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`
}

type Config struct {
	Name             string                  `json:"name"                 yaml:"name"                 mapstructure:"name"`
	Version          string                  `json:"version"              yaml:"version"              mapstructure:"version"`
	Description      string                  `json:"description"          yaml:"description"          mapstructure:"description"`
	Author           core.Author             `json:"author"               yaml:"author"               mapstructure:"author"`
	Workflows        []*WorkflowSourceConfig `json:"workflows"            yaml:"workflows"            mapstructure:"workflows"`
	Models           []*core.ProviderConfig  `json:"models"               yaml:"models"               mapstructure:"models"`
	Schemas          []schema.Schema         `json:"schemas"              yaml:"schemas"              mapstructure:"schemas"`
	Opts             Opts                    `json:"config"               yaml:"config"               mapstructure:"config"`
	Runtime          RuntimeConfig           `json:"runtime"              yaml:"runtime"              mapstructure:"runtime"`
	CacheConfig      *cache.Config           `json:"cache,omitempty"      yaml:"cache,omitempty"      mapstructure:"cache"`
	AutoLoad         *autoload.Config        `json:"autoload,omitempty"   yaml:"autoload,omitempty"   mapstructure:"autoload,omitempty"`
	MonitoringConfig *monitoring.Config      `json:"monitoring,omitempty" yaml:"monitoring,omitempty" mapstructure:"monitoring"`

	filePath           string
	CWD                *core.PathCWD `json:"CWD,omitempty" yaml:"CWD,omitempty" mapstructure:"CWD,omitempty"`
	env                *core.EnvMap
	autoloadValidated  bool
	autoloadValidError error
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
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	p.CWD = CWD
	return nil
}

func (p *Config) GetCWD() *core.PathCWD {
	return p.CWD
}

func (p *Config) HasSchema() bool {
	return false
}

func (p *Config) Validate() error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(p.CWD, p.Name),
	)
	if err := validator.Validate(); err != nil {
		return err
	}
	// Validate cache configuration if present
	if p.CacheConfig != nil {
		if err := p.CacheConfig.Validate(); err != nil {
			return fmt.Errorf("cache configuration validation failed: %w", err)
		}
	}
	// Validate monitoring configuration if present
	if p.MonitoringConfig != nil {
		if err := p.MonitoringConfig.Validate(); err != nil {
			return fmt.Errorf("monitoring configuration validation failed: %w", err)
		}
	}
	// Validate autoload configuration if present (with caching)
	if p.AutoLoad != nil {
		if !p.autoloadValidated {
			p.autoloadValidError = p.AutoLoad.Validate()
			p.autoloadValidated = true
		}
		if p.autoloadValidError != nil {
			return fmt.Errorf("autoload configuration validation failed: %w", p.autoloadValidError)
		}
	}

	return nil
}

func (p *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	return nil
}

func (p *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the project having a schema
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

func (p *Config) loadEnv(envFilePath string) (core.EnvMap, error) {
	if p.CWD == nil {
		return nil, fmt.Errorf("working directory not set for project %q", p.Name)
	}
	env, err := core.NewEnvFromFile(p.CWD.PathStr(), envFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}
	return env, nil
}

func (p *Config) SetEnv(env core.EnvMap) {
	p.env = &env
}

func (p *Config) GetEnv() core.EnvMap {
	if p.env == nil {
		return core.EnvMap{}
	}
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

func Load(cwd *core.PathCWD, path string, envFilePath string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	if config.CWD == nil {
		config.CWD = cwd
	}
	// Set autoload defaults if autoload config exists
	if config.AutoLoad != nil {
		config.AutoLoad.SetDefaults()
	}
	// Set monitoring defaults if not configured
	if config.MonitoringConfig == nil {
		config.MonitoringConfig = monitoring.DefaultConfig()
	}
	env, err := config.loadEnv(envFilePath)
	if err != nil {
		return nil, err
	}
	config.SetEnv(env)
	return config, nil
}
