package definitiondigest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Sum requires an installation key for identities that can outlive a process.
func Sum(data, key []byte) (string, error) {
	if len(key) == 0 {
		return "", errors.New("MCP definition digest key is required")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("compozy:mcp-definition:v1\x00"))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}
