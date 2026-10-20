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
}

var _ error = (*UnknownKeysError)(nil)

func (e *UnknownKeysError) Error() string {
	return fmt.Sprintf("unknown config keys in %q: %s", e.Path, strings.Join(e.Keys, ", "))
}
