package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const bundleActivationResourceKind = "bundle.activation"

type bundleActivationResourceSpec struct {
	ExtensionName string `json:"extension_name"`
}

func (g *BundleRepo) CountBundleActivationsForExtension(ctx context.Context, extensionName string) (int, error) {
	if err := g.checkReady(ctx, "count bundle activations for extension"); err != nil {
		return 0, err
	}

	trimmed := strings.TrimSpace(extensionName)
	if trimmed == "" {
		return 0, errors.New("store: extension name is required")
	}
	count, err := countBundleActivationResourcesForExtension(ctx, g.queries, trimmed)
	if err != nil {
		return 0, fmt.Errorf("store: count bundle activations for extension %q: %w", trimmed, err)
	}
	return count, nil
}

func countBundleActivationResourcesForExtension(
	ctx context.Context,
	queries *sqlcgen.Queries,
	extensionName string,
) (int, error) {
	rows, err := queries.ListBundleActivationResourceSpecs(ctx, bundleActivationResourceKind)
	if err != nil {
		return 0, err
	}

	count := 0
	trimmed := strings.TrimSpace(extensionName)
	for _, raw := range rows {
		var spec bundleActivationResourceSpec
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return 0, fmt.Errorf("decode bundle activation resource: %w", err)
		}
		if strings.TrimSpace(spec.ExtensionName) == trimmed {
			count++
		}
	}
	return count, nil
}
