package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// DigestFile computes the lowercase SHA-256 digest required by extension feed entries.
func DigestFile(path string) (digest string, err error) {
	artifact, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("marketplace catalog: open artifact %q: %w", path, err)
	}
	defer func() {
		if closeErr := artifact.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("marketplace catalog: close artifact %q: %w", path, closeErr))
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, artifact); err != nil {
		return "", fmt.Errorf("marketplace catalog: hash artifact %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
