package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/windowmanager"
)

var (
	// ErrBundleInvalid reports invalid extension bundle resources.
	ErrBundleInvalid = errors.New("extension: invalid bundle")
)

// BundleSpec declares one team/product package shipped by an extension.
type BundleSpec struct {
	Name        string          `toml:"name"                  json:"name"`
	Description string          `toml:"description,omitempty" json:"description,omitempty"`
	Profiles    []BundleProfile `toml:"profiles"              json:"profiles"`
}

// BundleProfile declares one activatable resource profile for a bundle.
type BundleProfile struct {
	Name        string               `toml:"name"                  json:"name"`
	Description string               `toml:"description,omitempty" json:"description,omitempty"`
	Channels    BundleChannelsConfig `toml:"channels"              json:"channels"`
	Agents      []BundleAgent        `toml:"agents,omitempty"      json:"agents,omitempty"`
	Layouts     []BundleLayout       `toml:"layouts,omitempty"     json:"layouts,omitempty"`
	Jobs        []BundleJob          `toml:"jobs,omitempty"        json:"jobs,omitempty"`
	Triggers    []BundleTrigger      `toml:"triggers,omitempty"    json:"triggers,omitempty"`
	Bridges     []BundleBridgePreset `toml:"bridges,omitempty"     json:"bridges,omitempty"`
}

// BundleAgent declares one activation-scoped agent packaged by a bundle profile.
type BundleAgent struct {
	Path      string              `toml:"path,omitempty" json:"path,omitempty"`
	Agent     aghconfig.AgentDef  `toml:"-"              json:"agent"`
	Soul      *BundleAgentSidecar `toml:"-"              json:"soul,omitempty"`
	Heartbeat *BundleAgentSidecar `toml:"-"              json:"heartbeat,omitempty"`
}

// BundleAgentSidecar stores immutable packaged authored-context content.
type BundleAgentSidecar struct {
	SourcePath string `toml:"-" json:"source_path"`
	Body       string `toml:"-" json:"body"`
}

// BundleLayout stores one immutable declarative window layout loaded from the package.
type BundleLayout struct {
	Path   string                       `toml:"path,omitempty" json:"path"`
	Layout windowmanager.LayoutResource `toml:"-"              json:"layout"`
}

// BundleChannelsConfig declares the canonical channels packaged by a profile.
type BundleChannelsConfig struct {
	Primary string          `toml:"primary,omitempty" json:"primary,omitempty"`
	Items   []BundleChannel `toml:"items,omitempty"   json:"items,omitempty"`
}

// BundleChannel describes one declared network channel bundled by a profile.
type BundleChannel struct {
	Name        string `toml:"name"                  json:"name"`
	Description string `toml:"description,omitempty" json:"description,omitempty"`
}

// BundleJob declares one package-managed automation job template.
type BundleJob struct {
	Name      string                        `toml:"name"                 json:"name"`
	AgentName string                        `toml:"agent"                json:"agent"`
	Prompt    string                        `toml:"prompt"               json:"prompt"`
	Schedule  automationpkg.ScheduleSpec    `toml:"schedule"             json:"schedule"`
	Task      *automationpkg.JobTaskConfig  `toml:"task,omitempty"       json:"task,omitempty"`
	Enabled   bool                          `toml:"enabled"              json:"enabled"`
	Retry     automationpkg.RetryConfig     `toml:"retry,omitempty"      json:"retry"`
	FireLimit automationpkg.FireLimitConfig `toml:"fire_limit,omitempty" json:"fire_limit"`
}

// BundleTrigger declares one package-managed automation trigger template.
type BundleTrigger struct {
	Name         string                        `toml:"name"                    json:"name"`
	AgentName    string                        `toml:"agent"                   json:"agent"`
	Prompt       string                        `toml:"prompt"                  json:"prompt"`
	Event        string                        `toml:"event"                   json:"event"`
	Filter       map[string]string             `toml:"filter,omitempty"        json:"filter,omitempty"`
	Enabled      bool                          `toml:"enabled"                 json:"enabled"`
	Retry        automationpkg.RetryConfig     `toml:"retry,omitempty"         json:"retry"`
	FireLimit    automationpkg.FireLimitConfig `toml:"fire_limit,omitempty"    json:"fire_limit"`
	EndpointSlug string                        `toml:"endpoint_slug,omitempty" json:"endpoint_slug,omitempty"`
}

// BundleBridgePreset declares one package-managed bridge instance template.
type BundleBridgePreset struct {
	Name             string                   `toml:"name"                        json:"name"`
	ExtensionName    string                   `toml:"extension_name,omitempty"    json:"extension_name,omitempty"`
	Platform         string                   `toml:"platform,omitempty"          json:"platform,omitempty"`
	DisplayName      string                   `toml:"display_name"                json:"display_name"`
	RoutingPolicy    bridgepkg.RoutingPolicy  `toml:"routing_policy"              json:"routing_policy"`
	DeliveryDefaults json.RawMessage          `toml:"delivery_defaults,omitempty" json:"delivery_defaults,omitempty"`
	SecretSlots      []BundleBridgeSecretSlot `toml:"secret_slots,omitempty"      json:"secret_slots,omitempty"`
}

