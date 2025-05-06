package common

type Config interface {
	SetCWD(path string)
	GetCWD() string
	Validate() error
	Merge(other any) error
	LoadID() (string, error)
}
