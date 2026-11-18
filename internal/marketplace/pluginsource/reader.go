package pluginsource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

var ErrSourceUnreachable = errors.New("source_unreachable")

const (
	rootDocumentPath   = "marketplace.json"
	clientDocumentPath = ".claude-plugin/marketplace.json"
	codexDocumentPath  = ".agents/plugins/marketplace.json"
)

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
	for _, location := range []string{rootDocumentPath, clientDocumentPath, codexDocumentPath} {
		document, err = readDocumentAt(ctx, directory, location)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err == nil {
			document.Path = location
		}
		return document, err
	}
	return Document{}, missingMarketplaceDocument()
}

func readDocumentAt(
	ctx context.Context,
	directory *fileutil.Directory,
	location string,
) (document Document, err error) {
	name, rest, nested := strings.Cut(location, "/")
	if !nested {
		return readDirectoryDocument(ctx, directory)
	}
	child, err := directory.OpenDirectory(name)
	if err != nil {
		return Document{}, fmt.Errorf("%w: open marketplace document directory: %w", ErrSourceUnreachable, err)
	}
	defer func() { err = errors.Join(err, child.Close()) }()
	return readDocumentAt(ctx, child, rest)
}

func readDirectoryDocument(ctx context.Context, directory *fileutil.Directory) (document Document, err error) {
	file, err := directory.OpenRegularFile(rootDocumentPath)
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
	return &NotMarketplaceError{Checked: []string{rootDocumentPath, clientDocumentPath, codexDocumentPath}}
}
