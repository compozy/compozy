package registry

import (
	"context"
	"errors"
	"io"
	"mime"

	"github.com/compozy/compozy/internal/fileutil"
)

// ExtractArchive materializes an archive into a caller-owned directory under explicit resource limits.
func ExtractArchive(
	ctx context.Context,
	reader io.Reader,
	root *fileutil.Directory,
	limits ExtractionLimits,
	contentType string,
) error {
	if ctx == nil || reader == nil {
		return errors.New("registry: archive context and reader are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateDownloadContentType(contentType); err != nil {
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
