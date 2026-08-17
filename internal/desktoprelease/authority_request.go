package desktoprelease

import (
	"fmt"
	"time"
)

func validateAuthorityRequest(operationID, channel, version string, publishedAt time.Time) error {
	if err := validateOperationRequest(operationID, channel, version); err != nil {
		return errorWithCode(ErrorVerificationFailed, err)
	}
	if publishedAt.IsZero() {
		return errorWithCode(
			ErrorVerificationFailed,
			fmt.Errorf("desktop release: published_at is required"),
		)
	}
	return nil
}

func validateOperationRequest(operationID, channel, version string) error {
	if operationID == "" || len(operationID) > 128 {
		return fmt.Errorf("desktop release: operation id must contain 1 to 128 safe characters")
	}
	for _, character := range operationID {
		if !isSafeOperationIDCharacter(character) {
			return fmt.Errorf("desktop release: operation id contains unsupported character %q", character)
		}
	}
	if err := ValidateDesktopChannel(channel); err != nil {
		return err
	}
	return ValidateVersion(version)
}

func isSafeOperationIDCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '-' || character == '_' || character == '.'
}

func validateExistingOperation(commit *ChannelCommit, operation, version string) error {
	if commit.Generation.Operation != operation || commit.Generation.Version != version {
		return fmt.Errorf(
			"desktop release: operation id %q already belongs to %s %s",
			commit.Generation.OperationID,
			commit.Generation.Operation,
			commit.Generation.Version,
		)
	}
	return nil
}
