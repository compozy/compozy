package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/windowmanager"
)

// LoadLayoutResources loads globally-scoped extension window layouts.
func LoadLayoutResources(
	ctx context.Context,
	rootDir string,
	paths []string,
) ([]windowmanager.LayoutResource, error) {
	if ctx == nil {
		return nil, errors.New("extension: layout resource context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, err := collectDeclaredResourceFiles(rootDir, paths, ".json", "layout resource")
	if err != nil {
		return nil, err
	}
	codec, err := windowmanager.NewLayoutResourceCodec()
	if err != nil {
		return nil, fmt.Errorf("extension: create layout resource codec: %w", err)
	}
	byID := make(map[string]windowmanager.LayoutResource)
	origins := make(map[string]string)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("extension: read layout resource %q: %w", file, err)
		}
		layout, err := codec.DecodeAndValidate(
			ctx,
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			body,
		)
		if err != nil {
			return nil, wrapResourceValidationError(
				file,
				fmt.Errorf("%w: validate layout resource: %w", ErrManifestInvalid, err),
			)
		}
		if _, exists := byID[layout.ID]; exists {
			return nil, wrapResourceValidationError(file, fmt.Errorf(
				"%w: duplicate layout resource %q also declared in %q",
				ErrManifestInvalid,
				layout.ID,
				origins[layout.ID],
			))
		}
		byID[layout.ID] = windowmanager.CloneLayoutResource(layout)
		origins[layout.ID] = file
	}
	result := make([]windowmanager.LayoutResource, 0, len(byID))
	for _, id := range sortedKeys(byID) {
		result = append(result, windowmanager.CloneLayoutResource(byID[id]))
	}
	return result, nil
}
