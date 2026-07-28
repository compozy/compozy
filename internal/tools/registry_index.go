package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type registryIndex struct {
	entries []*registryEntry
	byID    map[ToolID]*registryEntry
}

type registryEntry struct {
	descriptor Descriptor
	provider   Provider
	conflicts  []ReasonCode
}

func (r *RuntimeRegistry) buildIndex(ctx context.Context, scope Scope) (*registryIndex, error) {
	index := &registryIndex{
		entries: make([]*registryEntry, 0),
		byID:    make(map[ToolID]*registryEntry),
	}
	for _, provider := range r.providers {
		descriptors, err := provider.List(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("tools: list provider %s: %w", sourceKey(provider.ID()), err)
		}
		slices.SortFunc(descriptors, func(a Descriptor, b Descriptor) int {
			return strings.Compare(a.ID.String(), b.ID.String())
		})
		for descriptorIndex := range descriptors {
			descriptor := &descriptors[descriptorIndex]
			if err := descriptor.Validate(); err != nil {
				return nil, indexValidationError(descriptor.ID, err)
			}
			if existing, ok := index.byID[descriptor.ID]; ok {
				reason := conflictReason(&existing.descriptor, descriptor)
				existing.conflicts = appendReason(existing.conflicts, reason)
				continue
			}
			entry := &registryEntry{descriptor: cloneDescriptor(*descriptor), provider: provider}
			index.byID[descriptor.ID] = entry
			index.entries = append(index.entries, entry)
		}
	}
	slices.SortFunc(index.entries, func(a *registryEntry, b *registryEntry) int {
		return strings.Compare(a.descriptor.ID.String(), b.descriptor.ID.String())
	})
	r.descriptorMetadata.remember(scope.WorkspaceID, index.entries)
	return index, nil
}

func (r *RuntimeRegistry) evaluatorFor(ctx context.Context, scope Scope, ids []ToolID) (PolicyEvaluator, error) {
	if r.usePolicyInput {
		inputs, err := r.policyInputsFor(ctx, scope)
		if err != nil {
			return nil, err
		}
		return NewEffectivePolicyEvaluator(inputs, r.toolsets, ids)
	}
	if r.evaluator != nil {
		return r.evaluator, nil
	}
	return NewEffectivePolicyEvaluator(DefaultPolicyInputs(), ToolsetCatalog{}, ids)
}

func (r *RuntimeRegistry) policyInputsFor(ctx context.Context, scope Scope) (PolicyInputs, error) {
	resolver := r.policyResolver
	if resolver == nil {
		resolver = NewStaticPolicyInputResolver(r.policyInputs)
	}
	inputs, err := resolver.Resolve(ctx, scope)
	if err != nil {
		return PolicyInputs{}, err
	}
	return clonePolicyInputs(inputs), nil
}

func (r *RuntimeRegistry) toolsetView(ctx context.Context, scope Scope, toolset Toolset) (ToolsetView, error) {
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return ToolsetView{}, err
	}
	expanded, err := r.toolsets.Expand(toolset.ID, index.ids())
	if err != nil {
		reason, ok := ReasonOf(err)
		if !ok {
			reason = ReasonToolUnknown
		}
		return ToolsetView{
			Toolset:     cloneToolset(toolset),
			ReasonCodes: []ReasonCode{reason},
		}, nil
	}
	return ToolsetView{
		Toolset:       cloneToolset(toolset),
		ExpandedTools: expanded,
	}, nil
}

func (i *registryIndex) ids() []ToolID {
	ids := make([]ToolID, 0, len(i.entries))
	for _, entry := range i.entries {
		ids = append(ids, entry.descriptor.ID)
	}
	return ids
}

func indexValidationError(id ToolID, err error) error {
	reason, ok := ReasonOf(err)
	if !ok {
		reason = ReasonPolicyDenied
	}
	code := ErrorCodeDenied
	if reason == ReasonIDTooLong || reason == ReasonConflictedID || reason == ReasonConflictedSanitizedName {
		code = ErrorCodeConflict
	}
	return NewToolError(code, id, "tool descriptor rejected during indexing", err, reason)
}

func conflictReason(existing *Descriptor, candidate *Descriptor) ReasonCode {
	if isExternalDescriptor(existing) && isExternalDescriptor(candidate) &&
		externalRawKey(existing) != externalRawKey(candidate) {
		return ReasonConflictedSanitizedName
	}
	return ReasonConflictedID
}

func isExternalDescriptor(d *Descriptor) bool {
	return d != nil && (d.Source.Kind == SourceMCP || d.Source.Kind == SourceExtension)
}

func externalRawKey(d *Descriptor) string {
	if d == nil {
		return ""
	}
	return string(d.Source.Kind) + "\x00" + d.Source.Owner + "\x00" +
		d.Source.RawServerName + "\x00" + d.Source.RawToolName
}

func sourceKey(source SourceRef) string {
	return string(source.Kind) + "\x00" + source.Owner + "\x00" +
		source.RawServerName + "\x00" + source.RawToolName + "\x00" + source.Scope
}
