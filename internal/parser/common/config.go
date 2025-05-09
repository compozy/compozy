package common

type Config interface {
	SetCWD(path string)
	GetCWD() string
	Validate() error
	ValidateParams(input map[string]any) error
	Merge(other any) error
	LoadID() (string, error)
}
