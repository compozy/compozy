//go:build !linux && !darwin

package extensionpkg

import "os"

// Platforms without an OS change token rehash a manifest after every successful stat.
func extensionManifestPlatformChangeToken(os.FileInfo) (string, bool) {
	return "", false
}
