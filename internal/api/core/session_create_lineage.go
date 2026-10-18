package core

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
)

// validateCreateSessionLineageKind accepts only the kinds a plain create may record;
// continue and fork are written by the derive path alone.
func validateCreateSessionLineageKind(transport string, req contract.CreateSessionRequest) error {
	kind := store.LineageKind(strings.TrimSpace(req.LineageKind))
	if kind == store.LineageKindRoot {
		return nil
	}
	if kind != store.LineageKindProvenance && kind != store.LineageKindRecovery {
		return prefixedError(transport, fmt.Sprintf(
			"lineage_kind must be %q or %q, got %q",
			store.LineageKindProvenance,
			store.LineageKindRecovery,
			kind,
		))
	}
	if strings.TrimSpace(req.ParentSessionID) == "" {
		return prefixedError(transport, "lineage_kind requires parent_session_id")
	}
	return nil
}

// createSessionLineageKind maps an accepted create request kind to the store kind;
// provenance is the default relation for a parented create.
func createSessionLineageKind(value string) store.LineageKind {
	if kind := store.LineageKind(strings.TrimSpace(value)); kind != store.LineageKindRoot {
		return kind
	}
	return store.LineageKindProvenance
}
