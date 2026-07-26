package resources

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ResourceKind identifies one canonical desired-state resource family.
type ResourceKind string

// Normalize returns the canonical trimmed resource kind.
func (k ResourceKind) Normalize() ResourceKind {
	return ResourceKind(strings.TrimSpace(string(k)))
}

// Validate reports whether the resource kind is present.
func (k ResourceKind) Validate(path string) error {
	if strings.TrimSpace(string(k)) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	}
	return nil
}

// MutationActorKind identifies the authenticated caller class.
type MutationActorKind string

const (
	// MutationActorKindOperator identifies an operator-authorized control-plane caller.
	MutationActorKindOperator MutationActorKind = "operator"
	// MutationActorKindDaemon identifies a daemon-internal caller.
	MutationActorKindDaemon MutationActorKind = "daemon"
	// MutationActorKindExtension identifies an extension session caller.
	MutationActorKindExtension MutationActorKind = "extension"
)

// Normalize returns the canonical trimmed actor kind.
func (k MutationActorKind) Normalize() MutationActorKind {
	return MutationActorKind(strings.TrimSpace(string(k)))
}

// Validate reports whether the actor kind is supported.
func (k MutationActorKind) Validate(path string) error {
	switch k.Normalize() {
	case MutationActorKindOperator, MutationActorKindDaemon, MutationActorKindExtension:
		return nil
	default:
		return fmt.Errorf(
			"%w: %s must be %q, %q, or %q: %q",
			ErrValidation,
			path,
			MutationActorKindOperator,
			MutationActorKindDaemon,
			MutationActorKindExtension,
			k,
		)
	}
}

// ResourceScopeKind identifies the desired-state visibility scope.
type ResourceScopeKind string

const (
	// ResourceScopeKindGlobal identifies a global-scope record.
	ResourceScopeKindGlobal ResourceScopeKind = "global"
	// ResourceScopeKindWorkspace identifies a workspace-scope record.
	ResourceScopeKindWorkspace ResourceScopeKind = "workspace"
)

// Normalize returns the canonical trimmed scope kind.
func (k ResourceScopeKind) Normalize() ResourceScopeKind {
	return ResourceScopeKind(strings.TrimSpace(string(k)))
}

// Validate reports whether the scope kind is supported.
func (k ResourceScopeKind) Validate(path string) error {
	switch k.Normalize() {
	case ResourceScopeKindGlobal, ResourceScopeKindWorkspace:
		return nil
	default:
		return fmt.Errorf(
			"%w: %s must be %q or %q: %q",
			ErrValidation,
			path,
			ResourceScopeKindGlobal,
			ResourceScopeKindWorkspace,
			k,
		)
	}
}

// ResourceScope describes the persistence scope for one record.
type ResourceScope struct {
	Kind ResourceScopeKind `json:"kind"`
	ID   string            `json:"id,omitempty"`
}

// Normalize returns a trimmed scope value.
func (s ResourceScope) Normalize() ResourceScope {
	return ResourceScope{
		Kind: s.Kind.Normalize(),
		ID:   strings.TrimSpace(s.ID),
	}
}

// Validate reports whether the scope binding is internally consistent.
func (s ResourceScope) Validate(path string) error {
	scopePath := nestedPath(path, "kind")
	if err := s.Kind.Validate(scopePath); err != nil {
		return err
	}

	idPath := nestedPath(path, "id")
	switch s.Kind.Normalize() {
	case ResourceScopeKindGlobal:
		if strings.TrimSpace(s.ID) != "" {
			return fmt.Errorf(
				"%w: %s must be empty when %s is %q",
				ErrInvalidScopeBinding,
				idPath,
				scopePath,
				ResourceScopeKindGlobal,
			)
		}
	case ResourceScopeKindWorkspace:
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf(
				"%w: %s is required when %s is %q",
				ErrInvalidScopeBinding,
				idPath,
				scopePath,
				ResourceScopeKindWorkspace,
			)
		}
	}

	return nil
}

// ResourceSourceKind identifies the stamped source family for a record.
type ResourceSourceKind string

// Normalize returns the canonical trimmed source kind.
func (k ResourceSourceKind) Normalize() ResourceSourceKind {
	return ResourceSourceKind(strings.TrimSpace(string(k)))
}

