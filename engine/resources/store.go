package resources

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// ResourceType identifies the category of a stored resource.
// Values align with existing engine core config types, with additional types like "model".
// ResourceType aliases core.ConfigType to avoid type drift across packages.
// Additional resource-specific types (e.g., schema, model) are defined below.
type ResourceType = core.ConfigType

const (
	ResourceProject  ResourceType = core.ConfigProject
	ResourceWorkflow ResourceType = core.ConfigWorkflow
	ResourceTask     ResourceType = core.ConfigTask
	ResourceAgent    ResourceType = core.ConfigAgent
	ResourceTool     ResourceType = core.ConfigTool
	ResourceMCP      ResourceType = core.ConfigMCP
	ResourceMemory   ResourceType = core.ConfigMemory
	// Resource-specific extensions not yet in core:
	ResourceSchema ResourceType = "schema"
	ResourceModel  ResourceType = "model"
	// ResourceMeta stores provenance or auxiliary metadata for resources.
	// Not exposed via public HTTP router; used by importers/admin tooling.
	ResourceMeta ResourceType = "meta"
)

// ResourceKey uniquely identifies a resource within a project and type.
// Version is optional and reserved for future pinning semantics.
type ResourceKey struct {
	Project string       `json:"project"`
	Type    ResourceType `json:"type"`
	ID      string       `json:"id"`
	Version string       `json:"version,omitempty"`
}

// EventType enumerates supported store events.
type EventType string

const (
	EventPut    EventType = "put"
	EventDelete EventType = "delete"
)

// Event describes a change in the store for watchers.
// ETag is a deterministic content hash for the affected value.
type Event struct {
	Type EventType   `json:"type"`
	Key  ResourceKey `json:"key"`
	ETag string      `json:"etag"`
	At   time.Time   `json:"at"`
}

// StoredItem bundles a key with its stored value and ETag for bulk operations.
type StoredItem struct {
	Key   ResourceKey `json:"key"`
	Value any         `json:"value"`
	ETag  string      `json:"etag"`
}

// ResourceStore is the contract for storing and linking referencable resources.
// Implementations must be safe for concurrent use.
//
// Value is intentionally typed as any to allow storing concrete config structs.
// Implementers should deep-copy values on Put/Get to avoid shared state.
type ResourceStore interface {
	// Put inserts or replaces a resource value at the given key.
	// Returns a deterministic ETag for the stored value.
	Put(ctx context.Context, key ResourceKey, value any) (etag string, err error)

	// Get retrieves a resource by key. If not found, returns (nil, "", ErrNotFound).
	// Implementations should return a deep copy of the stored value.
	Get(ctx context.Context, key ResourceKey) (value any, etag string, err error)

	// Delete removes a resource by key. Deleting a missing key must be idempotent.
	Delete(ctx context.Context, key ResourceKey) error

	// List returns available keys for a given project and type.
	List(ctx context.Context, project string, typ ResourceType) ([]ResourceKey, error)

	// Watch streams store events for a project and type until ctx is done.
	// On subscription, implementations may choose to emit synthetic PUT events
	// for current items to prime caches.
	Watch(ctx context.Context, project string, typ ResourceType) (<-chan Event, error)

	// ListWithValues returns keys and values (with ETags) for a given project and type
	// in a batched fashion. Implementations should minimize round-trips to the backend.
	ListWithValues(ctx context.Context, project string, typ ResourceType) ([]StoredItem, error)

	// ListWithValuesPage returns a paginated slice of StoredItem and the total count
	// of items for the given project and type. Implementations may fetch all values
	// and slice, or apply efficient backend paging where possible.
	ListWithValuesPage(
		ctx context.Context,
		project string,
		typ ResourceType,
		offset, limit int,
	) ([]StoredItem, int, error)

	// Close releases underlying resources.
	Close() error
}

// ErrNotFound is returned by Get when a resource key does not exist.
var ErrNotFound = errors.New("resource not found")
