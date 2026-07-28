package extensionpkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func decodeHostAPIParams(raw json.RawMessage, target any) error {
	return decodeHostAPIParamsWithUnknownFields(raw, target, true)
}

func decodeHostAPIParamsStrict(raw json.RawMessage, target any) error {
	return decodeHostAPIParamsWithUnknownFields(raw, target, true)
}

func decodeHostAPIParamsWithUnknownFields(raw json.RawMessage, target any, disallowUnknown bool) error {
	if target == nil {
		return errors.New("extension: host api params target is required")
	}
	payload := bytes.TrimSpace(raw)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		payload = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return invalidParamsRPCError(fmt.Errorf("decode params: %w", err))
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return invalidParamsRPCError(errors.New("params must contain one JSON value"))
		}
		return invalidParamsRPCError(fmt.Errorf("decode trailing params: %w", err))
	}
	return nil
}
