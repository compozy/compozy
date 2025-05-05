package parser

import (
	"encoding/json"
	"io"
	"os"

	"github.com/compozy/compozy/parser/agent"
	"github.com/compozy/compozy/parser/author"
	"github.com/compozy/compozy/parser/project"
	"github.com/compozy/compozy/parser/registry"
	"github.com/compozy/compozy/parser/task"
	"github.com/compozy/compozy/parser/tool"
	"github.com/compozy/compozy/parser/workflow"
	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Project   *project.ProjectConfig     `json:"project" yaml:"project"`
	Author    *author.Author             `json:"author" yaml:"author"`
	Registry  *registry.RegistryConfig   `json:"registry" yaml:"registry"`
	Agents    []*agent.AgentConfig       `json:"agents" yaml:"agents"`
	Tools     []*tool.ToolConfig         `json:"tools" yaml:"tools"`
	Tasks     []*task.TaskConfig         `json:"tasks" yaml:"tasks"`
	Workflows []*workflow.WorkflowConfig `json:"workflows" yaml:"workflows"`
}

// ParseFile reads and parses a configuration file
func ParseFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Parse(file)
}

// Parse reads and parses a configuration from an io.Reader
func Parse(r io.Reader) (*Config, error) {
	var config Config
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ToJSON converts the configuration to JSON
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// ToYAML converts the configuration to YAML
func (c *Config) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}
