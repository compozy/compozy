package extensionpkg

import (
	"os"
	"time"
)

type extensionManifestFileMetadata struct {
	size        int64
	modTime     time.Time
	changeToken string
	statError   string
	reliable    bool
}

func newExtensionManifestFileMetadata(info os.FileInfo) extensionManifestFileMetadata {
	changeToken, reliable := extensionManifestPlatformChangeToken(info)
	return extensionManifestFileMetadata{
		size:        info.Size(),
		modTime:     info.ModTime(),
		changeToken: changeToken,
		reliable:    reliable,
	}
}

func unavailableExtensionManifestFileMetadata(err error) extensionManifestFileMetadata {
	if err == nil {
		return extensionManifestFileMetadata{}
	}
	return extensionManifestFileMetadata{statError: err.Error(), reliable: true}
}

func (m extensionManifestFileMetadata) Equal(other extensionManifestFileMetadata) bool {
	if m.reliable != other.reliable {
		return false
	}
	if m.statError != other.statError {
		return false
	}
	if m.statError != "" {
		return true
	}
	return m.reliable &&
		m.size == other.size &&
		m.modTime.Equal(other.modTime) &&
		m.changeToken == other.changeToken
}
