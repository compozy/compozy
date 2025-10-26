package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/test/helpers"
)

func TestBuilderBuildSuccess(t *testing.T) {
	ctx := helpers.NewTestContext(t)
	b := New("http://localhost:3100").
		WithAPIKey("secret").
		WithTimeout(12 * time.Second)

	client, err := b.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, "secret", client.apiKey)
	require.Equal(t, 12*time.Second, client.httpClient.Timeout)
	require.Contains(t, client.baseURL, "/api/")
}

func TestBuilderValidationErrors(t *testing.T) {
	ctx := helpers.NewTestContext(t)
	t.Run("empty endpoint", func(t *testing.T) {
		_, err := New(" ").Build(ctx)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
	t.Run("invalid timeout", func(t *testing.T) {
		_, err := New("http://localhost:3000").WithTimeout(0).Build(ctx)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
	})
}
