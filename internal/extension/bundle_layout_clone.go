package extensionpkg

import (
	"strings"

	"github.com/compozy/agh/internal/windowmanager"
)

func cloneBundleLayouts(values []BundleLayout) []BundleLayout {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]BundleLayout, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, BundleLayout{
			Path:   strings.TrimSpace(value.Path),
			Layout: windowmanager.CloneLayoutResource(value.Layout),
		})
	}
	return cloned
}
