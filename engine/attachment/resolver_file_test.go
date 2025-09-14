package attachment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_resolveFile(t *testing.T) {
	t.Run("Should error when CWD is nil", func(t *testing.T) {
		a := &FileAttachment{Path: "some.txt"}
		_, err := resolveFile(context.Background(), a, nil)
		require.Error(t, err)
	})
}
