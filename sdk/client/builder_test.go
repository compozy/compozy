package client

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/testutil"
)

func TestBuilderBuildSuccess(t *testing.T) {
	t.Parallel()
	ctx := testutil.NewTestContext(t)
	builder := New("http://localhost:3100").WithAPIKey("secret").WithTimeout(12 * time.Second)
	client, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, "secret", client.apiKey)
	require.Equal(t, 12*time.Second, client.httpClient.Timeout)
	require.Equal(t, "http://localhost:3100/api/v0", client.baseURL)
	require.Equal(t, "http://localhost:3100", client.rawBase)
}

func TestBuilderBuildSuccessWhenPathAlreadyIncludesAPIBase(t *testing.T) {
	t.Parallel()
	ctx := testutil.NewTestContext(t)
	builder := New("https://example.com/api/v1").WithTimeout(45 * time.Second)
	client, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, 45*time.Second, client.httpClient.Timeout)
	require.Equal(t, "https://example.com/api/v1/api/v0", client.baseURL)
}

func TestBuilderValidationErrors(t *testing.T) {
	t.Parallel()
	tests := []testutil.TableTest{
		{
			Name:        "empty endpoint",
			WantErr:     true,
			ErrContains: "endpoint",
			BuildFunc: func(ctx context.Context) (any, error) {
				return New(" ").Build(ctx)
			},
		},
		{
			Name:        "invalid timeout",
			WantErr:     true,
			ErrContains: "timeout",
			BuildFunc: func(ctx context.Context) (any, error) {
				return New("http://localhost:3000").WithTimeout(0).Build(ctx)
			},
		},
		{
			Name:        "invalid url",
			WantErr:     true,
			ErrContains: "url must be valid",
			BuildFunc: func(ctx context.Context) (any, error) {
				return New(":// bad url").Build(ctx)
			},
		},
	}
	testutil.RunTableTests(t, tests)
}

func TestBuilderBuildErrorsWhenContextMissing(t *testing.T) {
	t.Parallel()
	builder := New("http://localhost:3100")
	client, err := builder.Build(nil)
	require.Error(t, err)
	require.Nil(t, client)
}

func TestBuilderBuildReturnsBuildError(t *testing.T) {
	t.Parallel()
	ctx := testutil.NewTestContext(t)
	client, err := New("http://localhost:3100").WithTimeout(-1).Build(ctx)
	require.Error(t, err)
	require.Nil(t, client)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
}

func TestBuilderBuildHandlesNilReceiver(t *testing.T) {
	t.Parallel()
	var builder *Builder
	ctx := testutil.NewTestContext(t)
	client, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, client)
}

func TestBuildBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "https://example.com",
			expected: "https://example.com/api/v0",
		},
		{
			name:     "custom path",
			input:    "https://example.com/v1",
			expected: "https://example.com/v1/api/v0",
		},
		{
			name:     "already includes api",
			input:    "https://example.com/api",
			expected: "https://example.com/api/api/v0",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := url.Parse(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, buildBaseURL(parsed))
		})
	}
	require.Equal(t, "", buildBaseURL(nil))
}
