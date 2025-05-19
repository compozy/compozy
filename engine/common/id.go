package common

import "github.com/google/uuid"

// generateExecID generates a unique execution ID
func GenerateExecID() string {
	return uuid.New().String()
}

// generateEventID generates a unique event ID
func GenerateEventID() *string {
	id := uuid.New().String()
	return &id
}

// generateRequestID generates a unique request ID
func GenerateRequestID() *string {
	id := uuid.New().String()
	return &id
}
