package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
)

func decodeSessionListCursor(raw string, fingerprint string) (*store.SessionCatalogPosition, error) {
	if raw == "" {
		return nil, nil
	}
	position, err := listcursor.Decode[store.SessionCatalogPosition](
		raw,
		sessionListCursorVersion,
		sessionListCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	if err := position.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	position.PrimaryAt = position.PrimaryAt.UTC()
	position.SecondaryAt = position.SecondaryAt.UTC()
	position.CreatedAt = position.CreatedAt.UTC()
	position.ID = strings.TrimSpace(position.ID)
	return &position, nil
}

func encodeSessionListCursor(fingerprint string, position store.SessionCatalogPosition) (string, error) {
	encoded, err := listcursor.Encode(
		sessionListCursorVersion,
		sessionListCursorKind,
		fingerprint,
		position,
	)
	if err != nil {
		return "", fmt.Errorf("session: encode list cursor: %w", err)
	}
	return encoded, nil
}
