package extensionpkg

import (
	"context"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/windowmanager"
)

// LoadLayoutResources loads globally-scoped extension window layouts.
func LoadLayoutResources(rootDir string, paths []string) ([]windowmanager.LayoutResource, error) {
	files, err := collectDeclaredResourceFiles(rootDir, paths, ".json", "layout resource")
	if err != nil {
		return nil, err
	}
	codec, err := windowmanager.NewLayoutResourceCodec()
	if err != nil {
		return nil, fmt.Errorf("extension: create layout resource codec: %w", err)
	}
	byID := make(map[string]windowmanager.LayoutResource)
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("extension: read layout resource %q: %w", file, err)
		}
		layout, err := codec.DecodeAndValidate(
			context.Background(),
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
			return nil, fmt.Errorf("%w: duplicate layout resource %q", ErrManifestInvalid, layout.ID)
		}
		byID[layout.ID] = windowmanager.CloneLayoutResource(layout)
	}
	result := make([]windowmanager.LayoutResource, 0, len(byID))
	for _, id := range sortedKeys(byID) {
		result = append(result, windowmanager.CloneLayoutResource(byID[id]))
	}
	return result, nil
}
