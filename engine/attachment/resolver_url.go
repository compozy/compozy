package attachment

import (
	"context"
	"fmt"
)

func resolveURL(_ context.Context, a *URLAttachment) (Resolved, error) {
	s, err := validateHTTPURL(a.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url attachment: %w", err)
	}
	return &resolvedURL{url: s}, nil
}
