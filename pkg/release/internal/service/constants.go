package service

import "time"

// Timeout constants for service operations
const (
	// DefaultCliffTimeout is the timeout for git-cliff operations
	DefaultCliffTimeout = 30 * time.Second
	// DefaultNPMTimeout is the timeout for npm operations
	DefaultNPMTimeout = 60 * time.Second
)
