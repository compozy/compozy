package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Draft is the typed desired-state mutation shape exposed to domain code.
type Draft[T any] struct {
	ID              string
	Scope           ResourceScope
	ExpectedVersion int64
	Spec            T
}

// Record is the typed desired-state record shape exposed to domain code.
type Record[T any] struct {
	Kind      ResourceKind
	ID        string
	Version   int64
	Scope     ResourceScope
	Owner     ResourceOwner
	Source    ResourceSource
	Spec      T
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store is the typed CRUD façade used by domain code.
type Store[T any] interface {
	Put(ctx context.Context, actor MutationActor, draft Draft[T]) (Record[T], error)
	Delete(ctx context.Context, actor MutationActor, id string, expectedVersion int64) error
	Get(ctx context.Context, actor MutationActor, id string) (Record[T], error)
	List(ctx context.Context, actor MutationActor, filter ResourceFilter) ([]Record[T], error)
}

type typedStore[T any] struct {
	raw   RawStore
	codec KindCodec[T]
}

// NewStore constructs a typed store façade over the raw persistence kernel.
func NewStore[T any](raw RawStore, codec KindCodec[T]) (Store[T], error) {
	if raw == nil {
		return nil, errors.New("resources: raw store is required")
	}
	if _, err := validateCodec(codec); err != nil {
		return nil, err
	}

	return &typedStore[T]{
		raw:   raw,
		codec: codec,
	}, nil
}

func (s *typedStore[T]) Put(ctx context.Context, actor MutationActor, draft Draft[T]) (Record[T], error) {
	if ctx == nil {
		return Record[T]{}, errors.New("resources: typed put context is required")
	}

	normalizedDraft, err := normalizeTypedDraft(draft)
	if err != nil {
		return Record[T]{}, err
	}

	specJSON, err := encodeValidatedSpec(ctx, s.codec, normalizedDraft.Scope, normalizedDraft.Spec)
	if err != nil {
		return Record[T]{}, err
	}

	rawRecord, err := s.raw.PutRaw(ctx, actor, RawDraft{
		Kind:            s.codec.Kind(),
		ID:              normalizedDraft.ID,
		Scope:           normalizedDraft.Scope,
		ExpectedVersion: normalizedDraft.ExpectedVersion,
		SpecJSON:        specJSON,
	})
	if err != nil {
		return Record[T]{}, err
	}

	return decodeTypedRecord(ctx, s.codec, rawRecord)
}

func (s *typedStore[T]) Delete(ctx context.Context, actor MutationActor, id string, expectedVersion int64) error {
	return s.raw.DeleteRaw(ctx, actor, s.codec.Kind(), id, expectedVersion)
}

func (s *typedStore[T]) Get(ctx context.Context, actor MutationActor, id string) (Record[T], error) {
	rawRecord, err := s.raw.GetRaw(ctx, actor, s.codec.Kind(), id)
	if err != nil {
		return Record[T]{}, err
	}
	return decodeTypedRecord(ctx, s.codec, rawRecord)
}

func (s *typedStore[T]) List(ctx context.Context, actor MutationActor, filter ResourceFilter) ([]Record[T], error) {
	if filter.Kind != "" && filter.Kind.Normalize() != s.codec.Kind() {
		return nil, fmt.Errorf(
			"%w: typed store for kind %q cannot list filter kind %q",
			ErrValidation,
			s.codec.Kind(),
			filter.Kind,
		)
	}

	filter.Kind = s.codec.Kind()
	rawRecords, err := s.raw.ListRaw(ctx, actor, filter)
	if err != nil {
		return nil, err
	}

	records := make([]Record[T], 0, len(rawRecords))
	for _, rawRecord := range rawRecords {
		record, decodeErr := decodeTypedRecord(ctx, s.codec, rawRecord)
		if decodeErr != nil {
			return nil, decodeErr
		}
		records = append(records, record)
	}
	return records, nil
}

func normalizeTypedDraft[T any](draft Draft[T]) (Draft[T], error) {
	normalized := draft
	normalized.ID = strings.TrimSpace(draft.ID)
	normalized.Scope = draft.Scope.Normalize()

	if normalized.ID == "" {
		return Draft[T]{}, fmt.Errorf("%w: draft.id is required", ErrValidation)
	}
	if err := normalized.Scope.Validate("draft.scope"); err != nil {
		return Draft[T]{}, err
	}
	if normalized.ExpectedVersion < 0 {
		return Draft[T]{}, fmt.Errorf(
			"%w: draft.expected_version cannot be negative: %d",
			ErrValidation,
			normalized.ExpectedVersion,
		)
	}

	return normalized, nil
}

func encodeValidatedSpec[T any](
	ctx context.Context,
	codec KindCodec[T],
	scope ResourceScope,
	spec T,
) ([]byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return nil, err
	}

	validated, err := codec.DecodeAndValidate(ctx, scope, encoded)
	if err != nil {
		return nil, err
	}

	canonical, err := codec.Encode(validated)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func decodeTypedRecord[T any](ctx context.Context, codec KindCodec[T], rawRecord RawRecord) (Record[T], error) {
	if rawRecord.Kind.Normalize() != codec.Kind() {
		return Record[T]{}, fmt.Errorf(
			"%w: record kind %q does not match codec kind %q",
			ErrValidation,
			rawRecord.Kind,
			codec.Kind(),
		)
	}

	spec, err := codec.DecodeAndValidate(ctx, rawRecord.Scope, rawRecord.SpecJSON)
	if err != nil {
		return Record[T]{}, fmt.Errorf("resources: decode record %q/%q: %w", rawRecord.Kind, rawRecord.ID, err)
	}

	return Record[T]{
		Kind:      rawRecord.Kind,
		ID:        rawRecord.ID,
		Version:   rawRecord.Version,
		Scope:     rawRecord.Scope,
		Owner:     rawRecord.Owner,
		Source:    rawRecord.Source,
		Spec:      spec,
		CreatedAt: rawRecord.CreatedAt,
		UpdatedAt: rawRecord.UpdatedAt,
	}, nil
}

func decodeTypedRecords[T any](ctx context.Context, codec KindCodec[T], rawRecords []RawRecord) ([]Record[T], error) {
	records := make([]Record[T], 0, len(rawRecords))
	for _, rawRecord := range rawRecords {
		record, err := decodeTypedRecord(ctx, codec, rawRecord)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
