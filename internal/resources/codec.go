package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// KindCodec owns the typed encode/decode boundary for one resource kind.
type KindCodec[T any] interface {
	Kind() ResourceKind
	DecodeAndValidate(ctx context.Context, scope ResourceScope, raw []byte) (T, error)
	Encode(spec T) ([]byte, error)
	MaxBytes() int
}

// SpecValidator enforces typed invariants after decoding and before persistence.
type SpecValidator[T any] func(ctx context.Context, scope ResourceScope, spec T) (T, error)

type jsonCodec[T any] struct {
	kind      ResourceKind
	maxBytes  int
	validator SpecValidator[T]
}

type rawSpecCodec interface {
	Kind() ResourceKind
	MaxBytes() int
	ValidateAndCanonicalizeRaw(ctx context.Context, scope ResourceScope, raw []byte) ([]byte, error)
}

type rawCodecAdapter[T any] struct {
	codec KindCodec[T]
}

func (a rawCodecAdapter[T]) Kind() ResourceKind {
	return a.codec.Kind()
}

func (a rawCodecAdapter[T]) MaxBytes() int {
	return a.codec.MaxBytes()
}

func (a rawCodecAdapter[T]) ValidateAndCanonicalizeRaw(
	ctx context.Context,
	scope ResourceScope,
	raw []byte,
) ([]byte, error) {
	validated, err := a.codec.DecodeAndValidate(ctx, scope, raw)
	if err != nil {
		return nil, err
	}
	canonical, err := a.codec.Encode(validated)
	if err != nil {
		return nil, err
	}
	if err := validateCodecPayloadSize(len(canonical), a.codec.MaxBytes(), a.codec.Kind(), "encode"); err != nil {
		return nil, err
	}
	return canonical, nil
}

// NewJSONCodec builds a JSON-backed codec with a typed validation hook.
func NewJSONCodec[T any](kind ResourceKind, maxBytes int, validator SpecValidator[T]) (KindCodec[T], error) {
	normalizedKind := kind.Normalize()
	if err := normalizedKind.Validate("codec.kind"); err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("%w: codec.max_bytes must be positive: %d", ErrValidation, maxBytes)
	}

	return &jsonCodec[T]{
		kind:      normalizedKind,
		maxBytes:  maxBytes,
		validator: validator,
	}, nil
}

func (c *jsonCodec[T]) Kind() ResourceKind {
	return c.kind
}

func (c *jsonCodec[T]) MaxBytes() int {
	return c.maxBytes
}

func (c *jsonCodec[T]) Encode(spec T) ([]byte, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("resources: encode %q spec: %w", c.kind, err)
	}
	if err := validateCodecPayloadSize(len(encoded), c.maxBytes, c.kind, "encode"); err != nil {
		return nil, err
	}
	return append([]byte(nil), encoded...), nil
}

func (c *jsonCodec[T]) DecodeAndValidate(ctx context.Context, scope ResourceScope, raw []byte) (T, error) {
	var zero T
	if ctx == nil {
		return zero, errors.New("resources: codec decode context is required")
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return zero, fmt.Errorf("%w: codec payload is required for kind %q", ErrValidation, c.kind)
	}
	if err := validateCodecPayloadSize(len(trimmed), c.maxBytes, c.kind, "decode"); err != nil {
		return zero, err
	}

	var spec T
	if err := json.Unmarshal(trimmed, &spec); err != nil {
		return zero, fmt.Errorf("resources: decode %q spec: %w: %w", c.kind, ErrValidation, err)
	}
	if c.validator == nil {
		return spec, nil
	}
	validated, err := c.validator(ctx, scope, spec)
	if err != nil {
		return zero, wrapCodecValidationError(c.kind, err)
	}
	return validated, nil
}

func (c *jsonCodec[T]) ValidateAndCanonicalizeRaw(
	ctx context.Context,
	scope ResourceScope,
	raw []byte,
) ([]byte, error) {
	validated, err := c.DecodeAndValidate(ctx, scope, raw)
	if err != nil {
		return nil, err
	}
	return c.Encode(validated)
}

func validateCodecPayloadSize(size int, maxBytes int, kind ResourceKind, operation string) error {
	if size > maxBytes {
		return fmt.Errorf(
			"%w: %s %q payload exceeds %d bytes: %d",
			ErrPayloadTooLarge,
			operation,
			kind,
			maxBytes,
			size,
		)
	}
	return nil
}

