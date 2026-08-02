package acp

import "errors"

// ErrInvalidPromptMetadata identifies metadata that cannot be sent to an ACP agent.
var ErrInvalidPromptMetadata = errors.New("acp: invalid prompt metadata")

type promptMetadataError struct {
	message string
}

func (e *promptMetadataError) Error() string {
	return e.message
}

func (e *promptMetadataError) Unwrap() error {
	return ErrInvalidPromptMetadata
}

func invalidPromptMetadata(message string) error {
	return &promptMetadataError{message: message}
}
