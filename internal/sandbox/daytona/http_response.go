package daytona

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxDaytonaResponseDrainBytes = int64(64 << 10)

func mergeHTTPResponseCloseError(target *error, response *http.Response, operation string) {
	if target == nil || response == nil || response.Body == nil {
		return
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, maxDaytonaResponseDrainBytes)); err != nil {
		*target = errors.Join(
			*target,
			fmt.Errorf("sandbox/daytona: drain %s response body: %w", operation, err),
		)
	}
	if err := response.Body.Close(); err != nil {
		*target = errors.Join(
			*target,
			fmt.Errorf("sandbox/daytona: close %s response body: %w", operation, err),
		)
	}
}
