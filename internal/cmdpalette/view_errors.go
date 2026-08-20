package cmdpalette

import "fmt"

// ViewValidationError names the failing wire path. Match with errors.As.
type ViewValidationError struct {
	Path    string
	Message string
}

func (e *ViewValidationError) Error() string {
	return fmt.Sprintf("cmd palette view: %s: %s", e.Path, e.Message)
}

// UnknownViewKindError reports a view kind this host cannot render. Match with errors.As.
type UnknownViewKindError struct {
	Kind ViewKind
}

func (e *UnknownViewKindError) Error() string {
	return fmt.Sprintf("cmd palette view: unknown view kind %q", e.Kind)
}

// ViewRevisionMismatchError reports a patch fence gap. Match with errors.As.
type ViewRevisionMismatchError struct {
	Current string
	From    string
}

func (e *ViewRevisionMismatchError) Error() string {
	return fmt.Sprintf("cmd palette view: patch starts at %q, current revision is %q", e.From, e.Current)
}

// ViewNotFoundError reports a missing view id. Match with errors.As.
type ViewNotFoundError struct {
	ViewID string
}

func (e *ViewNotFoundError) Error() string {
	return fmt.Sprintf("cmd palette view: %q not found", e.ViewID)
}
