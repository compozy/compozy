package daytona

import (
	"errors"
	"fmt"
	"net/http"
)

func mergeHTTPResponseCloseError(target *error, response *http.Response, operation string) {
	if target == nil || response == nil || response.Body == nil {
		return
	}
	if err := response.Body.Close(); err != nil {
		*target = errors.Join(
			*target,
			fmt.Errorf("sandbox/daytona: close %s response body: %w", operation, err),
		)
	}
}
