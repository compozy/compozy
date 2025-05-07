package project

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
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
func (p *ProjectConfig) SetCWD(path string) {
	if p.cwd == nil {
		p.cwd = common.NewCWD(path)
	} else {
		p.cwd.Set(path)
	}
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
		return nil, NewFileOpenError(err)
	}

	var config ProjectConfig
	decoder := yaml.NewDecoder(file)
	decodeErr := decoder.Decode(&config)
	closeErr := file.Close()

	if decodeErr != nil {
		return nil, NewDecodeError(decodeErr)
	}
	if closeErr != nil {
		return nil, NewFileCloseError(closeErr)
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the project configuration
func (p *ProjectConfig) Validate() error {
	validator := common.NewCompositeValidator(
		NewCWDValidator(p.cwd),
		NewWorkflowsValidator(p.Workflows),
		NewEnvironmentsValidator(p.Environments),
		NewDependenciesValidator(p.Dependencies),
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
			return nil, NewWorkflowLoadError(err)
		}
		workflows = append(workflows, wfConfig)
	}
	return workflows, nil
}
