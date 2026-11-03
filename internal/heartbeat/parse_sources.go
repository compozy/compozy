package heartbeat

import (
	"encoding/json/v2"
	"errors"

	"github.com/compozy/compozy/internal/heartbeat/source"
)

func sourceDiagnosticCode(err error) string {
	if e, ok := errors.AsType[*source.Error](err); ok {
		return string(e.Code)
	}
	return heartbeatHeartbeatInvalidFieldTypeKey
}
func parseSources(value any) ([]SourceDecl, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, source.Failure("heartbeat_source_ref_invalid", "context.sources must be a list")
	}
	refs := make([]SourceDecl, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		data, err := json.Marshal(item, json.Deterministic(true))
		if err != nil {
			return nil, err
		}
		var ref SourceDecl
		if err := json.Unmarshal(data, &ref, json.RejectUnknownMembers(true)); err != nil {
			return nil, source.Failure("heartbeat_source_ref_invalid", "invalid source declaration")
		}
		if err := ref.Validate(); err != nil {
			return nil, err
		}
		if seen[ref.ID] {
			return nil, source.Failure("heartbeat_source_duplicate_id", "source id is declared twice")
		}
		seen[ref.ID] = true
		refs = append(refs, ref)
	}
	return refs, nil
}
