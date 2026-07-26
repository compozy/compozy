package extensionpkg

import (
	"fmt"

	"strings"
)

// Validate ensures the bundle spec is internally consistent for the owning manifest.
func (b BundleSpec) Validate(manifest *Manifest) error {
	name := strings.TrimSpace(b.Name)
	if name == "" {
		return fmt.Errorf("%w: bundle.name is required", ErrBundleInvalid)
	}
	if len(b.Profiles) == 0 {
		return fmt.Errorf("%w: bundle %q must declare at least one profile", ErrBundleInvalid, name)
	}

	seenProfiles := make(map[string]struct{}, len(b.Profiles))
	for idx, profile := range b.Profiles {
		profileName := strings.TrimSpace(profile.Name)
		if profileName == "" {
			return fmt.Errorf("%w: bundle %q profile[%d].name is required", ErrBundleInvalid, name, idx)
		}
		profileKey := bundleLookupKey(profileName)
		if _, exists := seenProfiles[profileKey]; exists {
			return fmt.Errorf("%w: bundle %q profile %q is duplicated", ErrBundleInvalid, name, profileName)
		}
		seenProfiles[profileKey] = struct{}{}
		if err := profile.Validate(name, manifest); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures one bundle profile is internally consistent.
func (p BundleProfile) Validate(bundleName string, manifest *Manifest) error {
	channelNames, err := p.validateChannels(bundleName)
	if err != nil {
		return err
	}
	if err := p.validateAgents(bundleName); err != nil {
		return err
	}
	if err := p.validateLayouts(bundleName); err != nil {
		return err
	}
	if err := p.validateJobs(bundleName, channelNames); err != nil {
		return err
	}
	if err := p.validateTriggers(bundleName); err != nil {
		return err
	}
	return p.validateBridges(bundleName, manifest)
}

func (p BundleProfile) validateLayouts(bundleName string) error {
	seenLayouts := make(map[string]struct{}, len(p.Layouts))
	for idx, layout := range p.Layouts {
		path := strings.TrimSpace(layout.Path)
		if path == "" {
			return fmt.Errorf(
				"%w: bundle %q profile %q layouts[%d].path is required",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				idx,
			)
		}
		id := strings.TrimSpace(layout.Layout.ID)
		if id == "" {
			return fmt.Errorf(
				"%w: bundle %q profile %q layout %q id is required",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				path,
			)
		}
		key := bundleLookupKey(id)
		if _, exists := seenLayouts[key]; exists {
			return fmt.Errorf(
				"%w: bundle %q profile %q layout %q is duplicated",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				id,
			)
		}
		seenLayouts[key] = struct{}{}
	}
	return nil
}

func (p BundleProfile) validateChannels(bundleName string) (map[string]struct{}, error) {
	channelNames := make(map[string]struct{}, len(p.Channels.Items))
	for idx, item := range p.Channels.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, fmt.Errorf(
				"%w: bundle %q profile %q channels[%d].name is required",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				idx,
			)
		}
		if _, exists := channelNames[name]; exists {
			return nil, fmt.Errorf(
				"%w: bundle %q profile %q channel %q is duplicated",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				name,
			)
		}
		channelNames[name] = struct{}{}
	}

	primary := strings.TrimSpace(p.Channels.Primary)
	switch {
	case len(channelNames) > 0 && primary == "":
		return nil, fmt.Errorf(
			"%w: bundle %q profile %q must declare channels.primary",
			ErrBundleInvalid,
			bundleName,
			p.Name,
		)
	case primary != "":
		if _, ok := channelNames[primary]; !ok {
			return nil, fmt.Errorf(
				"%w: bundle %q profile %q primary channel %q is not declared",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				primary,
			)
		}
	}

	return channelNames, nil
}

func (p BundleProfile) validateJobs(bundleName string, channelNames map[string]struct{}) error {
	seenJobs := make(map[string]struct{}, len(p.Jobs))
	for _, job := range p.Jobs {
		jobName := strings.TrimSpace(job.Name)
		if jobName == "" {
			return fmt.Errorf("%w: bundle %q profile %q job.name is required", ErrBundleInvalid, bundleName, p.Name)
		}
		if _, exists := seenJobs[jobName]; exists {
			return fmt.Errorf(
				"%w: bundle %q profile %q job %q is duplicated",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				jobName,
			)
		}
		seenJobs[jobName] = struct{}{}
		if err := job.Validate(bundleName, p.Name, channelNames); err != nil {
			return err
		}
	}
	return nil
}

func (p BundleProfile) validateTriggers(bundleName string) error {
	seenTriggers := make(map[string]struct{}, len(p.Triggers))
	for _, trigger := range p.Triggers {
		triggerName := strings.TrimSpace(trigger.Name)
		if triggerName == "" {
			return fmt.Errorf("%w: bundle %q profile %q trigger.name is required", ErrBundleInvalid, bundleName, p.Name)
		}
		if _, exists := seenTriggers[triggerName]; exists {
			return fmt.Errorf(
				"%w: bundle %q profile %q trigger %q is duplicated",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				triggerName,
			)
		}
		seenTriggers[triggerName] = struct{}{}
		if err := trigger.Validate(bundleName, p.Name); err != nil {
			return err
		}
	}
	return nil
}

func (p BundleProfile) validateBridges(bundleName string, manifest *Manifest) error {
	seenBridges := make(map[string]struct{}, len(p.Bridges))
	for _, bridge := range p.Bridges {
		bridgeName := strings.TrimSpace(bridge.Name)
		if bridgeName == "" {
			return fmt.Errorf("%w: bundle %q profile %q bridge.name is required", ErrBundleInvalid, bundleName, p.Name)
		}
		if _, exists := seenBridges[bridgeName]; exists {
			return fmt.Errorf(
				"%w: bundle %q profile %q bridge %q is duplicated",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				bridgeName,
			)
		}
		seenBridges[bridgeName] = struct{}{}
		if err := bridge.Validate(bundleName, p.Name, manifest); err != nil {
			return err
		}
	}
	return nil
}

func (p BundleProfile) validateAgents(bundleName string) error {
	seenAgents := make(map[string]struct{}, len(p.Agents))
	for idx, agent := range p.Agents {
		name := strings.TrimSpace(agent.Agent.Name)
		if name == "" {
			return fmt.Errorf(
				"%w: bundle %q profile %q agents[%d].AGENT.md name is required",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				idx,
			)
		}
		key := bundleLookupKey(name)
		if _, exists := seenAgents[key]; exists {
			return fmt.Errorf(
				"%w: bundle %q profile %q agent %q is duplicated",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				name,
			)
		}
		seenAgents[key] = struct{}{}
		if strings.TrimSpace(agent.Path) == "" {
			return fmt.Errorf(
				"%w: bundle %q profile %q agent %q path is required",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				name,
			)
		}
		if err := agent.Agent.Validate(); err != nil {
			return fmt.Errorf(
				"%w: bundle %q profile %q agent %q: %w",
				ErrBundleInvalid,
				bundleName,
				p.Name,
				name,
				err,
			)
		}
	}
	return nil
}
