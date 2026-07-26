// Package listcursor provides versioned, query-bound opaque cursors for stable list pagination.
package listcursor

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// DefaultMaxEncodedSize bounds cursors accepted from public inputs.
	DefaultMaxEncodedSize = 4096
)

var (
	// ErrInvalid reports malformed cursors and cursors reused with a different query contract.
	ErrInvalid = errors.New("list cursor is invalid")
)

type envelope[T any] struct {
	Version     int    `json:"v"`
	Kind        string `json:"k"`
	Fingerprint string `json:"f"`
	Position    T      `json:"p"`
}

// Fingerprint returns a deterministic SHA-256 digest for normalized query state.
func Fingerprint(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("list cursor: encode fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

// Encode serializes a typed cursor position with its version, kind, and query fingerprint.
func Encode[T any](version int, kind string, fingerprint string, position T) (string, error) {
	encoded, err := json.Marshal(envelope[T]{
		Version:     version,
		Kind:        kind,
		Fingerprint: fingerprint,
		Position:    position,
	})
	if err != nil {
		return "", fmt.Errorf("list cursor: encode: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

// Decode parses a typed cursor and verifies its version, kind, and query fingerprint.
func Decode[T any](
	raw string,
	version int,
	kind string,
	fingerprint string,
	maxEncodedSize int,
) (T, error) {
	var zero T
	if maxEncodedSize <= 0 {
		maxEncodedSize = DefaultMaxEncodedSize
	}
	if len(raw) > maxEncodedSize {
		return zero, fmt.Errorf("%w: encoded value is too large", ErrInvalid)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return zero, fmt.Errorf("%w: decode: %w", ErrInvalid, err)
	}
	var cursor envelope[T]
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return zero, fmt.Errorf("%w: parse: %w", ErrInvalid, err)
	}
	if cursor.Version != version || cursor.Kind != kind || cursor.Fingerprint != fingerprint {
		return zero, fmt.Errorf("%w: cursor does not match this query", ErrInvalid)
	}
	return cursor.Position, nil
}
