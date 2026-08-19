package settings

import "fmt"

// AliasConflictError identifies both owners of one workspace alias.
type AliasConflictError struct {
	Alias   string
	Owner   string
	Command string
}

func (e *AliasConflictError) Error() string {
	return fmt.Sprintf("alias %q conflicts between %q and %q", e.Alias, e.Owner, e.Command)
}

// InvalidAliasError reports the stable public alias grammar.
type InvalidAliasError struct {
	CommandID string
}

func (e *InvalidAliasError) Error() string {
	return "aliases are 1-32 chars, no whitespace"
}
