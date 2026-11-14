package definitiondigest

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

var ephemeralKey = sync.OnceValue(func() []byte { return []byte(rand.Text()) })

// Sum uses an installation key, or a process-local key for readers without persistent state.
func Sum(data, key []byte) string {
	if len(key) == 0 {
		key = ephemeralKey()
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("compozy:mcp-definition:v1\x00"))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
