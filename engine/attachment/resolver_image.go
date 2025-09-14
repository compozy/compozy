package attachment

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func resolveImage(ctx context.Context, a *ImageAttachment, cwd *core.PathCWD) (Resolved, error) {
	if a.Source == SourceURL {
		if a.URL != "" {
			if !strings.HasPrefix(a.URL, "http://") && !strings.HasPrefix(a.URL, "https://") {
				return nil, fmt.Errorf("unsupported URL scheme")
			}
			logger.FromContext(ctx).Debug("Resolved URL attachment", "attachment_type", string(TypeImage))
			return &resolvedURL{url: a.URL}, nil
		}
		if len(a.URLs) == 1 {
			if !strings.HasPrefix(a.URLs[0], "http://") && !strings.HasPrefix(a.URLs[0], "https://") {
				return nil, fmt.Errorf("unsupported URL scheme")
			}
			logger.FromContext(ctx).Debug("Resolved URL attachment", "attachment_type", string(TypeImage))
			return &resolvedURL{url: a.URLs[0]}, nil
		}
		return nil, nil
	}
	if a.Path != "" {
		return resolveLocalFile(ctx, cwd, a.Path, TypeImage)
	}
	if len(a.Paths) == 1 {
		return resolveLocalFile(ctx, cwd, a.Paths[0], TypeImage)
	}
	return nil, nil
}
