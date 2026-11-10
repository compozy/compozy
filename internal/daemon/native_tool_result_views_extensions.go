package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

type compactExtension struct {
	MissingInputs               []string                              `json:"missing_inputs,omitempty"`
	Name                        string                                `json:"name"`
	Profile                     string                                `json:"profile"`
	WorkspaceID                 string                                `json:"workspace_id,omitempty"`
	Version                     string                                `json:"version"`
	Type                        string                                `json:"type"`
	Format                      string                                `json:"format"`
	Source                      string                                `json:"source"`
	Enabled                     bool                                  `json:"enabled"`
	State                       string                                `json:"state"`
	MissingEnv                  []string                              `json:"missing_env,omitempty"`
	GatewayRequirementDigest    string                                `json:"gateway_requirement_digest,omitempty"`
	GatewayConfirmationRequired bool                                  `json:"gateway_confirmation_required"`
	Health                      string                                `json:"health,omitempty"`
	HealthMessage               string                                `json:"health_message,omitempty"`
	LastError                   string                                `json:"last_error,omitempty"`
	FailureCode                 string                                `json:"failure_code,omitempty"`
	ConsecutiveFailures         int                                   `json:"consecutive_failures"`
	RestartBackoffMS            int64                                 `json:"restart_backoff_ms"`
	GenerationHash              string                                `json:"generation_hash,omitempty"`
	Dev                         bool                                  `json:"dev,omitzero"`
	OriginPath                  string                                `json:"origin_path,omitempty"`
	UpdateAvailable             bool                                  `json:"update_available"`
	RemoteVersion               string                                `json:"remote_version,omitempty"`
	DigestMatched               bool                                  `json:"digest_matched"`
	DaemonRunning               bool                                  `json:"daemon_running"`
	Trust                       *contract.ExtensionTrustReportPayload `json:"trust,omitzero"`
	Diagnostics                 []contract.DiagnosticItem             `json:"diagnostics,omitempty"`
}

func projectCompactExtension(p *contract.ExtensionPayload) compactExtension {
	return compactExtension{
		MissingInputs:               p.MissingInputs,
		Name:                        p.Name,
		Profile:                     p.Profile,
		WorkspaceID:                 p.WorkspaceID,
		Version:                     p.Version,
		Type:                        p.Type,
		Format:                      p.Format,
		Source:                      p.Source,
		Enabled:                     p.Enabled,
		State:                       p.State,
		MissingEnv:                  p.MissingEnv,
		GatewayRequirementDigest:    p.GatewayRequirementDigest,
		GatewayConfirmationRequired: p.GatewayConfirmationRequired,
		Health:                      p.Health,
		HealthMessage:               p.HealthMessage,
		LastError:                   p.LastError,
		FailureCode:                 p.FailureCode,
		ConsecutiveFailures:         p.ConsecutiveFailures,
		RestartBackoffMS:            p.RestartBackoffMS,
		GenerationHash:              p.GenerationHash,
		Dev:                         p.Dev,
		OriginPath:                  p.OriginPath,
		UpdateAvailable:             p.UpdateAvailable,
		RemoteVersion:               p.RemoteVersion,
		DigestMatched:               p.DigestMatched,
		DaemonRunning:               p.DaemonRunning,
		Trust:                       p.Trust,
		Diagnostics:                 p.Diagnostics,
	}
}

type extensionRecordResult struct {
	Extension   compactExtension               `json:"extension"`
	EnableError *contract.ExtensionEnableError `json:"enable_error,omitzero"`
}

func compactExtensionRecord(p *contract.InstallExtensionResponse) any {
	return extensionRecordResult{Extension: projectCompactExtension(&p.Extension), EnableError: p.EnableError}
}
func compactExtensionList(p []contract.ExtensionPayload) any {
	var items []compactExtension
	if p != nil {
		items = make([]compactExtension, len(p))
		for i := range p {
			items[i] = projectCompactExtension(&p[i])
		}
	}
	return map[string]any{"extensions": items}
}

func compactExtensionBuild(p extensionpkg.BuildResult) any {
	return map[string]any{"build": struct {
		GenerationDir  string `json:"generation_dir"`
		GenerationHash string `json:"generation_hash"`
		ManifestPath   string `json:"manifest_path"`
		Manifest       struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"manifest"`
	}{GenerationDir: p.GenerationDir, GenerationHash: p.GenerationHash, ManifestPath: p.ManifestPath,
		Manifest: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{p.Manifest.Name, p.Manifest.Version}}}
}
