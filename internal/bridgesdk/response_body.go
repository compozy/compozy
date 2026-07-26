package bridgesdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DecodeSingleJSONValue decodes exactly one JSON value and rejects trailing values or tokens.
func DecodeSingleJSONValue(reader io.Reader, destination any) error {
	if reader == nil {
		return errors.New("bridgesdk: HTTP response body is required")
	}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return fmt.Errorf("bridgesdk: decode trailing HTTP response JSON: %w", err)
	default:
		return errors.New("bridgesdk: HTTP response contains multiple JSON values")
	}
}

// ReadHTTPResponseBody returns bytes received before an HTTP response read fails.
func ReadHTTPResponseBody(reader io.Reader) (string, error) {
	if reader == nil {
		return "", nil
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return string(body), fmt.Errorf("bridgesdk: read HTTP response body: %w", err)
	}
	return string(body), nil
}

// HTTPResponseCommitPolicy defines which observation proves an HTTP operation committed remotely.
type HTTPResponseCommitPolicy uint8

const (
	// HTTPResponseNoCommit keeps response cleanup failures on the returned operation error.
	HTTPResponseNoCommit HTTPResponseCommitPolicy = iota
	// HTTPResponseCommitOnSuccessStatus treats a successful status as remote commit evidence.
	HTTPResponseCommitOnSuccessStatus
)

// HTTPResponseCommitEvidence records why retrying a successful HTTP operation is unsafe.
type HTTPResponseCommitEvidence uint8

const (
	// HTTPResponseCommitUnconfirmed means no remote mutation has been proven.
	HTTPResponseCommitUnconfirmed HTTPResponseCommitEvidence = iota
	// HTTPResponseCommittedBySuccessStatus means a successful status confirms the mutation.
	HTTPResponseCommittedBySuccessStatus
)

// Evidence derives observed commit evidence after a successful status.
func (p HTTPResponseCommitPolicy) Evidence() HTTPResponseCommitEvidence {
	switch p {
	case HTTPResponseCommitOnSuccessStatus:
		return HTTPResponseCommittedBySuccessStatus
	default:
		return HTTPResponseCommitUnconfirmed
	}
}

// DrainAndCloseHTTPResponseBody drains and closes a provider response, joining both failures.
func DrainAndCloseHTTPResponseBody(body io.ReadCloser) error {
	if body == nil {
		return nil
	}

	var drainErr error
	if _, err := io.Copy(io.Discard, body); err != nil {
		drainErr = fmt.Errorf("bridgesdk: drain HTTP response body: %w", err)
	}
	var closeErr error
	if err := body.Close(); err != nil {
		closeErr = fmt.Errorf("bridgesdk: close HTTP response body: %w", err)
	}
	return errors.Join(drainErr, closeErr)
}

// FinalizeHTTPResponseBody preserves cleanup failures until remote commit evidence exists. Once
// committed, cleanup is reported without invalidating the operation. A missing reporter keeps the
// cleanup failure on the returned operation error rather than discarding it.
func FinalizeHTTPResponseBody(
	body io.ReadCloser,
	operationErr error,
	commitEvidence HTTPResponseCommitEvidence,
	reportCleanup func(error),
) error {
	cleanupErr := DrainAndCloseHTTPResponseBody(body)
	if operationErr != nil && commitEvidence.confirmed() {
		return MarkCommittedMutation(errors.Join(operationErr, cleanupErr))
	}
	if cleanupErr == nil {
		return operationErr
	}
	if operationErr != nil || !commitEvidence.confirmed() {
		return errors.Join(operationErr, cleanupErr)
	}
	if reportCleanup == nil {
		return MarkCommittedMutation(cleanupErr)
	}
	reportCleanup(cleanupErr)
	return nil
}

func (e HTTPResponseCommitEvidence) confirmed() bool {
	return e == HTTPResponseCommittedBySuccessStatus
}
