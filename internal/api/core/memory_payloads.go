package core

import (
	"errors"
	"fmt"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/frontmatter"
	"github.com/compozy/agh/internal/memory"
)

func validateMemoryCreateRequest(req contract.MemoryCreateRequest) error {
	if err := req.Type.Validate(); err != nil {
		return NewMemoryValidationError(err)
	}
	if strings.TrimSpace(req.Name) == "" {
		return NewMemoryValidationError(errors.New("name is required"))
	}
	if strings.TrimSpace(req.Content) == "" {
		return NewMemoryValidationError(errors.New("content is required"))
	}
	return nil
}

func (h *BaseHandlers) memoryCandidateFromCreate(
	selector memorySelector,
	req contract.MemoryCreateRequest,
) memcontract.Candidate {
	metadata := cloneMemoryStringMap(req.Metadata)
	metadata[memoryMetadataIDKey] = strings.TrimSpace(req.IdempotencyKey)
	metadata[memoryMetadataTargetEntityKey] = strings.TrimSpace(req.Entity)
	metadata[memoryMetadataTargetAttributeKey] = strings.TrimSpace(req.Attribute)
	return memcontract.Candidate{
		WorkspaceID: selector.WorkspaceID,
		Scope:       selector.Scope,
		AgentName:   selector.AgentName,
		AgentTier:   selector.AgentTier,
		Origin:      firstNonEmptyOrigin(req.Origin, h.memoryOrigin()),
		Content:     strings.TrimSpace(req.Content),
		Frontmatter: memcontract.Header{
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
			Type:        req.Type.Normalize(),
			Scope:       selector.Scope,
			AgentName:   selector.AgentName,
			AgentTier:   selector.AgentTier,
		},
		Entity:      strings.TrimSpace(req.Entity),
		Attribute:   strings.TrimSpace(req.Attribute),
		Metadata:    metadata,
		SubmittedAt: h.nowUTC(),
	}
}

func (h *BaseHandlers) memoryCandidateFromEdit(
	location MemoryLocation,
	req contract.MemoryEditRequest,
	rawContent []byte,
) (memcontract.Candidate, error) {
	header, body, err := memoryHeaderAndBody(rawContent)
	if err != nil {
		return memcontract.Candidate{}, err
	}
	header.Name = firstNonEmptyString(req.Name, header.Name)
	header.Description = firstNonEmptyString(req.Description, header.Description)
	if req.Type.Normalize() != "" {
		header.Type = req.Type.Normalize()
	}
	header.Scope = location.Scope
	header.AgentName = firstNonEmptyString(req.AgentName, header.AgentName, location.AgentName)
	header.AgentTier = firstNonEmptyAgentTier(req.AgentTier, header.AgentTier, location.AgentTier)
	content := strings.TrimSpace(req.Content)
	if content == "" {
		content = strings.TrimSpace(body)
	}
	metadata := cloneMemoryStringMap(req.Metadata)
	metadata[memoryMetadataIDKey] = strings.TrimSpace(req.IdempotencyKey)
	metadata[memoryMetadataTargetFilenameKey] = strings.TrimSpace(locationFilename(location, header))
	return memcontract.Candidate{
		WorkspaceID: location.WorkspaceID,
		Scope:       location.Scope,
		AgentName:   header.AgentName,
		AgentTier:   header.AgentTier,
		Origin:      h.memoryOrigin(),
		Content:     content,
		Frontmatter: header,
		Metadata:    metadata,
		SubmittedAt: h.nowUTC(),
	}, nil
}

func (h *BaseHandlers) memoryEntryPayload(
	store *memory.Store,
	location MemoryLocation,
	content []byte,
) (contract.MemoryEntryPayload, error) {
	header, body, err := memoryHeaderAndBody(content)
	if err != nil {
		return contract.MemoryEntryPayload{}, err
	}
	header.Scope = location.Scope
	header.AgentName = firstNonEmptyString(header.AgentName, location.AgentName)
	header.AgentTier = firstNonEmptyAgentTier(header.AgentTier, location.AgentTier)
	header.Filename = locationFilename(location, header)
	if header.ModTime.IsZero() {
		if found := memoryHeaderByFilename(store, location.Scope, header.Filename); found.Filename != "" {
			header = found
		}
	}
	return contract.MemoryEntryPayload{
		Summary: memorySummaryPayload(header),
		Content: strings.TrimSpace(body),
	}, nil
}

func memoryHeaderByFilename(store *memory.Store, scope memcontract.Scope, filename string) memcontract.Header {
	if store == nil {
		return memcontract.Header{}
	}
	headers, err := store.Scan(scope)
	if err != nil {
		return memcontract.Header{}
	}
	for _, header := range headers {
		if header.Filename == filename {
			return header
		}
	}
	return memcontract.Header{}
}

func memoryHeaderAndBody(content []byte) (memcontract.Header, string, error) {
	header, err := memory.ParseHeader(content)
	if err != nil {
		return memcontract.Header{}, "", err
	}
	parts, err := frontmatter.Split(content)
	if err != nil {
		return memcontract.Header{}, "", fmt.Errorf("memory: split frontmatter: %w", err)
	}
	return header, parts.Body, nil
}

func memorySummaryPayloads(headers []memcontract.Header) []contract.MemoryEntrySummaryPayload {
	payloads := make([]contract.MemoryEntrySummaryPayload, 0, len(headers))
	for _, header := range headers {
		payloads = append(payloads, MemorySummaryPayloadFromHeader(header))
	}
	return payloads
}

func memorySummaryPayload(header memcontract.Header) contract.MemoryEntrySummaryPayload {
	return MemorySummaryPayloadFromHeader(header)
}

