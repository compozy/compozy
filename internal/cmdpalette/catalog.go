package cmdpalette

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
)

type structuralCatalog struct {
	ProfileLens ProfileLens         `json:"profile_lens"`
	Commands    []structuralCommand `json:"commands"`
	Sources     []SourceStatus      `json:"sources"`
}

type structuralCommand struct {
	Descriptor  Descriptor `json:"descriptor"`
	Bindings    []string   `json:"bindings"`
	Alias       *string    `json:"alias"`
	GlobalChord string     `json:"global_chord,omitempty"`
}

// BindableIDs returns the current workspace catalog ids in deterministic order.
func (s *Service) BindableIDs(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
) ([]CommandID, error) {
	if ctx == nil {
		return nil, fmt.Errorf("cmd palette: bindable ids context is required")
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("cmd palette: workspace ID is required")
	}
	if err := profileLens.Validate(); err != nil {
		return nil, err
	}
	descriptors, _, _, err := s.collectDescriptors(ctx, CatalogRequest{
		ProfileLens: profileLens,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]CommandID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	slices.Sort(ids)
	return ids, nil
}

// Catalog returns the workspace command projection, optionally scoped to one
// attached client's context, with commands sorted by id.
func (s *Service) Catalog(
	ctx context.Context,
	request CatalogRequest,
) (Catalog, error) {
	if ctx == nil {
		return Catalog{}, fmt.Errorf("cmd palette: catalog context is required")
	}
	if request.WorkspaceID == "" {
		return Catalog{}, fmt.Errorf("cmd palette: workspace ID is required")
	}
	if err := request.ProfileLens.Validate(); err != nil {
		return Catalog{}, err
	}
	descriptors, sources, defaults, err := s.collectDescriptors(ctx, request)
	if err != nil {
		return Catalog{}, err
	}
	bindings, aliases, err := s.resolveSnapshotBindings(
		ctx, request.ProfileLens, request.WorkspaceID, descriptors, defaults,
	)
	if err != nil {
		return Catalog{}, err
	}
	globalBindings, err := s.resolveSnapshotGlobalBindings(
		ctx, request.ProfileLens, request.WorkspaceID, descriptors,
	)
	if err != nil {
		return Catalog{}, err
	}
	globalStatuses := map[CommandID]GlobalShortcut{}
	var snapshot *ContextSnapshot
	if request.ClientID != "" {
		if s.clients == nil {
			return Catalog{}, ErrNoAttachedShell
		}
		resolved, contextErr := s.clients.Context(ctx, request.WorkspaceID, request.ClientID)
		if contextErr != nil {
			return Catalog{}, contextErr
		}
		snapshot = &resolved
		if directory, ok := s.clients.(GlobalShortcutStatusDirectory); ok {
			globalStatuses, err = directory.GlobalShortcutStatuses(
				ctx, request.ProfileLens, request.WorkspaceID, request.ClientID,
			)
			if err != nil {
				return Catalog{}, err
			}
		}
	}
	commands := resolveCatalogCommands(descriptors, bindings, aliases, globalBindings, globalStatuses, snapshot)
	sort.Slice(commands, func(left, right int) bool { return commands[left].ID < commands[right].ID })
	revision, err := structuralRevision(request.ProfileLens, commands, sources)
	if err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{
		Commands: commands, Sources: sources, Revision: revision,
		ProfileLens: request.ProfileLens,
	}
	if snapshot != nil {
		catalog.ContextRevision = snapshot.Revision
	}
	return catalog, nil
}

func resolveCatalogCommands(
	descriptors []Descriptor,
	bindings map[CommandID][]string,
	aliases map[CommandID]string,
	globalBindings map[CommandID]string,
	globalStatuses map[CommandID]GlobalShortcut,
	snapshot *ContextSnapshot,
) []ResolvedCommand {
	commands := make([]ResolvedCommand, 0, len(descriptors))
	for _, descriptor := range descriptors {
		available, reason := resolveAvailability(descriptor, snapshot)
		resolvedBindings := append([]string(nil), bindings[descriptor.ID]...)
		sort.Strings(resolvedBindings)
		var alias *string
		if value, exists := aliases[descriptor.ID]; exists {
			cloned := value
			alias = &cloned
		}
		var globalShortcut *GlobalShortcut
		if chord, exists := globalBindings[descriptor.ID]; exists {
			resolved := GlobalShortcut{IntendedChord: chord}
			if status, reported := globalStatuses[descriptor.ID]; reported && status.IntendedChord == chord {
				resolved.ActiveChord = status.ActiveChord
				resolved.Status = status.Status
				resolved.Reason = status.Reason
				resolved.SettingsURL = status.SettingsURL
			}
			globalShortcut = &resolved
		}
		commands = append(commands, ResolvedCommand{
			Descriptor:        cloneDescriptor(descriptor),
			Available:         available,
			UnavailableReason: reason,
			Bindings:          resolvedBindings,
			Alias:             alias,
			GlobalShortcut:    globalShortcut,
		})
	}
	return commands
}

func (s *Service) resolveSnapshotGlobalBindings(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	descriptors []Descriptor,
) (map[CommandID]string, error) {
	resolver, ok := s.bindings.(SnapshotGlobalBindingsResolver)
	if !ok {
		return map[CommandID]string{}, nil
	}
	ids := make([]CommandID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	bindings, err := resolver.GlobalBindingsForCatalogSnapshot(ctx, profileLens, workspaceID, ids)
	if err != nil {
		return nil, fmt.Errorf("cmd palette: resolve global bindings: %w", err)
	}
	return bindings, nil
}

func (s *Service) collectDescriptors(
	ctx context.Context,
	request CatalogRequest,
) ([]Descriptor, []SourceStatus, []ExtensionDefaultShortcut, error) {
	if err := request.ProfileLens.Validate(); err != nil {
		return nil, nil, nil, err
	}
	commands := make([]Descriptor, 0)
	sources := make([]SourceStatus, 0, len(s.providers))
	defaults := make([]ExtensionDefaultShortcut, 0)
	seen := make(map[CommandID]string)
	for _, registration := range s.providers {
		sourceID := registration.Source.ID()
		if provider, ok := registration.Provider.(ContributionProvider); ok {
			contribution, err := provider.ProvideContribution(ctx, request)
			if err != nil {
				sources = append(sources, SourceStatus{
					Source: sourceID, Status: SourceDegraded, Reason: err.Error(),
				})
				continue
			}
			sources = append(sources, contribution.Sources...)
			for _, descriptor := range contribution.Commands {
				descriptor = normalizeDescriptor(descriptor)
				if descriptor.Source.Kind != SourceKindExtension {
					return nil, nil, nil, invalidDescriptor(
						"%s: contribution source must be extension", descriptor.ID,
					)
				}
				if err := ValidateDescriptor(descriptor); err != nil {
					return nil, nil, nil, err
				}
				descriptorSource := descriptor.Source.ID()
				if first, exists := seen[descriptor.ID]; exists {
					return nil, nil, nil, &DuplicateCommandIDError{
						ID: descriptor.ID, First: first, Second: descriptorSource,
					}
				}
				seen[descriptor.ID] = descriptorSource
				commands = append(commands, cloneDescriptor(descriptor))
			}
			defaults = append(defaults, contribution.Defaults...)
			continue
		}
		provided, err := registration.Provider.ProvideCommands(ctx, request)
		if err != nil {
			sources = append(sources, SourceStatus{Source: sourceID, Status: SourceDegraded, Reason: err.Error()})
			continue
		}
		sources = append(sources, SourceStatus{Source: sourceID, Status: SourceHealthy})
		for _, descriptor := range provided {
			descriptor = normalizeDescriptor(descriptor)
			if err := validateProviderDescriptor(registration.Source, descriptor); err != nil {
				return nil, nil, nil, err
			}
			if first, exists := seen[descriptor.ID]; exists {
				return nil, nil, nil, &DuplicateCommandIDError{
					ID: descriptor.ID, First: first, Second: sourceID,
				}
			}
			seen[descriptor.ID] = sourceID
			commands = append(commands, cloneDescriptor(descriptor))
		}
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].Source < sources[right].Source })
	return commands, sources, defaults, nil
}

