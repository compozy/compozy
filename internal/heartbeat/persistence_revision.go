package heartbeat

import (
	"fmt"
	"strings"
)

// Normalize trims revision metadata fields.
func (r Revision) Normalize() Revision {
	r.ID = strings.TrimSpace(r.ID)
	r.WorkspaceID = strings.TrimSpace(r.WorkspaceID)
	r.AgentName = strings.TrimSpace(r.AgentName)
	r.SourcePath = strings.TrimSpace(r.SourcePath)
	r.Operation = RevisionOperation(strings.TrimSpace(string(r.Operation)))
	r.PreviousDigest = strings.TrimSpace(r.PreviousDigest)
	r.NewDigest = strings.TrimSpace(r.NewDigest)
	r.NewSnapshotID = strings.TrimSpace(r.NewSnapshotID)
	r.ActorKind = ActorKind(strings.TrimSpace(string(r.ActorKind)))
	r.ActorID = strings.TrimSpace(r.ActorID)
	return r
}

// Validate ensures the revision is append-only authoring history.
func (r Revision) Validate() error {
	normalized := r.Normalize()
	switch {
	case normalized.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidRevision)
	case normalized.WorkspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidRevision)
	case normalized.AgentName == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidRevision)
	case normalized.SourcePath == "":
		return fmt.Errorf("%w: source path is required", ErrInvalidRevision)
	case !ValidRevisionOperation(normalized.Operation):
		return fmt.Errorf("%w: invalid operation %q", ErrInvalidRevision, normalized.Operation)
	case normalized.Operation == RevisionOperationDelete && normalized.NewDigest != "":
		return fmt.Errorf("%w: delete revision must not set new digest", ErrInvalidRevision)
	case normalized.Operation == RevisionOperationWrite && normalized.NewDigest == "":
		return fmt.Errorf("%w: new digest is required", ErrInvalidRevision)
	case normalized.Operation == RevisionOperationRollback &&
		normalized.NewDigest == "" &&
		normalized.NewSnapshotID == "":
		return fmt.Errorf("%w: rollback revision requires new digest or snapshot", ErrInvalidRevision)
	case !ValidActorKind(normalized.ActorKind):
		return fmt.Errorf("%w: invalid actor kind %q", ErrInvalidRevision, normalized.ActorKind)
	case normalized.ActorID == "":
		return fmt.Errorf("%w: actor id is required", ErrInvalidRevision)
	case normalized.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidRevision)
	}
	return nil
}

// Validate ensures the revision query uses sane bounds and operation filters.
func (q RevisionListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("%w: invalid revision limit %d", ErrInvalidRevision, q.Limit)
	}
	if q.Operation != "" && !ValidRevisionOperation(q.Operation) {
		return fmt.Errorf("%w: invalid operation %q", ErrInvalidRevision, q.Operation)
	}
	return nil
}

// Validate ensures rollback lookup identifiers are complete.
func (q RollbackLookup) Validate() error {
	switch {
	case strings.TrimSpace(q.WorkspaceID) == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidRevision)
	case strings.TrimSpace(q.AgentName) == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidRevision)
	case strings.TrimSpace(q.RevisionID) == "":
		return fmt.Errorf("%w: revision id is required", ErrInvalidRevision)
	default:
		return nil
	}
}
