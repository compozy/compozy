package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ReadRawJSON reads up to limit bytes from r, enforcing bounds and ensuring valid JSON.
// Returns the raw body bytes if within limit and valid JSON; otherwise returns an error.
func ReadRawJSON(r io.Reader, limit int64) ([]byte, error) {
	lr := io.LimitedReader{R: r, N: limit + 1}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(&lr); err != nil {
		return nil, err
	}
	if int64(buf.Len()) > limit {
		return nil, fmt.Errorf("payload too large")
	}
	var js any
	if err := json.Unmarshal(buf.Bytes(), &js); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return buf.Bytes(), nil
}
