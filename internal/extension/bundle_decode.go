package extensionpkg

import (
	"context"
	"encoding/json"

	"fmt"

	"os"
	"path/filepath"

	"strings"

	"github.com/BurntSushi/toml"
)

func loadBundleSpecAtPath(ctx context.Context, rootDir string, path string) (BundleSpec, error) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".toml":
		return loadBundleTOML(ctx, rootDir, path)
	case ".json":
		return loadBundleJSON(ctx, rootDir, path)
	default:
		return BundleSpec{}, fmt.Errorf("%w: unsupported bundle path %q", ErrBundleInvalid, path)
	}
}

func bundleLookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func loadBundleTOML(ctx context.Context, rootDir string, path string) (BundleSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BundleSpec{}, fmt.Errorf("extension: read bundle %q: %w", path, err)
	}

	var doc bundleDocument
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return BundleSpec{}, fmt.Errorf("extension: decode bundle %q: %w", path, err)
	}
	return doc.toBundleSpec(ctx, bundleAgentRoot(rootDir, path))
}

func loadBundleJSON(ctx context.Context, rootDir string, path string) (BundleSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BundleSpec{}, fmt.Errorf("extension: read bundle %q: %w", path, err)
	}

	var doc bundleDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return BundleSpec{}, fmt.Errorf("extension: decode bundle %q: %w", path, err)
	}
	return doc.toBundleSpec(ctx, bundleAgentRoot(rootDir, path))
}

func bundleAgentRoot(rootDir string, bundlePath string) string {
	if strings.TrimSpace(rootDir) != "" {
		return rootDir
	}
	return filepath.Dir(strings.TrimSpace(bundlePath))
}

func (d bundleDocument) toBundleSpec(ctx context.Context, rootDir string) (BundleSpec, error) {
	name, err := mergeManifestValue("bundle.name", d.Name, d.Bundle.Name)
	if err != nil {
		return BundleSpec{}, err
	}
	description, err := mergeManifestValue("bundle.description", d.Description, d.Bundle.Description)
	if err != nil {
		return BundleSpec{}, err
	}

	profiles := d.Profiles
	if len(profiles) == 0 {
		profiles = d.Bundle.Profiles
	}
	if len(d.Profiles) > 0 && len(d.Bundle.Profiles) > 0 {
		return BundleSpec{}, fmt.Errorf("%w: conflicting root and bundle profiles", ErrBundleInvalid)
	}

	spec := BundleSpec{
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Profiles:    make([]BundleProfile, 0, len(profiles)),
	}
	for _, profile := range profiles {
		bundleProfile, err := profile.toBundleProfile(ctx, rootDir)
		if err != nil {
			return BundleSpec{}, err
		}
		spec.Profiles = append(spec.Profiles, bundleProfile)
	}
	return spec, nil
}

func (p bundleRawProfile) toBundleProfile(ctx context.Context, rootDir string) (BundleProfile, error) {
	profile := BundleProfile{
		Name:        strings.TrimSpace(p.Name),
		Description: strings.TrimSpace(p.Description),
		Channels: BundleChannelsConfig{
			Primary: strings.TrimSpace(p.Channels.Primary),
			Items:   normalizeBundleChannels(p.Channels.Items),
		},
		Agents:   make([]BundleAgent, 0, len(p.Agents)),
		Layouts:  make([]BundleLayout, 0, len(p.Layouts)),
		Jobs:     make([]BundleJob, 0, len(p.Jobs)),
		Triggers: make([]BundleTrigger, 0, len(p.Triggers)),
		Bridges:  normalizeBundleBridges(p.Bridges),
	}
	for _, agent := range p.Agents {
		loaded, err := loadBundleAgent(ctx, rootDir, agent.Path)
		if err != nil {
			return BundleProfile{}, err
		}
		profile.Agents = append(profile.Agents, loaded)
	}
	for _, layout := range p.Layouts {
		loaded, err := loadBundleLayout(ctx, rootDir, layout.Path)
		if err != nil {
			return BundleProfile{}, err
		}
		profile.Layouts = append(profile.Layouts, loaded)
	}
	for _, job := range p.Jobs {
		profile.Jobs = append(profile.Jobs, job.toBundleJob())
	}
	for _, trigger := range p.Triggers {
		profile.Triggers = append(profile.Triggers, trigger.toBundleTrigger())
	}
	return profile, nil
}

func (j bundleRawJob) toBundleJob() BundleJob {
	job := BundleJob{
		Name:      strings.TrimSpace(j.Name),
		AgentName: strings.TrimSpace(j.AgentName),
		Prompt:    strings.TrimSpace(j.Prompt),
		Schedule:  j.Schedule,
		Task:      cloneBundleTaskConfig(j.Task),
		Enabled:   true,
		Retry:     j.Retry,
		FireLimit: j.FireLimit,
	}
	if j.Enabled != nil {
		job.Enabled = *j.Enabled
	}
	return job
}

func (t bundleRawTrigger) toBundleTrigger() BundleTrigger {
	trigger := BundleTrigger{
		Name:         strings.TrimSpace(t.Name),
		AgentName:    strings.TrimSpace(t.AgentName),
		Prompt:       strings.TrimSpace(t.Prompt),
		Event:        strings.TrimSpace(t.Event),
		Filter:       cloneStringMap(t.Filter),
		Enabled:      true,
		Retry:        t.Retry,
		FireLimit:    t.FireLimit,
		EndpointSlug: strings.TrimSpace(t.EndpointSlug),
	}
	if t.Enabled != nil {
		trigger.Enabled = *t.Enabled
	}
	return trigger
}
