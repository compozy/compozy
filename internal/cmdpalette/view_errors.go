package cmdpalette

import "fmt"

type ViewValidationError struct {
	Path    string
	Message string
}

func (e *ViewValidationError) Error() string {
	return fmt.Sprintf("cmd palette view: %s: %s", e.Path, e.Message)
}

type UnknownViewKindError struct {
	Kind ViewKind
}

func (e *UnknownViewKindError) Error() string {
	return fmt.Sprintf("cmd palette view: unknown view kind %q", e.Kind)
}

type ViewRevisionMismatchError struct {
	Current string
	From    string
}

func (e *ViewRevisionMismatchError) Error() string {
	return fmt.Sprintf("cmd palette view: patch starts at %q, current revision is %q", e.From, e.Current)
}

type ViewNotFoundError struct {
	ViewID string
}

func (e *ViewNotFoundError) Error() string {
	return fmt.Sprintf("cmd palette view: %q not found", e.ViewID)
}
