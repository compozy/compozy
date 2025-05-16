package common

type Config interface {
	Component() ConfigType
	SetCWD(path string) error
	GetCWD() *CWD
	Validate() error
	ValidateParams(input map[string]any) error
	Merge(other any) error
	LoadID() (string, error)
}

type ConfigType string

const (
	ConfigTypeProject  ConfigType = "project"
	ConfigTypeWorkflow ConfigType = "workflow"
	ConfigTypeTask     ConfigType = "task"
	ConfigTypeAgent    ConfigType = "agent"
	ConfigTypeTool     ConfigType = "tool"
)

type RefLoader interface {
	LoadFileRef(cwd *CWD) (Config, error)
}
