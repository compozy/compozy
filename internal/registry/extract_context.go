package registry

import (
	"context"
	"io"
	"mime"

	"github.com/compozy/compozy/internal/fileutil"
)

func extractArchiveWithContext(
	ctx context.Context,
	reader io.Reader,
	root *fileutil.Directory,
	limits extractLimits,
	contentType string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return err
	}
	bounded := &contextArchiveReader{ctx: ctx, reader: reader}
	if mediaType == TarContentType {
		return extractTar(bounded, root, limits)
	}
	return extractArchive(bounded, root, limits)
}

type contextArchiveReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextArchiveReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	read, err := r.reader.Read(buffer)
	if contextErr := r.ctx.Err(); contextErr != nil {
		return read, contextErr
	}
	return read, err
}
