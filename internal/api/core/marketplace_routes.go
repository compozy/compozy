package core

import "net/url"

const marketplaceExtensionsInstalledPath = "/marketplace/extensions?tab=installed"

func marketplaceBundleActivationPath(activationID string) string {
	return "/marketplace/bundles/activations/" + url.PathEscape(activationID)
}
