package attachment

import "context"

func resolveURL(_ context.Context, a *URLAttachment) (Resolved, error) {
	return &resolvedURL{url: a.URL}, nil
}
