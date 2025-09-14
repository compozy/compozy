package attachment

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func resolveAudio(ctx context.Context, a *AudioAttachment, cwd *core.PathCWD) (Resolved, error) {
	if a.Source == SourceURL {
		u := chooseURL(a.URL, a.URLs)
		if u == "" {
			return nil, fmt.Errorf("audio URL is empty")
		}
		cfg := appconfig.FromContext(ctx)
		maxBytes := int64(0)
		if cfg != nil && cfg.Attachments.MaxDownloadSizeBytes > 0 {
			maxBytes = cfg.Attachments.MaxDownloadSizeBytes
		}
		dctx := ctx
		if cfg != nil && cfg.Attachments.DownloadTimeout > 0 {
			var cancel context.CancelFunc
			dctx, cancel = context.WithTimeout(ctx, cfg.Attachments.DownloadTimeout)
			defer cancel()
		}
		path, mime, err := httpDownloadToTemp(dctx, u, maxBytes)
		if err != nil {
			return nil, err
		}
		if !isAllowedByTypeCtx(ctx, TypeAudio, mime) {
			(&resolvedFile{path: path, temp: true}).Cleanup()
			return nil, errMimeDenied(TypeAudio, mime)
		}
		logger.FromContext(ctx).Debug("Resolved URL attachment", "attachment_type", string(TypeAudio))
		return &resolvedFile{path: path, mime: mime, temp: true}, nil
	}
	return resolveLocalFile(ctx, cwd, choosePath(a.Path, a.Paths), TypeAudio)
}
