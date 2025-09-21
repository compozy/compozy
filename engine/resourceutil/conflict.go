package resourceutil

import "fmt"

type ReferenceDetail struct {
	Resource string
	IDs      []string
}

type ConflictError struct {
	Details []ReferenceDetail
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("resource referenced by %d collections", len(e.Details))
}
