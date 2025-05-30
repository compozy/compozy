package core

import "context"

type ConfigMetadata struct {
	CWD         *CWD
	FilePath    string
	ProjectRoot string
}

func (m *ConfigMetadata) ResolvedPath() (string, error) {
	return ResolvedPath(m.CWD, m.FilePath)
}

type Config interface {
	Component() ConfigType
	GetCWD() *CWD
	GetEnv() *EnvMap
	GetInput() *Input
	GetMetadata() *ConfigMetadata
	SetMetadata(metadata *ConfigMetadata)
	ResolveRef(ctx context.Context, currentDoc map[string]any, projectRoot, filePath string) error
	Validate() error
	ValidateParams(input *Input) error
	Merge(other any) error
}

type ConfigType string

const (
	ConfigProject  ConfigType = "project"
	ConfigWorkflow ConfigType = "workflow"
	ConfigTask     ConfigType = "task"
	ConfigAgent    ConfigType = "agent"
	ConfigTool     ConfigType = "tool"
)

type RefLoader interface {
	LoadFileRef(cwd *CWD) (Config, error)
}
