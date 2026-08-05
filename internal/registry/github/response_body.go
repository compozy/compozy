package github

import (
	"errors"
	"fmt"
	"io"
)

const maxReleaseMetadataBytes = int64(4 << 20)

var errReleaseMetadataTooLarge = errors.New("github: release metadata response exceeds limit")

func readReleaseMetadata(body io.Reader, context string) ([]byte, error) {
	if body == nil {
		return nil, fmt.Errorf("github: read %s: response body is required", context)
	}
	reader := &io.LimitedReader{R: body, N: maxReleaseMetadataBytes + 1}
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("github: read %s: %w", context, err)
	}
	if int64(len(payload)) > maxReleaseMetadataBytes {
		return nil, fmt.Errorf(
			"github: read %s: %w: limit=%d",
			context,
			errReleaseMetadataTooLarge,
			maxReleaseMetadataBytes,
		)
	}
	return payload, nil
}

func drainAndCloseResponseBody(body io.ReadCloser, context string) error {
	if body == nil {
		return nil
	}
	var cleanupErr error
	if _, err := io.Copy(io.Discard, io.LimitReader(body, maxErrorBodyBytes)); err != nil {
		cleanupErr = fmt.Errorf("github: drain %s: %w", context, err)
	}
	if err := body.Close(); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("github: close %s: %w", context, err))
	}
	return cleanupErr
}
