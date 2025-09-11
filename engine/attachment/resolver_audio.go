package attachment

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func resolveAudio(ctx context.Context, a *AudioAttachment, cwd *core.PathCWD) (Resolved, error) {
	if a.Source == SourceURL {
		path, mime, err := httpDownloadToTemp(ctx, chooseURL(a.URL, a.URLs), 0)
		if err != nil {
			return nil, err
		}
		if !isAllowedByType(TypeAudio, mime) {
			(&resolvedFile{path: path, temp: true}).Cleanup()
			return nil, errMimeDenied(TypeAudio, mime)
		}
		logger.FromContext(ctx).Debug("Resolved URL attachment", "attachment_type", string(TypeAudio))
		return &resolvedFile{path: path, mime: mime, temp: true}, nil
	}
	return resolveLocalFile(ctx, cwd, choosePath(a.Path, a.Paths), TypeAudio)
}
