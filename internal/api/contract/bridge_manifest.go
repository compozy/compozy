package contract

import bridgepkg "github.com/compozy/agh/internal/bridges"

// SlackAppManifestResponse wraps one generated Slack app manifest.
type SlackAppManifestResponse struct {
	Manifest bridgepkg.SlackAppManifest `json:"manifest"`
}
