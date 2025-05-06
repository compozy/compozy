package common

type ComponentConfig interface {
	SetCWD(path string)
	GetCWD() string
	Validate() error
	Merge(other any) error
}
