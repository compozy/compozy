package core

type Config interface {
	Component() ConfigType
	SetCWD(path string) error
	GetCWD() *CWD
	GetEnv() *EnvMap
	GetInput() *Input
	Validate() error
	ValidateParams(input *Input) error
	Merge(other any) error
	LoadID() (string, error)
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
