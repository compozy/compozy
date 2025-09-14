package attachment

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func resolvePDF(ctx context.Context, a *PDFAttachment, cwd *core.PathCWD) (Resolved, error) {
	if a.Source == SourceURL {
		path, mime, err := httpDownloadToTemp(ctx, chooseURL(a.URL, a.URLs), 0)
		if err != nil {
			return nil, err
		}
		if !isAllowedByTypeCtx(ctx, TypePDF, mime) {
			(&resolvedFile{path: path, temp: true}).Cleanup()
			return nil, errMimeDenied(TypePDF, mime)
		}
		logger.FromContext(ctx).Debug("Resolved URL attachment", "attachment_type", string(TypePDF), "mime", mime)
		return &resolvedFile{path: path, mime: mime, temp: true}, nil
	}
	return resolveLocalFile(ctx, cwd, choosePath(a.Path, a.Paths), TypePDF)
}