// BundleBridgeSecretSlot declares one required bridge secret binding.
type BundleBridgeSecretSlot struct {
	Name        string `toml:"name"                  json:"name"`
	Kind        string `toml:"kind"                  json:"kind"`
	Description string `toml:"description,omitempty" json:"description,omitempty"`
}

type bundleDocument struct {
	Bundle bundleRawSpec `toml:"bundle" json:"bundle"`

	Name        string             `toml:"name"                  json:"name"`
	Description string             `toml:"description,omitempty" json:"description,omitempty"`
	Profiles    []bundleRawProfile `toml:"profiles"              json:"profiles"`
}

type bundleRawSpec struct {
	Name        string             `toml:"name"                  json:"name"`
	Description string             `toml:"description,omitempty" json:"description,omitempty"`
	Profiles    []bundleRawProfile `toml:"profiles"              json:"profiles"`
}

type bundleRawProfile struct {
	Name        string               `toml:"name"                  json:"name"`
	Description string               `toml:"description,omitempty" json:"description,omitempty"`
	Channels    BundleChannelsConfig `toml:"channels"              json:"channels"`
	Agents      []bundleRawAgent     `toml:"agents,omitempty"      json:"agents,omitempty"`
	Layouts     []bundleRawLayout    `toml:"layouts,omitempty"     json:"layouts,omitempty"`
	Jobs        []bundleRawJob       `toml:"jobs,omitempty"        json:"jobs,omitempty"`
	Triggers    []bundleRawTrigger   `toml:"triggers,omitempty"    json:"triggers,omitempty"`
	Bridges     []BundleBridgePreset `toml:"bridges,omitempty"     json:"bridges,omitempty"`
}

type bundleRawAgent struct {
	Path string `toml:"path" json:"path"`
}

type bundleRawLayout struct {
	Path string `toml:"path" json:"path"`
}

type bundleRawJob struct {
	Name      string                        `toml:"name"                 json:"name"`
	AgentName string                        `toml:"agent"                json:"agent"`
	Prompt    string                        `toml:"prompt"               json:"prompt"`
	Schedule  automationpkg.ScheduleSpec    `toml:"schedule"             json:"schedule"`
	Task      *automationpkg.JobTaskConfig  `toml:"task,omitempty"       json:"task,omitempty"`
	Enabled   *bool                         `toml:"enabled,omitempty"    json:"enabled,omitempty"`
	Retry     automationpkg.RetryConfig     `toml:"retry,omitempty"      json:"retry"`
	FireLimit automationpkg.FireLimitConfig `toml:"fire_limit,omitempty" json:"fire_limit"`
}

type bundleRawTrigger struct {
	Name         string                        `toml:"name"                    json:"name"`
	AgentName    string                        `toml:"agent"                   json:"agent"`
	Prompt       string                        `toml:"prompt"                  json:"prompt"`
	Event        string                        `toml:"event"                   json:"event"`
	Filter       map[string]string             `toml:"filter,omitempty"        json:"filter,omitempty"`
	Enabled      *bool                         `toml:"enabled,omitempty"       json:"enabled,omitempty"`
	Retry        automationpkg.RetryConfig     `toml:"retry,omitempty"         json:"retry"`
	FireLimit    automationpkg.FireLimitConfig `toml:"fire_limit,omitempty"    json:"fire_limit"`
	EndpointSlug string                        `toml:"endpoint_slug,omitempty" json:"endpoint_slug,omitempty"`
}

// LoadBundleSpecs resolves and validates bundle resources declared by a manifest.
func LoadBundleSpecs(ctx context.Context, rootDir string, manifest *Manifest) ([]BundleSpec, error) {
	if manifest == nil || len(manifest.Resources.Bundles) == 0 {
		return nil, nil
	}
	if ctx == nil {
		return nil, errors.New("extension: bundle load context is required")
	}

	loaded := make(map[string]BundleSpec)
	for _, resourcePath := range manifest.Resources.Bundles {
		resourceRoot, err := resolveResourcePath(rootDir, resourcePath)
		if err != nil {
			return nil, err
		}
		files, err := collectBundleFiles(resourceRoot)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			spec, err := loadBundleSpecAtPath(ctx, rootDir, file)
			if err != nil {
				return nil, err
			}
			if err := spec.Validate(manifest); err != nil {
				return nil, err
			}
			key := bundleLookupKey(spec.Name)
			if _, exists := loaded[key]; exists {
				return nil, fmt.Errorf("%w: duplicate bundle %q", ErrBundleInvalid, spec.Name)
			}
			loaded[key] = spec
		}
	}

	bundles := make([]BundleSpec, 0, len(loaded))
	for _, key := range sortedKeys(loaded) {
		bundles = append(bundles, loaded[key])
	}
	return bundles, nil
}
