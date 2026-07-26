package bundles

import (
	"context"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

func canonicalizeBundleLayouts(ctx context.Context, bundle *extensionpkg.BundleSpec) error {
	if bundle == nil {
		return nil
	}
	codec, err := windowmanager.NewLayoutResourceCodec()
	if err != nil {
		return fmt.Errorf("bundles: create window layout codec: %w", err)
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	for profileIndex := range bundle.Profiles {
		profile := &bundle.Profiles[profileIndex]
		for layoutIndex := range profile.Layouts {
			layout := &profile.Layouts[layoutIndex]
			encoded, err := codec.Encode(layout.Layout)
			if err != nil {
				return fmt.Errorf(
					"bundles: encode layout %q in profile %q: %w",
					strings.TrimSpace(layout.Layout.ID),
					strings.TrimSpace(profile.Name),
					err,
				)
			}
			validated, err := codec.DecodeAndValidate(ctx, scope, encoded)
			if err != nil {
				return fmt.Errorf(
					"bundles: validate layout %q in profile %q: %w",
					strings.TrimSpace(layout.Layout.ID),
					strings.TrimSpace(profile.Name),
					err,
				)
			}
			layout.Path = strings.TrimSpace(layout.Path)
			layout.Layout = windowmanager.CloneLayoutResource(validated)
		}
	}
	return nil
}
