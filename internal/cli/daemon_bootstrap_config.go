package cli

import (
	"errors"
	"slices"

	burnttoml "github.com/BurntSushi/toml"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// bootstrapConfigError names the config file and its fault so the desktop does not parse messages.
type bootstrapConfigError struct {
	Path        string   `json:"path,omitempty"`
	UnknownKeys []string `json:"unknown_keys,omitempty"`
	// Line and Column locate invalid TOML; both are 1-based and omitted when unknown.
	Line   int `json:"line,omitzero"`
	Column int `json:"column,omitzero"`
}

// bootstrapConfigFailure reports that this process could not load its configuration. Retrying
// cannot fix it and no daemon answered, so it never counts as an unhealthy runtime.
type bootstrapConfigFailure struct {
	cause error
	// startErr is the launch failure the config error explains, when one preceded the diagnosis.
	startErr error
}

func (e *bootstrapConfigFailure) Error() string {
	return "cli: CompozyOS configuration could not be loaded: " + e.cause.Error()
}

func (e *bootstrapConfigFailure) Unwrap() []error {
	if e.startErr == nil {
		return []error{e.cause}
	}
	return []error{e.cause, e.startErr}
}

func (e *bootstrapConfigFailure) detail() *bootstrapConfigError {
	detail := &bootstrapConfigError{}
	if unknown, ok := errors.AsType[*compozyconfig.UnknownKeysError](e.cause); ok {
		detail.Path = unknown.Path
		detail.UnknownKeys = slices.Clone(unknown.Keys)
		return detail
	}
	if file, ok := errors.AsType[compozyconfig.FileError](e.cause); ok {
		detail.Path = file.Path
	}
	if parse, ok := errors.AsType[burnttoml.ParseError](e.cause); ok {
		detail.Line, detail.Column = parse.Position.Line, parse.Position.Col
	}
	return detail
}

// configFailureAfterStart explains a failed launch with the config error the runtime also hits.
// It loads config only after a start already failed, so it never blocks a boot that would succeed.
func (e *bootstrapExecution) configFailureAfterStart(startErr error) error {
	if failure, ok := errors.AsType[*bootstrapConfigFailure](startErr); ok {
		return failure
	}
	if e.cmd.Context().Err() != nil {
		return nil
	}
	if _, err := e.deps.loadConfig(); err != nil {
		return &bootstrapConfigFailure{cause: err, startErr: startErr}
	}
	return nil
}
