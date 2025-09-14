package attachment

import (
	"context"
	"fmt"
)

func resolveURL(ctx context.Context, a *URLAttachment) (Resolved, error) {
	s, err := validateHTTPURL(ctx, a.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url attachment: %w", err)
	}
	return &resolvedURL{url: s}, nil
}
