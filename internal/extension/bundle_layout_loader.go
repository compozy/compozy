package extensionpkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

func loadBundleLayout(ctx context.Context, rootDir string, path string) (BundleLayout, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return BundleLayout{}, fmt.Errorf("%w: profile layout path is required", ErrBundleInvalid)
	}
	if filepath.IsAbs(trimmed) {
		return BundleLayout{}, fmt.Errorf(
			"%w: profile layout path %q must be relative",
			ErrBundleInvalid,
			trimmed,
		)
	}
	if !strings.EqualFold(filepath.Ext(trimmed), ".json") {
		return BundleLayout{}, fmt.Errorf(
			"%w: profile layout path %q must be a JSON file",
			ErrBundleInvalid,
			trimmed,
		)
	}

	resolved, err := resolveBundlePathWithinRoot(rootDir, trimmed, "profile layout")
	if err != nil {
		return BundleLayout{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return BundleLayout{}, fmt.Errorf("extension: stat profile layout %q: %w", trimmed, err)
	}
	if !info.Mode().IsRegular() {
		return BundleLayout{}, fmt.Errorf(
			"%w: profile layout path %q must be a regular file",
			ErrBundleInvalid,
			trimmed,
		)
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return BundleLayout{}, fmt.Errorf("extension: read profile layout %q: %w", trimmed, err)
	}
	codec, err := windowmanager.NewLayoutResourceCodec()
	if err != nil {
		return BundleLayout{}, fmt.Errorf("extension: create profile layout codec: %w", err)
	}
	layout, err := codec.DecodeAndValidate(
		ctx,
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		raw,
	)
	if err != nil {
		return BundleLayout{}, fmt.Errorf(
			"%w: validate profile layout %q: %w",
			ErrBundleInvalid,
			trimmed,
			err,
		)
	}
	return BundleLayout{
		Path:   filepath.ToSlash(filepath.Clean(trimmed)),
		Layout: windowmanager.CloneLayoutResource(layout),
	}, nil
}
