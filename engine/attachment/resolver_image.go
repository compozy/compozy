package attachment

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func resolveImage(ctx context.Context, a *ImageAttachment, cwd *core.PathCWD) (Resolved, error) {
	if a.Source == SourceURL {
		if a.URL != "" {
			logger.FromContext(ctx).Debug("Resolved URL attachment", "attachment_type", string(TypeImage))
			return &resolvedURL{url: a.URL}, nil
		}
		if len(a.URLs) == 1 {
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
