package opendesign

import (
	"embed"
	"io/fs"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

const Name = "open-design"

//go:generate bun scripts/build-lint.ts
//go:generate go run scripts/generate-manifest.go
//go:generate bunx oxfmt extension.json

//go:embed extension.json agents loops all:skills SOURCES.md LICENSES.md
var bundledFS embed.FS

func FS() fs.FS { return bundledFS }

func EnsureManagedInstall(paths compozyconfig.HomePaths, registry *extensionpkg.Registry) error {
	return extensionpkg.InstallBundledExtension(paths, registry, extensionpkg.BundledInstallSpec{Name: Name, FS: FS()})
}
