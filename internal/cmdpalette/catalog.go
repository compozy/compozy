package cmdpalette

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type structuralCatalog struct {
	Commands []structuralCommand `json:"commands"`
	Sources  []SourceStatus      `json:"sources"`
}

type structuralCommand struct {
	Descriptor Descriptor `json:"descriptor"`
	Bindings   []string   `json:"bindings"`
	Alias      *string    `json:"alias"`
}

func (s *Service) Catalog(
	ctx context.Context,
	workspaceID WorkspaceID,
	clientID ClientID,
) (Catalog, error) {
	if ctx == nil {
		return Catalog{}, fmt.Errorf("cmd palette: catalog context is required")
	}
	if workspaceID == "" {
		return Catalog{}, fmt.Errorf("cmd palette: workspace ID is required")
	}
	descriptors, sources, err := s.collectDescriptors(ctx, workspaceID)
	if err != nil {
		return Catalog{}, err
	}
	bindings, aliases, err := s.resolveBindings(ctx, workspaceID)
	if err != nil {
		return Catalog{}, err
	}
	var snapshot *ContextSnapshot
	if clientID != "" {
		if s.clients == nil {
			return Catalog{}, ErrNoAttachedShell
		}
		resolved, contextErr := s.clients.Context(ctx, workspaceID, clientID)
		if contextErr != nil {
			return Catalog{}, contextErr
		}
		snapshot = &resolved
	}
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
		commands = append(commands, ResolvedCommand{
			Descriptor:        cloneDescriptor(descriptor),
			Available:         available,
			UnavailableReason: reason,
			Bindings:          resolvedBindings,
			Alias:             alias,
		})
	}
	sort.Slice(commands, func(left, right int) bool { return commands[left].ID < commands[right].ID })
	revision, err := structuralRevision(commands, sources)
	if err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{Commands: commands, Sources: sources, Revision: revision}
	if snapshot != nil {
		catalog.ContextRevision = snapshot.Revision
	}
	return catalog, nil
}

func (s *Service) collectDescriptors(
	ctx context.Context,
	workspaceID WorkspaceID,
) ([]Descriptor, []SourceStatus, error) {
	commands := make([]Descriptor, 0)
	sources := make([]SourceStatus, 0, len(s.providers))
	seen := make(map[CommandID]string)
	for _, registration := range s.providers {
		sourceID := registration.Source.ID()
		provided, err := registration.Provider.ProvideCommands(ctx, workspaceID)
		if err != nil {
			sources = append(sources, SourceStatus{Source: sourceID, Status: SourceDegraded, Reason: err.Error()})
			continue
		}
		sources = append(sources, SourceStatus{Source: sourceID, Status: SourceHealthy})
		for _, descriptor := range provided {
			descriptor = normalizeDescriptor(descriptor)
			if err := validateProviderDescriptor(registration.Source, descriptor); err != nil {
				return nil, nil, err
			}
			if first, exists := seen[descriptor.ID]; exists {
				return nil, nil, &DuplicateCommandIDError{ID: descriptor.ID, First: first, Second: sourceID}
			}
			seen[descriptor.ID] = sourceID
			commands = append(commands, cloneDescriptor(descriptor))
		}
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].Source < sources[right].Source })
	return commands, sources, nil
}

func (s *Service) resolveBindings(
	ctx context.Context,
	workspaceID WorkspaceID,
) (map[CommandID][]string, map[CommandID]string, error) {
	if s.bindings == nil {
		return map[CommandID][]string{}, map[CommandID]string{}, nil
	}
	bindings, aliases, err := s.bindings.Bindings(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("cmd palette: resolve bindings: %w", err)
	}
	return bindings, aliases, nil
}

func structuralRevision(commands []ResolvedCommand, sources []SourceStatus) (string, error) {
	structural := structuralCatalog{Sources: append([]SourceStatus(nil), sources...)}
	structural.Commands = make([]structuralCommand, 0, len(commands))
	for _, command := range commands {
		structural.Commands = append(structural.Commands, structuralCommand{
			Descriptor: cloneDescriptor(command.Descriptor),
			Bindings:   append([]string(nil), command.Bindings...),
			Alias:      cloneString(command.Alias),
		})
	}
	payload, err := json.Marshal(structural)
	if err != nil {
		return "", fmt.Errorf("cmd palette: encode structural catalog: %w", err)
	}
	digest := sha256.Sum256(payload)
	return "cr_" + hex.EncodeToString(digest[:]), nil
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
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
