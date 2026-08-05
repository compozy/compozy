package bridgesdk

import (
	"bytes"
	"encoding/json"

	"github.com/compozy/compozy/internal/subprocess"
)

func decodeParams(raw json.RawMessage, dest any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: err.Error(),
		})
	}
	return nil
}
