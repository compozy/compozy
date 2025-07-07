package helpers

import "time"

// BoolPtr returns a pointer to a bool value
func BoolPtr(b bool) *bool {
	return &b
}

// IntPtr returns a pointer to an int value
func IntPtr(i int) *int {
	return &i
}

// TimePtr returns a pointer to a time value
func TimePtr(t time.Time) *time.Time {
	return &t
}
