package session

import "time"

func cloneSessionTimePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	return new(value.UTC())
}
