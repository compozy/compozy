package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrSessionReparentRefused is the sentinel every lineage-rule refusal wraps.
	ErrSessionReparentRefused = errors.New("session: reparent refused")
	// ErrSessionReparentUnavailable reports that lineage persistence is not configured.
	ErrSessionReparentUnavailable = errors.New("session: session lineage store is unavailable")
)

// ReparentRefusal names the lineage rule that rejected a reparent request.
type ReparentRefusal string

const (
	// ReparentRefusalCrossWorkspace rejects subtrees spanning workspace boundaries.
	ReparentRefusalCrossWorkspace ReparentRefusal = "cross_workspace_subtree"
	// ReparentRefusalSourceHasParent rejects adopting a session that already has a parent.
	ReparentRefusalSourceHasParent ReparentRefusal = "source_has_parent"
	// ReparentRefusalSourceNotUser rejects reparenting managed, spawned, or subagent sessions.
	ReparentRefusalSourceNotUser ReparentRefusal = "source_not_user"
	// ReparentRefusalParentIneligible rejects a parent that is not a user session.
	ReparentRefusalParentIneligible ReparentRefusal = "parent_ineligible"
	// ReparentRefusalCycle rejects a parent that is the session itself or one of its descendants.
	ReparentRefusalCycle ReparentRefusal = "cycle"
	// ReparentRefusalArchived rejects reparenting when either session is archived.
	ReparentRefusalArchived ReparentRefusal = "archived"
	// ReparentRefusalNotDetachable rejects detaching a session that was not adopted.
	ReparentRefusalNotDetachable ReparentRefusal = "not_detachable"
	// ReparentRefusalSourceRunning rejects taking over a spawned worker that is still running.
	ReparentRefusalSourceRunning ReparentRefusal = "source_running"
)

// ReparentError reports which lineage rule refused a reparent request.
type ReparentError struct {
	Reason    ReparentRefusal
	SessionID string
	Detail    string
}

func (e *ReparentError) Error() string {
	if e == nil {
		return ErrSessionReparentRefused.Error()
	}
	return fmt.Sprintf("%s: %s: %s", ErrSessionReparentRefused, e.SessionID, e.Detail)
}

// Unwrap exposes ErrSessionReparentRefused to errors.Is.
func (e *ReparentError) Unwrap() error {
	return ErrSessionReparentRefused
}

func reparentRefusal(reason ReparentRefusal, sessionID string, detail string) error {
	return &ReparentError{Reason: reason, SessionID: strings.TrimSpace(sessionID), Detail: detail}
}

