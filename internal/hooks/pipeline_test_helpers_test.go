package hooks

import (
	"bytes"
	"encoding/json"
)

func decodeJSON[T any](payload []byte) (T, error) {
	var decoded T
	if len(bytes.TrimSpace(payload)) == 0 {
		return decoded, nil
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return decoded, err
	}
	return decoded, nil
}
