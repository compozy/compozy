package orchestrator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/compozy/compozy/engine/core"
)

var fingerprintPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}

func stableJSONFingerprint(raw []byte) string {
	if len(raw) == 0 || !json.Valid(raw) {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:])
	}
	pooled := fingerprintPool.Get()
	buf, ok := pooled.(*bytes.Buffer)
	if !ok {
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	defer fingerprintPool.Put(buf)
	core.WriteStableJSON(buf, v)
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}
