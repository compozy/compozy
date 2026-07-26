package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func decodeStrictContractJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
