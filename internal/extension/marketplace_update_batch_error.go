package extensionpkg

import (
	"fmt"
	"strings"
)

// MarketplaceUpdateBatchError reports a failed target after earlier batch
// results became externally visible. Completed is a stable snapshot of those
// results so callers can emit lifecycle events and report partial progress.
type MarketplaceUpdateBatchError struct {
	FailedName string
	Completed  []MarketplaceUpdateResult
	Cause      error
}

func (e *MarketplaceUpdateBatchError) Error() string {
	if e == nil {
		return "extension: marketplace update batch failed"
	}
	name := strings.TrimSpace(e.FailedName)
	if name == "" {
		name = "unknown"
	}
	return fmt.Sprintf(
		"extension: marketplace update batch failed at %q after %d completed result(s): %v",
		name,
		len(e.Completed),
		e.Cause,
	)
}

func (e *MarketplaceUpdateBatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newMarketplaceUpdateBatchError(
	failedName string,
	completed []MarketplaceUpdateResult,
	cause error,
) *MarketplaceUpdateBatchError {
	return &MarketplaceUpdateBatchError{
		FailedName: strings.TrimSpace(failedName),
		Completed:  append([]MarketplaceUpdateResult(nil), completed...),
		Cause:      cause,
	}
}
