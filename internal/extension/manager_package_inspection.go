package extensionpkg

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// InspectPackageResources loads one installed package's declarative resources
// without registering it, starting a subprocess, or mutating runtime state.
func (m *Manager) InspectPackageResources(ctx context.Context, name string) (*Extension, error) {
	if m == nil {
		return nil, ErrManagerRequired
	}
	if ctx == nil {
		return nil, errors.New("extension: package inspection context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := m.registry.Get(strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Dir(strings.TrimSpace(info.ManifestPath))
	manifest, err := LoadManifest(rootDir)
	if err != nil {
		return nil, err
	}
	managed := &managedExtension{info: *info, rootDir: rootDir, manifest: manifest}
	loaded, err := m.loadDeclarativeResources(ctx, managed)
	if err != nil {
		return nil, err
	}
	loaded.apply(managed)
	return m.cloneExtension(managed), nil
}
