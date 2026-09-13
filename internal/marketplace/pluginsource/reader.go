package pluginsource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

var ErrSourceUnreachable = errors.New("source_unreachable")

type NotMarketplaceError struct {
	Checked []string
}

var _ error = &NotMarketplaceError{}

func (e *NotMarketplaceError) Error() string {
	return fmt.Sprintf("%s: checked %v", ErrNotMarketplace, e.Checked)
}

func (e *NotMarketplaceError) Unwrap() error {
	return ErrNotMarketplace
}

// ReadDirectory reads a marketplace document from an already acquired checkout or local source.
func ReadDirectory(ctx context.Context, root string) (document Document, err error) {
	if ctx == nil {
		return Document{}, errors.New("pluginsource: context is required")
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	directory, err := fileutil.OpenDirectory(root)
	if err != nil {
		return Document{}, fmt.Errorf("%w: open marketplace folder: %w", ErrSourceUnreachable, err)
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	document, err = readDirectoryDocument(ctx, directory)
	if err == nil {
		document.Path = "marketplace.json"
		return document, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Document{}, err
	}
	child, err := directory.OpenDirectory(".claude-plugin")
	if errors.Is(err, os.ErrNotExist) {
		return Document{}, missingMarketplaceDocument()
	}
	if err != nil {
		return Document{}, fmt.Errorf("%w: open marketplace document directory: %w", ErrSourceUnreachable, err)
	}
	defer func() { err = errors.Join(err, child.Close()) }()
	document, err = readDirectoryDocument(ctx, child)
	if errors.Is(err, os.ErrNotExist) {
		return Document{}, missingMarketplaceDocument()
	}
	if err == nil {
		document.Path = ".claude-plugin/marketplace.json"
	}
	return document, err
}

func readDirectoryDocument(ctx context.Context, directory *fileutil.Directory) (document Document, err error) {
	file, err := directory.OpenRegularFile("marketplace.json")
	if err != nil {
		return Document{}, fmt.Errorf("%w: open marketplace document: %w", ErrSourceUnreachable, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	raw, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("%w: read marketplace document: %w", ErrSourceUnreachable, err)
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	return DecodeDocument(raw)
}

func missingMarketplaceDocument() error {
	return &NotMarketplaceError{Checked: []string{"marketplace.json", ".claude-plugin/marketplace.json"}}
}