func wrapCodecValidationError(kind ResourceKind, err error) error {
	if err == nil {
		return nil
	}
	if isClassifiedResourceError(err) {
		return fmt.Errorf("resources: validate %q spec: %w", kind, err)
	}
	return fmt.Errorf("resources: validate %q spec: %w: %w", kind, ErrValidation, err)
}

func isClassifiedResourceError(err error) bool {
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrValidation) ||
		errors.Is(err, ErrInvalidScopeBinding) ||
		errors.Is(err, ErrPermissionDenied) ||
		errors.Is(err, ErrDirectMutationNotAllowed) ||
		errors.Is(err, ErrConflict) ||
		errors.Is(err, ErrPayloadTooLarge) ||
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrSessionNotActive) ||
		errors.Is(err, ErrStaleSourceVersion) ||
		errors.Is(err, ErrCodecNotFound) ||
		errors.Is(err, ErrCodecTypeMismatch)
}

type codecRegistration struct {
	specType       reflect.Type
	codec          any
	rawCanonicaler rawSpecCodec
}

// CodecRegistry holds explicit kind-to-codec registrations for typed adapters.
type CodecRegistry struct {
	mu     sync.RWMutex
	codecs map[ResourceKind]codecRegistration
}

// NewCodecRegistry constructs an empty kind codec registry.
func NewCodecRegistry() *CodecRegistry {
	return &CodecRegistry{
		codecs: make(map[ResourceKind]codecRegistration),
	}
}

// ResolveCodec returns the typed codec registered for one resource kind.
func ResolveCodec[T any](registry *CodecRegistry, kind ResourceKind) (KindCodec[T], error) {
	if registry == nil {
		return nil, errors.New("resources: codec registry is required")
	}

	normalizedKind := kind.Normalize()
	if err := normalizedKind.Validate("kind"); err != nil {
		return nil, err
	}

	registry.mu.RLock()
	entry, ok := registry.codecs[normalizedKind]
	registry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: codec not registered for kind %q", ErrCodecNotFound, normalizedKind)
	}

	codec, ok := entry.codec.(KindCodec[T])
	if !ok {
		return nil, fmt.Errorf(
			"%w: codec for kind %q is %s, not %s",
			ErrCodecTypeMismatch,
			normalizedKind,
			entry.specType,
			specTypeOf[T](),
		)
	}
	return codec, nil
}

// RegisterCodec adds one typed codec keyed by its resource kind.
func RegisterCodec[T any](registry *CodecRegistry, codec KindCodec[T]) error {
	if registry == nil {
		return errors.New("resources: codec registry is required")
	}

	normalizedKind, err := validateCodec(codec)
	if err != nil {
		return err
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.codecs[normalizedKind]; exists {
		return fmt.Errorf("%w: codec already registered for kind %q", ErrConflict, normalizedKind)
	}

	registry.codecs[normalizedKind] = codecRegistration{
		specType:       specTypeOf[T](),
		codec:          codec,
		rawCanonicaler: rawCodecAdapter[T]{codec: codec},
	}
	return nil
}

func validateCodec[T any](codec KindCodec[T]) (ResourceKind, error) {
	if codec == nil {
		return "", errors.New("resources: codec is required")
	}

	normalizedKind := codec.Kind().Normalize()
	if err := normalizedKind.Validate("codec.kind"); err != nil {
		return "", err
	}
	if codec.MaxBytes() <= 0 {
		return "", fmt.Errorf("%w: codec.max_bytes must be positive: %d", ErrValidation, codec.MaxBytes())
	}
	return normalizedKind, nil
}

func specTypeOf[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}

// ValidateAndCanonicalizeIfRegistered validates one raw spec against a registered
// codec and returns its canonical encoded form. When the kind has no registered
// codec yet, the original payload is returned unchanged and validated is false.
func ValidateAndCanonicalizeIfRegistered(
	ctx context.Context,
	registry *CodecRegistry,
	kind ResourceKind,
	scope ResourceScope,
	raw []byte,
) ([]byte, bool, error) {
	if registry == nil {
		return append([]byte(nil), raw...), false, nil
	}

	normalizedKind := kind.Normalize()
	if err := normalizedKind.Validate("kind"); err != nil {
		return nil, false, err
	}

	registry.mu.RLock()
	entry, ok := registry.codecs[normalizedKind]
	registry.mu.RUnlock()
	if !ok {
		return append([]byte(nil), raw...), false, nil
	}

	canonical, err := entry.rawCanonicaler.ValidateAndCanonicalizeRaw(ctx, scope, raw)
	if err != nil {
		return nil, true, err
	}
	return canonical, true, nil
}
