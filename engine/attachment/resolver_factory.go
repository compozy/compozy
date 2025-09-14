package attachment

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// Resolve delegates attachment resolution to the appropriate per-type resolver.
func Resolve(ctx context.Context, att Attachment, cwd *core.PathCWD) (Resolved, error) {
	switch a := att.(type) {
	case *ImageAttachment:
		return resolveImage(ctx, a, cwd)
	case *PDFAttachment:
		return resolvePDF(ctx, a, cwd)
	case *AudioAttachment:
		return resolveAudio(ctx, a, cwd)
	case *VideoAttachment:
		return resolveVideo(ctx, a, cwd)
	case *URLAttachment:
		return resolveURL(ctx, a)
	case *FileAttachment:
		return resolveFile(ctx, a, cwd)
	default:
		return nil, fmt.Errorf("unsupported attachment type: %T", att)
	}
}