// ExtensionDefaults returns the active and dormant extension shortcut claims in enable order.
func (s *Service) ExtensionDefaults(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
) ([]ExtensionDefaultShortcut, error) {
	if ctx == nil {
		return nil, fmt.Errorf("cmd palette: extension defaults context is required")
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("cmd palette: workspace ID is required")
	}
	if err := profileLens.Validate(); err != nil {
		return nil, err
	}
	result := make([]ExtensionDefaultShortcut, 0)
	for _, registration := range s.providers {
		provider, ok := registration.Provider.(ContributionProvider)
		if !ok {
			continue
		}
		contribution, err := provider.ProvideContribution(ctx, CatalogRequest{
			ProfileLens: profileLens,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, contribution.Defaults...)
	}
	return result, nil
}

func (s *Service) resolveBindings(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
) (map[CommandID][]string, map[CommandID]string, error) {
	if s.bindings == nil {
		return map[CommandID][]string{}, map[CommandID]string{}, nil
	}
	bindings, aliases, err := s.bindings.Bindings(ctx, profileLens, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("cmd palette: resolve bindings: %w", err)
	}
	return bindings, aliases, nil
}

func (s *Service) resolveSnapshotBindings(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	descriptors []Descriptor,
	defaults []ExtensionDefaultShortcut,
) (map[CommandID][]string, map[CommandID]string, error) {
	if resolver, ok := s.bindings.(SnapshotBindingsResolver); ok {
		ids := make([]CommandID, 0, len(descriptors))
		for _, descriptor := range descriptors {
			ids = append(ids, descriptor.ID)
		}
		bindings, aliases, err := resolver.BindingsForCatalogSnapshot(
			ctx, profileLens, workspaceID, ids, defaults,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("cmd palette: resolve snapshot bindings: %w", err)
		}
		return bindings, aliases, nil
	}
	return s.resolveBindings(ctx, profileLens, workspaceID)
}

func structuralRevision(
	profileLens ProfileLens,
	commands []ResolvedCommand,
	sources []SourceStatus,
) (string, error) {
	structural := structuralCatalog{
		ProfileLens: profileLens,
		Sources:     append([]SourceStatus(nil), sources...),
	}
	structural.Commands = make([]structuralCommand, 0, len(commands))
	for _, command := range commands {
		structural.Commands = append(structural.Commands, structuralCommand{
			Descriptor:  cloneDescriptor(command.Descriptor),
			Bindings:    append([]string(nil), command.Bindings...),
			Alias:       cloneString(command.Alias),
			GlobalChord: globalShortcutIntent(command.GlobalShortcut),
		})
	}
	payload, err := json.Marshal(structural)
	if err != nil {
		return "", fmt.Errorf("cmd palette: encode structural catalog: %w", err)
	}
	digest := sha256.Sum256(payload)
	return "cr_" + hex.EncodeToString(digest[:]), nil
}

func globalShortcutIntent(binding *GlobalShortcut) string {
	if binding == nil {
		return ""
	}
	return binding.IntendedChord
}

// CloneDescriptor returns an independent copy of one command descriptor.
func CloneDescriptor(descriptor Descriptor) Descriptor {
	return cloneDescriptor(descriptor)
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	cloned := descriptor
	cloned.Keywords = append([]string(nil), descriptor.Keywords...)
	cloned.Arguments = append([]Argument(nil), descriptor.Arguments...)
	for index := range cloned.Arguments {
		cloned.Arguments[index].Options = append([]string(nil), descriptor.Arguments[index].Options...)
	}
	cloned.When = append([]Predicate(nil), descriptor.When...)
	cloned.Action.Args = cloneAnyMap(descriptor.Action.Args)
	if descriptor.Confirmation != nil {
		confirmation := *descriptor.Confirmation
		cloned.Confirmation = &confirmation
	}
	return cloned
}

func cloneAnyMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	cloned := make(map[string]any, len(source))
	maps.Copy(cloned, source)
	return cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
