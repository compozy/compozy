package attachment

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// resolveFile resolves a local file attachment relative to the provided CWD.
// It requires a non-nil, non-empty CWD; callers must set it accordingly.
func resolveFile(ctx context.Context, a *FileAttachment, cwd *core.PathCWD) (Resolved, error) {
	if cwd == nil || cwd.Path == "" {
		return nil, fmt.Errorf("current working directory not set for file attachment: %q", a.Path)
	}
	return resolveLocalFile(ctx, cwd, a.Path, TypeFile)
}