// AdoptSession makes a parentless user session a provenance child of another
// user session in the same workspace and profile. The session's subtree is
// re-rooted under the new parent's root; governance fields are unchanged.
func (m *Manager) AdoptSession(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	parentSessionID string,
) (*Info, error) {
	if err := m.requireReparentInputs(ctx); err != nil {
		return nil, err
	}
	source, err := normalizeStoredSessionID(sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parentSessionID) == "" {
		return nil, fmt.Errorf("%w: parent session id is required", ErrValidation)
	}
	parent, err := normalizeStoredSessionID(parentSessionID)
	if err != nil {
		return nil, err
	}
	if parent == source {
		return nil, reparentRefusal(ReparentRefusalCycle, source, "a session cannot be its own parent")
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	workspaceID = strings.TrimSpace(workspaceID)
	sourceNode, err := m.loadReparentNode(ctx, source, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := validateAdoptSource(&sourceNode); err != nil {
		return nil, err
	}
	parentNode, err := m.loadReparentNode(ctx, parent, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("session: resolve parent session %q: %w", parent, err)
	}
	if err := validateAdoptParent(&sourceNode, &parentNode); err != nil {
		return nil, err
	}
	if err := m.requireReparentUnarchived(ctx, &sourceNode, &parentNode); err != nil {
		return nil, err
	}
	if err := m.requireAdoptParentNotInLater(ctx, &parentNode); err != nil {
		return nil, err
	}
	if err := m.requireNotAncestor(ctx, source, &parentNode); err != nil {
		return nil, err
	}

	rootID := parentNode.lineage.RootSessionID
	if rootID == "" {
		rootID = parentNode.id
	}
	adopted := sourceNode.lineage
	adopted.ParentSessionID = parentNode.id
	adopted.RootSessionID = rootID
	adopted.SpawnDepth = parentNode.lineage.SpawnDepth + 1
	adopted.Kind = store.LineageKindProvenance
	return m.applyReparent(ctx, &sourceNode, reparentTarget{lineage: adopted})
}

// DetachSession makes a child session top-level again, re-rooting its subtree at
// the session itself. An adopted (provenance) child keeps its type. A stopped
// spawned worker is taken over: the operator's explicit request is the authority
// that ends its spawn governance, so it becomes a root user session without the
// parent-stop binding, TTL, creator wake, spawn budget, or permission narrowing.
func (m *Manager) DetachSession(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	if err := m.requireReparentInputs(ctx); err != nil {
		return nil, err
	}
	source, err := normalizeStoredSessionID(sessionID)
	if err != nil {
		return nil, err
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	sourceNode, err := m.loadReparentNode(ctx, source, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	takeOver, err := validateDetachSource(&sourceNode)
	if err != nil {
		return nil, err
	}
	if err := m.requireReparentUnarchived(ctx, &sourceNode); err != nil {
		return nil, err
	}

	if takeOver {
		return m.applyReparent(ctx, &sourceNode, reparentTarget{
			lineage:     store.SessionLineage{RootSessionID: sourceNode.id, Kind: store.LineageKindRoot},
			sessionType: SessionTypeUser,
		})
	}
	detached := sourceNode.lineage
	detached.ParentSessionID = ""
	detached.RootSessionID = sourceNode.id
	detached.SpawnDepth = 0
	detached.Kind = store.LineageKindRoot
	// A root lineage can never auto-stop on a parent.
	detached.AutoStopOnParent = false
	return m.applyReparent(ctx, &sourceNode, reparentTarget{lineage: detached})
}

func (m *Manager) requireReparentInputs(ctx context.Context) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: reparent context is required")
	}
	if _, err := m.sessionLineageStore(); err != nil {
		return err
	}
	return nil
}

func (m *Manager) sessionLineageStore() (store.SessionLineageStore, error) {
	lineageStore, ok := m.sessionCatalog.(store.SessionLineageStore)
	if !ok || lineageStore == nil {
		return nil, ErrSessionReparentUnavailable
	}
	return lineageStore, nil
}

func validateAdoptSource(node *reparentNode) error {
	if node.sessionType != SessionTypeUser || node.lineage.SpawnRole != "" {
		return reparentRefusal(ReparentRefusalSourceNotUser, node.id, "only user sessions can be adopted")
	}
	if node.lineage.ParentSessionID != "" || node.lineage.Kind != store.LineageKindRoot {
		return reparentRefusal(ReparentRefusalSourceHasParent, node.id, "session already has a parent")
	}
	return nil
}

func validateAdoptParent(source *reparentNode, parent *reparentNode) error {
	if parent.profileID != source.profileID {
		return fmt.Errorf("%w: parent session %s is not in this profile", ErrSessionNotFound, parent.id)
	}
	if parent.sessionType != SessionTypeUser || parent.lineage.SpawnRole != "" {
		return reparentRefusal(
			ReparentRefusalParentIneligible, parent.id, "only user sessions can adopt children",
		)
	}
	return nil
}

// validateDetachSource reports whether detaching node takes over a spawned worker.
func validateDetachSource(node *reparentNode) (bool, error) {
	if node.sessionType == SessionTypeSpawned {
		return true, validateTakeOverSource(node)
	}
	if node.sessionType != SessionTypeUser || node.lineage.SpawnRole != "" {
		return false, reparentRefusal(ReparentRefusalSourceNotUser, node.id, "only user sessions can be made top-level")
	}
	if node.lineage.Kind != store.LineageKindProvenance || node.lineage.ParentSessionID == "" {
		return false, reparentRefusal(
			ReparentRefusalNotDetachable, node.id, "only adopted (provenance) child sessions can be made top-level",
		)
	}
	return false, nil
}

// validateTakeOverSource admits a stopped spawned worker. Subagents stay owned by
// their parent's delegation, and a running worker keeps the governance its
// runtime was launched with until it stops.
func validateTakeOverSource(node *reparentNode) error {
	if node.lineage.SpawnRole == store.SubagentSpawnRole || node.lineage.Kind != store.LineageKindSpawn {
		return reparentRefusal(
			ReparentRefusalSourceNotUser,
			node.id,
			"only spawned workers can be taken over; subagents stay with their parent",
		)
	}
	if node.active != nil {
		return reparentRefusal(
			ReparentRefusalSourceRunning, node.id, "stop the spawned session before taking it over",
		)
	}
	return nil
}

func (m *Manager) requireReparentUnarchived(ctx context.Context, nodes ...*reparentNode) error {
	for _, node := range nodes {
		err := m.requireSessionUnarchived(ctx, node.workspaceID, node.id)
		if errors.Is(err, ErrSessionArchived) {
			return reparentRefusal(ReparentRefusalArchived, node.id, "archived sessions cannot be reparented")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// requireNotAncestor walks the raw lineage parent chain from parent upward and
// refuses when it reaches sourceID: adopting would create a cycle.
func (m *Manager) requireNotAncestor(ctx context.Context, sourceID string, parent *reparentNode) error {
	if parent.lineage.RootSessionID == sourceID {
		return reparentRefusal(ReparentRefusalCycle, parent.id, "the parent is a descendant of the session")
	}
	visited := map[string]struct{}{parent.id: {}}
	next := parent.lineage.ParentSessionID
	for next != "" {
		if next == sourceID {
			return reparentRefusal(ReparentRefusalCycle, parent.id, "the parent is a descendant of the session")
		}
		if _, seen := visited[next]; seen {
			return nil
		}
		visited[next] = struct{}{}
		meta, err := m.readSessionMetaReadOnly(ctx, next)
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("session: resolve ancestor %q: %w", next, err)
		}
		next = store.NormalizeSessionLineage(meta.ID, meta.Lineage).ParentSessionID
	}
	return nil
}
