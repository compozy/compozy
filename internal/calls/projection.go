package calls

import (
	"context"
	"fmt"
	"strings"
)

// ProjectionContent contains the prompt and result bytes needed by public call pages.
type ProjectionContent struct {
	Prompt     []byte
	Result     []byte
	Superseded []byte
}

type projectionRef struct {
	index int
	kind  byte
	ref   string
}

// ProjectPayloads batches the durable blobs needed to render the supplied call records.
func (s *Service) ProjectPayloads(ctx context.Context, records []CallRecord) ([]ProjectionContent, error) {
	contents := make([]ProjectionContent, len(records))
	if len(records) == 0 {
		return contents, nil
	}
	reader, err := s.payloadStore()
	if err != nil {
		return nil, err
	}
	refsByWorkspace := make(map[string]map[string]struct{})
	assignments := make(map[string][]projectionRef)
	for index, record := range records {
		workspaceID := strings.TrimSpace(record.WorkspaceID)
		for kind, ref := range map[byte]string{
			'p': record.PromptRef, 'r': record.ResultRef, 's': record.SupersededRef,
		} {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			if refsByWorkspace[workspaceID] == nil {
				refsByWorkspace[workspaceID] = make(map[string]struct{})
			}
			refsByWorkspace[workspaceID][ref] = struct{}{}
			assignments[workspaceID] = append(assignments[workspaceID], projectionRef{
				index: index, kind: kind, ref: ref,
			})
		}
	}
	for workspaceID, refSet := range refsByWorkspace {
		refs := make([]string, 0, len(refSet))
		for ref := range refSet {
			refs = append(refs, ref)
		}
		payloads, batchErr := reader.GetCallPayloads(ctx, workspaceID, refs)
		if batchErr != nil {
			return nil, fmt.Errorf("calls: project payloads for workspace %q: %w", workspaceID, batchErr)
		}
		for _, assignment := range assignments[workspaceID] {
			payload, ok := payloads[assignment.ref]
			if !ok {
				return nil, fmt.Errorf("calls: projected payload %q is missing", assignment.ref)
			}
			payload = append([]byte(nil), payload...)
			switch assignment.kind {
			case 'p':
				contents[assignment.index].Prompt = payload
			case 'r':
				contents[assignment.index].Result = payload
			case 's':
				contents[assignment.index].Superseded = payload
			}
		}
	}
	return contents, nil
}
