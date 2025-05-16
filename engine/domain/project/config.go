package project

import (
	"errors"
	"fmt"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/schema"
)

type LogLevel string
type Dependencies []*common.PackageRef
type Environment string

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
	Author       common.Author                 `json:"author,omitempty"       yaml:"author,omitempty"`
	Dependencies *Dependencies                 `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Environments map[string]*EnvironmentConfig `json:"environments,omitempty" yaml:"environments,omitempty"`
	Workflows    []*WorkflowSourceConfig       `json:"workflows"              yaml:"workflows"`

	cwd *common.CWD
}

func (p *Config) Component() common.ConfigType {
	return common.ConfigTypeProject
}

func (p *Config) SetCWD(path string) error {
	cwd, err := common.CWDFromPath(path)
	if err != nil {
		return err
	}
	p.cwd = cwd
	return nil
}

func (p *Config) GetCWD() *common.CWD {
	return p.cwd
}

func (p *Config) Validate() error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(p.cwd, p.Name),
	)
	return validator.Validate()
}

func (p *Config) ValidateParams(_ map[string]any) error {
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

func Load(cwd *common.CWD, path string) (*Config, error) {
	config, err := common.LoadConfig[*Config](cwd, path)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (p *Config) WorkflowsFromSources() ([]*workflow.Config, error) {
	var ws []*workflow.Config
	for _, wf := range p.Workflows {
		config, err := workflow.Load(p.cwd, wf.Source)
		if err != nil {
			return nil, err
		}
		ws = append(ws, config)
	}
	return ws, nil
}
