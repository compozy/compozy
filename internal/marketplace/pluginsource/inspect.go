package pluginsource

import (
	"context"
	"errors"
	"os"

	"github.com/compozy/compozy/internal/registry"
)

// Inspect lends a verified package tree to the install loader, then removes that private tree.
func (r *Resolver) Inspect(
	ctx context.Context,
	digest string,
	inspect func(context.Context, string) error,
) (err error) {
	if inspect == nil {
		return errors.New("pluginsource: package inspector is required")
	}
	reader, err := r.Cache.Open(ctx, digest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	root, err := os.MkdirTemp(r.Sources.TempDir, "compozy-marketplace-load-*")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(root)) }()
	if err := extractSnapshot(
		ctx,
		root,
		&registry.DownloadResult{Reader: reader, ContentType: registry.TarContentType},
	); err != nil {
		return err
	}
	return inspect(ctx, root)
}
