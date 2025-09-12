package attachment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_detectMIME(t *testing.T) {
	t.Run("Should detect PNG from signature", func(t *testing.T) {
		// PNG signature + minimal IHDR chunk header bytes
		head := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82}
		mt := detectMIME(head)
		require.Equal(t, "image/png", mt)
	})
	t.Run("Should fallback to octet-stream for empty", func(t *testing.T) {
		require.Equal(t, "application/octet-stream", detectMIME(nil))
	})
}
