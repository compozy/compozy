package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
)

func decodeSessionListCursor(raw string, fingerprint string, sortKey string) (*store.SessionCatalogPosition, error) {
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
	if sortKey == ListSortName && position.Name == nil {
		return nil, fmt.Errorf("%w: name cursor key is required", ErrListCursorInvalid)
	}
	if sortKey == ListSortNavigator && position.NavigatorBand == nil {
		return nil, fmt.Errorf("%w: navigator cursor band is required", ErrListCursorInvalid)
	}
	if sortKey == ListSortManual && (position.ManualFolderRank == nil || position.ManualPosition == nil) {
		return nil, fmt.Errorf("%w: manual cursor keys are required", ErrListCursorInvalid)
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

func sessionListPageCursor(query ListQuery) (string, *store.SessionCatalogPosition, error) {
	fingerprint, err := sessionListFingerprintForQuery(query)
	if err != nil {
		return "", nil, err
	}
	after, err := decodeSessionListCursor(query.Cursor, fingerprint, query.Sort)
	return fingerprint, after, err
}