// MemorySummaryPayloadFromHeader converts one resolved memory header for public transports.
func MemorySummaryPayloadFromHeader(header memcontract.Header) contract.MemoryEntrySummaryPayload {
	payload := contract.MemoryEntrySummaryPayload{
		Filename:      strings.TrimSpace(header.Filename),
		Name:          strings.TrimSpace(header.Name),
		Description:   strings.TrimSpace(header.Description),
		Type:          header.Type.Normalize(),
		Scope:         header.Scope.Normalize(),
		WorkspaceID:   strings.TrimSpace(header.WorkspaceID),
		AgentName:     strings.TrimSpace(header.AgentName),
		AgentTier:     header.AgentTier.Normalize(),
		ModTime:       header.ModTime.UTC(),
		Injection:     !memory.IsSystemManagedFilename(header.Filename),
		SystemManaged: memory.IsSystemManagedFilename(header.Filename),
	}
	if header.Provenance != nil {
		created := header.Provenance.CreatedAt.UTC()
		updated := header.Provenance.UpdatedAt.UTC()
		if !created.IsZero() {
			payload.CreatedAt = &created
		}
		if !updated.IsZero() {
			payload.UpdatedAt = &updated
		}
		payload.SupersededBy = strings.TrimSpace(header.Provenance.SupersededBy)
	}
	return payload
}

func memorySelectorPayload(selector memorySelector) contract.MemoryScopeSelectorPayload {
	return contract.MemoryScopeSelectorPayload{
		Scope:       selector.Scope.Normalize(),
		WorkspaceID: strings.TrimSpace(selector.WorkspaceID),
		AgentName:   strings.TrimSpace(selector.AgentName),
		AgentTier:   selector.AgentTier.Normalize(),
	}
}

func memoryPrecedencePayloads(selector memorySelector) []contract.MemoryScopeSelectorPayload {
	preference := make([]contract.MemoryScopeSelectorPayload, 0, 3)
	if selector.Scope == memcontract.ScopeAgent {
		preference = append(preference, memorySelectorPayload(selector))
	}
	if selector.Scope == memcontract.ScopeWorkspace || selector.WorkspaceID != "" {
		preference = append(preference, contract.MemoryScopeSelectorPayload{
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: strings.TrimSpace(selector.WorkspaceID),
		})
	}
	preference = append(preference, contract.MemoryScopeSelectorPayload{Scope: memcontract.ScopeGlobal})
	return preference
}

func (h *BaseHandlers) memorySelectorRoots(selector memorySelector) map[string]string {
	roots := map[string]string{
		string(memcontract.ScopeGlobal): strings.TrimSpace(h.Config.Memory.GlobalDir),
	}
	if selector.Workspace != "" {
		roots[string(memcontract.ScopeWorkspace)] = selector.Workspace
	}
	return roots
}

func (h *BaseHandlers) memoryMutableConfigPaths() []string {
	return []string{
		"memory.enabled",
		"memory.controller.mode",
		"memory.recall.top_k",
		"memory.provider.name",
	}
}

func (h *BaseHandlers) memoryLockedConfigPaths() []string {
	return []string{
		"memory.global_dir",
		"memory.workspace.toml_path",
		"memory.session.ledger_root",
	}
}

func (h *BaseHandlers) memoryProviderPayloads() []contract.MemoryProviderPayload {
	name := strings.TrimSpace(h.Config.Memory.Provider.Name)
	if name == "" {
		name = memoryLocalProviderName
	}
	return []contract.MemoryProviderPayload{{
		Name:    name,
		Status:  contract.MemoryProviderStateActive,
		Active:  true,
		Builtin: name == memoryLocalProviderName,
		Tools:   []string{},
	}}
}

func memorySearchResultPayloads(recall memcontract.Packaged) []contract.MemorySearchResultPayload {
	results := make([]contract.MemorySearchResultPayload, 0)
	for _, block := range recall.Blocks {
		for idx, entry := range block.Entries {
			memoryType := entry.Type.Normalize()
			if memoryType == "" {
				memoryType = memcontract.TypeReference
			}
			results = append(results, contract.MemorySearchResultPayload{
				Memory: contract.MemoryEntrySummaryPayload{
					Filename:        firstNonEmptyString(entry.Filename, entry.ID),
					Name:            strings.TrimSpace(entry.Title),
					Type:            memoryType,
					Scope:           block.Scope.Normalize(),
					WorkspaceID:     strings.TrimSpace(entry.WorkspaceID),
					AgentTier:       block.AgentTier.Normalize(),
					ModTime:         entry.ModTime.UTC(),
					StalenessBanner: strings.TrimSpace(entry.StalenessBanner),
					Injection:       true,
				},
				Score:       1 / float64(idx+1),
				Snippet:     strings.TrimSpace(entry.Body),
				WhyRecalled: cloneStrings(entry.WhyRecalled),
			})
		}
	}
	return results
}

func locationFilename(location MemoryLocation, header memcontract.Header) string {
	if header.Filename != "" {
		return strings.TrimSpace(header.Filename)
	}
	return strings.TrimSpace(location.Filename)
}

func (h *BaseHandlers) memoryOrigin() memcontract.Origin {
	switch h.transportName() {
	case transportNameUDSAPI:
		return memcontract.OriginUDS
	default:
		return memcontract.OriginHTTP
	}
}

func firstNonEmptyOrigin(values ...memcontract.Origin) memcontract.Origin {
	for _, value := range values {
		if normalized := value.Normalize(); normalized != "" {
			return normalized
		}
	}
	return ""
}

func firstNonEmptyAgentTier(values ...memcontract.AgentTier) memcontract.AgentTier {
	for _, value := range values {
		if normalized := value.Normalize(); normalized != "" {
			return normalized
		}
	}
	return ""
}
