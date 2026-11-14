package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

// AdvertisedCatalog records entitlement independently of transient backend health.
type AdvertisedCatalog struct {
	Digest        string            `json:"digest"`
	Servers       map[string]int    `json:"servers"`
	ServerDigests map[string]string `json:"-"`
}

// AdvertisedCatalogDigest hashes external source identity, tool names and input schemas only.
func AdvertisedCatalogDigest(views []ToolView) string {
	atoms := make([]string, 0, len(views))
	for _, view := range views {
		d := view.Descriptor
		if d.Source.Kind != SourceMCP && d.Source.Kind != SourceExtension {
			continue
		}
		atoms = append(atoms, strings.Join([]string{
			string(d.Source.Kind), d.Source.Owner, d.Source.RawServerName, d.Source.Scope,
			d.Source.ProfileID, d.Source.WorkspaceID, d.Source.ResourceID,
			d.ID.String(), d.InputSchemaDigest,
		}, "\x00"))
	}
	slices.Sort(atoms)
	sum := sha256.Sum256([]byte(strings.Join(atoms, "\n")))
	return hex.EncodeToString(sum[:])
}

// NewAdvertisedCatalog captures the digest and per-source tool counts of one advertised revision.
func NewAdvertisedCatalog(views []ToolView) AdvertisedCatalog {
	catalog := AdvertisedCatalog{
		Digest:        AdvertisedCatalogDigest(views),
		Servers:       make(map[string]int),
		ServerDigests: make(map[string]string),
	}
	servers := make(map[string][]ToolView)
	for _, view := range views {
		source := view.Descriptor.Source
		if source.Kind != SourceMCP && source.Kind != SourceExtension {
			continue
		}
		name := source.RawServerName
		if name == "" {
			name = source.Owner
		}
		catalog.Servers[name]++
		servers[name] = append(servers[name], view)
	}
	for name, entries := range servers {
		catalog.ServerDigests[name] = AdvertisedCatalogDigest(entries)
	}
	return catalog
}

// AdvertisedSessionProjection keeps policy and auth withdrawals, while ignoring health-only failures.
func (r *RuntimeRegistry) AdvertisedSessionProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	ctx = withPolicyResolutionCache(ctx)
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return nil, err
	}
	evaluator, err := r.evaluatorFor(ctx, scope, index.ids())
	if err != nil {
		return nil, err
	}
	candidates := make([]sessionProjectionEntry, 0, len(index.entries))
	for _, entry := range index.entries {
		if !isExternalDescriptor(&entry.descriptor) || len(entry.conflicts) > 0 {
			continue
		}
		decision, err := evaluateIndexedDescriptor(ctx, scope, evaluator, entry.descriptor)
		if err != nil {
			return nil, err
		}
		if decision.VisibleToSession && decision.Callable {
			candidates = append(candidates, sessionProjectionEntry{entry: entry, decision: decision})
		}
	}
	batched := projectionAvailabilities(ctx, scope, candidates)
	views := make([]ToolView, 0, len(candidates))
	for _, candidate := range candidates {
		entry := candidate.entry
		availability, found := batched[entry.descriptor.ID]
		if !found {
			availability = r.availabilityFor(ctx, scope, entry)
		}
		if !advertisedEntitlement(availability) {
			continue
		}
		views = append(views, ToolView{
			Descriptor: cloneDescriptor(entry.descriptor), Availability: availability, Decision: candidate.decision,
		})
	}
	return views, nil
}

func advertisedEntitlement(a Availability) bool {
	if a.Conflicted || !a.Enabled {
		return false
	}
	if a.Executable {
		return true
	}
	if len(a.ReasonCodes) == 0 {
		return false
	}
	for _, reason := range a.ReasonCodes {
		switch reason {
		case ReasonBackendUnhealthy, ReasonBackendDead, ReasonBackendNotExecutable,
			ReasonExtensionInactive, ReasonRuntimeDescriptorMissing:
		default:
			return false
		}
	}
	return true
}
