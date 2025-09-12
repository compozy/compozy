package attachment

import (
	"context"

	"github.com/compozy/compozy/engine/core"
)

func resolveFile(ctx context.Context, a *FileAttachment, cwd *core.PathCWD) (Resolved, error) {
	return resolveLocalFile(ctx, cwd, a.Path, TypeFile)
}
