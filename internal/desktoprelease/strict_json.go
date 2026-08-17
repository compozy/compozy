package desktoprelease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func decodeStrictJSON[T any](contents []byte, label string) (T, error) {
	var value T
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("desktop release: decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return value, fmt.Errorf("desktop release: %s contains multiple JSON documents", label)
		}
		return value, fmt.Errorf("desktop release: decode %s trailing data: %w", label, err)
	}
	return value, nil
}
