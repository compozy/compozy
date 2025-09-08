package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrInvalidJSON     = errors.New("invalid json")
)

// ReadRawJSON reads up to limit bytes from r, enforcing bounds and ensuring valid JSON.
// Returns the raw body bytes if within limit and valid JSON; otherwise returns an error.
func ReadRawJSON(r io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit: %d", limit)
	}
	lr := io.LimitedReader{R: r, N: limit + 1}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(&lr); err != nil {
		return nil, err
	}
	if int64(buf.Len()) > limit {
		return nil, ErrPayloadTooLarge
	}
	var js any
	if err := json.Unmarshal(buf.Bytes(), &js); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return buf.Bytes(), nil
}
