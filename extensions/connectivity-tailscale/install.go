package connectivitytailscale

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

const Name = "connectivity-tailscale"

// EnsureManagedInstall enrolls the first-party provider without replacing an operator install.
func EnsureManagedInstall(homePaths compozyconfig.HomePaths, registry *extensionpkg.Registry) error {
	return extensionpkg.InstallBundledExtension(homePaths, registry, extensionpkg.BundledInstallSpec{
		Name: Name,
		FS:   FS(),
	})
}
