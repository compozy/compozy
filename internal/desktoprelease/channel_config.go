package desktoprelease

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ChannelConfigRequest struct {
	Version           string
	Channel           string
	PublicKey         string
	PreviousPublicKey string
	AzureEndpoint     string
	OutputPath        string
}

func WriteChannelConfig(request ChannelConfigRequest) error {
	if err := ValidateVersion(request.Version); err != nil {
		return err
	}
	if err := ValidateDesktopChannel(request.Channel); err != nil {
		return err
	}
	if err := validatePublicKey(request.PublicKey); err != nil {
		return err
	}
	if request.PreviousPublicKey != "" {
		if err := validatePublicKey(request.PreviousPublicKey); err != nil {
			return fmt.Errorf("desktop release: previous public key: %w", err)
		}
	}
	endpoint := DistributionOrigin + "/desktop/" + request.Channel + "/latest.json"
	config := map[string]any{
		"version": request.Version,
		"plugins": map[string]any{
			"updater": map[string]any{
				"endpoints":      []string{endpoint},
				"pubkey":         request.PublicKey,
				"previousPubkey": request.PreviousPublicKey,
				"windows": map[string]any{
					"installMode": "passive",
				},
			},
		},
	}
	if request.AzureEndpoint != "" {
		azure, err := validateAzureEndpoint(request.AzureEndpoint)
		if err != nil {
			return err
		}
		config["bundle"] = map[string]any{
			"windows": map[string]any{
				"signCommand": map[string]any{
					"cmd": "artifact-signing-cli",
					"args": []string{
						"%1",
						"--endpoint",
						azure,
						"--description",
						"CompozyOS",
					},
				},
			},
		}
	}
	contents, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("desktop release: encode channel config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(request.OutputPath), 0o755); err != nil {
		return fmt.Errorf("desktop release: create channel config directory: %w", err)
	}
	if err := os.WriteFile(request.OutputPath, contents, 0o600); err != nil {
		return fmt.Errorf("desktop release: write channel config: %w", err)
	}
	return nil
}

func validatePublicKey(encoded string) error {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return fmt.Errorf("desktop release: updater public key must be base64 content: %w", err)
	}
	if !strings.Contains(string(decoded), "minisign public key") {
		return fmt.Errorf("desktop release: updater public key does not decode to a minisign public key")
	}
	return nil
}

func validateAzureEndpoint(raw string) (string, error) {
	parsed, err := validateHTTPSURL(raw)
	if err != nil {
		return "", fmt.Errorf("desktop release: Azure Artifact Signing endpoint: %w", err)
	}
	return parsed, nil
}
