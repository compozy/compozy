// Package contracts owns structured result contracts without depending on a
// runtime, persistence implementation, or consumer domain.
package contracts

import (
	"context"
	"encoding/json"
)

// Contract is the immutable, schema-only identity stored by a RegistryStore.
type Contract struct {
	Digest string
	Schema json.RawMessage
}

// OverflowMode declares how a result consumer handles an over-budget payload.
type OverflowMode string

const (
	// OverflowStore retains the complete payload and projects a bounded preview.
	OverflowStore OverflowMode = "store"
	// OverflowReject rejects a payload that exceeds the declared budget.
	OverflowReject OverflowMode = "reject"
)

// ByteBudget is an immutable per-consumer result budget.
type ByteBudget struct {
	MaxBytes int
	Overflow OverflowMode
}

// CallsResultsConfig is the parsed calls.results budget policy.
type CallsResultsConfig struct {
	DefaultBudget ByteBudget
	MaxBudget     int
}

// BudgetOutcome reports the complete payload and its bounded projection.
type BudgetOutcome struct {
	Payload    json.RawMessage
	Preview    string
	Overflowed bool
}

// ValidationIssue is one validator issue with a JSONPath-like instance path.
type ValidationIssue struct {
	Path    string
	Message string
}

// Verdict reports whether a payload satisfied a contract.
type Verdict struct {
	Valid     bool
	Issues    []ValidationIssue
	Unwrapped bool
}

// Redaction describes one secret-bearing value removed from untrusted text.
type Redaction struct {
	Path        string
	Fingerprint string
}

// Registry pins, resolves, and validates immutable schema identities.
type Registry interface {
	Pin(ctx context.Context, schema json.RawMessage) (Contract, error)
	Resolve(ctx context.Context, digest string) (Contract, error)
	Validate(ctx context.Context, digest string, payload json.RawMessage) (Verdict, error)
}

// RegistryStore persists immutable canonical schema rows.
type RegistryStore interface {
	PutContract(ctx context.Context, contract Contract) error
	GetContract(ctx context.Context, digest string) (Contract, error)
}

// EntityKind identifies a repository entity referenced by a schema annotation.
type EntityKind string

const (
	EntityAgent     EntityKind = "agent"
	EntitySkill     EntityKind = "skill"
	EntityLoop      EntityKind = "loop"
	EntityWorktree  EntityKind = "worktree"
	EntitySession   EntityKind = "session"
	EntityWorkspace EntityKind = "workspace"
	EntitySecret    EntityKind = "secret"
)

// EntityCatalog resolves x-compozy-kind values without coupling contracts to a domain.
type EntityCatalog interface {
	EntityExists(ctx context.Context, kind EntityKind, value string) (bool, error)
}
