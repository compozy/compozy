package loop

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func encodeRosterCursor(cursor rosterCursor) string {
	raw := fmt.Appendf(nil, `{"offset":%d}`, cursor.Offset)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeRosterCursor(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, fmt.Errorf("%w: malformed roster cursor", ErrInvalidRosterCursor)
	}
	var cursor rosterCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Offset < 0 {
		return 0, fmt.Errorf("%w: malformed roster cursor", ErrInvalidRosterCursor)
	}
	return cursor.Offset, nil
}
