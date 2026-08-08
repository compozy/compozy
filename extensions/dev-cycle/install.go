package devcycle

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

const Name = "dev-cycle"

// EnsureManagedInstall enrolls the first-party extension without replacing an operator install.
func EnsureManagedInstall(homePaths compozyconfig.HomePaths, registry *extensionpkg.Registry) error {
	return extensionpkg.InstallBundledExtension(homePaths, registry, extensionpkg.BundledInstallSpec{
		Name: Name,
		FS:   FS(),
	})
}