// ResourceSource identifies the stamped canonical source for a record.
type ResourceSource struct {
	Kind ResourceSourceKind `json:"kind"`
	ID   string             `json:"id"`
}

// Normalize returns a trimmed source value.
func (s ResourceSource) Normalize() ResourceSource {
	return ResourceSource{
		Kind: s.Kind.Normalize(),
		ID:   strings.TrimSpace(s.ID),
	}
}

// Validate reports whether the source is present.
func (s ResourceSource) Validate(path string) error {
	if strings.TrimSpace(string(s.Kind)) == "" {
		return fmt.Errorf("%w: %s.kind is required", ErrValidation, path)
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: %s.id is required", ErrValidation, path)
	}
	return nil
}

// ResourceOwnerKind identifies the stamped ownership family for a record.
type ResourceOwnerKind string

// Normalize returns the canonical trimmed owner kind.
func (k ResourceOwnerKind) Normalize() ResourceOwnerKind {
	return ResourceOwnerKind(strings.TrimSpace(string(k)))
}

// ResourceOwner identifies the stamped owner for a record.
type ResourceOwner struct {
	Kind ResourceOwnerKind `json:"kind"`
	ID   string            `json:"id"`
}

// Normalize returns a trimmed owner value.
func (o ResourceOwner) Normalize() ResourceOwner {
	return ResourceOwner{
		Kind: o.Kind.Normalize(),
		ID:   strings.TrimSpace(o.ID),
	}
}

// Validate reports whether the owner is present.
func (o ResourceOwner) Validate(path string) error {
	if strings.TrimSpace(string(o.Kind)) == "" {
		return fmt.Errorf("%w: %s.kind is required", ErrValidation, path)
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("%w: %s.id is required", ErrValidation, path)
	}
	return nil
}

// MutationActor describes the authoritative caller boundary for one mutation or read.
type MutationActor struct {
	Kind          MutationActorKind   `json:"kind"`
	ID            string              `json:"id"`
	SessionNonce  string              `json:"session_nonce,omitempty"`
	Owner         ResourceOwner       `json:"owner"`
	Source        ResourceSource      `json:"source"`
	MaxScope      ResourceScope       `json:"max_scope"`
	GrantedKinds  []ResourceKind      `json:"granted_kinds,omitempty"`
	GrantedScopes []ResourceScopeKind `json:"granted_scopes,omitempty"`
}

// RawDraft carries one raw desired-state mutation at the persistence boundary.
type RawDraft struct {
	Kind            ResourceKind
	ID              string
	Scope           ResourceScope
	ExpectedVersion int64
	SpecJSON        []byte
}

// SourceSnapshot carries the full desired-state snapshot for one source session.
type SourceSnapshot struct {
	SourceVersion int64
	Records       []RawDraft
}

// RawRecord is the persisted raw desired-state shape.
type RawRecord struct {
	Kind      ResourceKind
	ID        string
	Version   int64
	Scope     ResourceScope
	Owner     ResourceOwner
	Source    ResourceSource
	SpecJSON  []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ResourceFilter narrows list operations at the persistence boundary.
type ResourceFilter struct {
	Kind   ResourceKind
	Scope  *ResourceScope
	Owner  *ResourceOwner
	Source *ResourceSource
	Limit  int
}

// RawStore defines the raw CRUD plus snapshot boundary for desired-state persistence.
type RawStore interface {
	PutRaw(ctx context.Context, actor MutationActor, draft RawDraft) (RawRecord, error)
	DeleteRaw(ctx context.Context, actor MutationActor, kind ResourceKind, id string, expectedVersion int64) error
	ApplySourceSnapshotRaw(ctx context.Context, actor MutationActor, snapshot SourceSnapshot) error
	GetRaw(ctx context.Context, actor MutationActor, kind ResourceKind, id string) (RawRecord, error)
	ListRaw(ctx context.Context, actor MutationActor, filter ResourceFilter) ([]RawRecord, error)
}

// SourceSessionManager manages active source-session state for snapshot publication.
type SourceSessionManager interface {
	ActivateSourceSession(ctx context.Context, actor MutationActor, source ResourceSource, sessionNonce string) error
	ResetSource(ctx context.Context, actor MutationActor, source ResourceSource) error
}
