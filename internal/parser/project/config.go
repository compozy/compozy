package project

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/validator"
	"github.com/compozy/compozy/internal/parser/workflow"
)

type LogLevel string
type Dependencies []*pkgref.PackageRef
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

// IsValidLogLevel checks if the given log level is valid
func IsValidLogLevel(level LogLevel) bool {
	switch level {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError:
		return true
	default:
		return false
	}
}

// EnvironmentConfig represents environment configuration
type EnvironmentConfig struct {
	LogLevel LogLevel `json:"log_level" yaml:"log_level"`
	EnvFile  string   `json:"env_file" yaml:"env_file"`
}

// WorkflowSourceConfig represents a workflow source configuration
type WorkflowSourceConfig struct {
	Source string `json:"source" yaml:"source"`
}

// ProjectConfig represents a project configuration
type ProjectConfig struct {
	Name         string                        `json:"name" yaml:"name"`
	Version      string                        `json:"version" yaml:"version"`
	Description  string                        `json:"description,omitempty" yaml:"description,omitempty"`
	Author       author.Author                 `json:"author,omitempty" yaml:"author,omitempty"`
	Dependencies *Dependencies                 `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Environments map[string]*EnvironmentConfig `json:"environments,omitempty" yaml:"environments,omitempty"`
	Workflows    []*WorkflowSourceConfig       `json:"workflows" yaml:"workflows"`

	cwd *common.CWD // internal field for current working directory
}

// SetCWD sets the current working directory for the project
func (p *ProjectConfig) SetCWD(path string) error {
	normalizedPath, err := common.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	p.cwd = normalizedPath
	return nil
}

// GetCWD returns the current working directory
func (p *ProjectConfig) GetCWD() string {
	if p.cwd == nil {
		return ""
	}
	return p.cwd.Get()
}

// Load loads a project configuration from a file
func Load(path string) (*ProjectConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open project config file: %w", err)
	}

	var config ProjectConfig
	decoder := yaml.NewDecoder(file)
	decodeErr := decoder.Decode(&config)
	closeErr := file.Close()

	if decodeErr != nil {
		return nil, fmt.Errorf("failed to decode project config file: %w", decodeErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("failed to close project config file: %w", closeErr)
	}

	if err := config.SetCWD(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("failed to set project CWD: %w", err)
	}
	return &config, nil
}

// Validate validates the project configuration
func (p *ProjectConfig) Validate() error {
	validator := validator.NewCompositeValidator(
		validator.NewCWDValidator(p.cwd, p.Name),
	)
	return validator.Validate()
}

// WorkflowsFromSources loads all workflow configurations from their sources
func (p *ProjectConfig) WorkflowsFromSources() ([]*workflow.WorkflowConfig, error) {
	var workflows []*workflow.WorkflowConfig
	for _, wf := range p.Workflows {
		workflowPath := p.cwd.Join(wf.Source)
		wfConfig, err := workflow.Load(workflowPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load workflow from source: %w", err)
		}
		workflows = append(workflows, wfConfig)
	}
	return workflows, nil
}
