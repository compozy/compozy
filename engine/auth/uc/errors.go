package uc

import "errors"

// ErrUserNotFound is returned when a user is not found in the repository
var ErrUserNotFound = errors.New("user not found")
