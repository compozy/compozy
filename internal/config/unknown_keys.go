package config

import (
	"errors"
	"fmt"
	"strings"
)

var ErrWorkspaceConfigInvalid = errors.New("workspace config invalid")

type UnknownKeysError struct {
	Path string
	Keys []string
	Err  error
}

func (e *UnknownKeysError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("unknown config keys in %q: %s", e.Path, strings.Join(e.Keys, ", "))
}

func (e *UnknownKeysError) Unwrap() error { return e.Err }
