package llmadapter

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFactory_CreateClient(t *testing.T) {
	registry := NewProviderRegistry()
	factory := NewDefaultFactoryWithRegistry(registry)

	t.Run("Should return error when config is nil", func(t *testing.T) {
		client, err := factory.CreateClient(context.Background(), nil)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "provider config must not be nil")
	})

	t.Run("Should return error for unregistered provider", func(t *testing.T) {
		config := &core.ProviderConfig{Provider: core.ProviderName("unsupported")}
		client, err := factory.CreateClient(context.Background(), config)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "provider unsupported is not registered")
	})

	t.Run("Should create client for registered provider", func(t *testing.T) {
		reg := NewProviderRegistry()
		client := &stubClient{}
		require.NoError(t, reg.Register(&stubProvider{name: core.ProviderName("stub"), client: client}))
		fac := NewDefaultFactoryWithRegistry(reg)
		cfg := &core.ProviderConfig{Provider: core.ProviderName("stub")}

		got, err := fac.CreateClient(context.Background(), cfg)
		require.NoError(t, err)
		assert.Equal(t, client, got)
	})
}

func TestDefaultFactory_BuildRouteFallback(t *testing.T) {
	reg := NewProviderRegistry()
	primaryErr := errors.New("primary failure")
	require.NoError(t, reg.Register(&stubProvider{name: core.ProviderName("primary"), err: primaryErr}))
	secondaryClient := &stubClient{}
	require.NoError(t, reg.Register(&stubProvider{name: core.ProviderName("secondary"), client: secondaryClient}))
	factory := NewDefaultFactoryWithRegistry(reg)

	route, err := factory.BuildRoute(
		&core.ProviderConfig{Provider: core.ProviderName("primary")},
		&core.ProviderConfig{Provider: core.ProviderName("secondary")},
	)
	require.NoError(t, err)

	client, err := route.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, secondaryClient, client)
}

func TestDefaultFactory_Capabilities(t *testing.T) {
	reg := NewProviderRegistry()
	caps := ProviderCapabilities{StructuredOutput: true, Streaming: true}
	require.NoError(
		t,
		reg.Register(&stubProvider{name: core.ProviderName("cap"), capabilities: caps, client: &stubClient{}}),
	)

	factory := NewDefaultFactoryWithRegistry(reg)
	actual, err := factory.Capabilities(core.ProviderName("cap"))
	require.NoError(t, err)
	assert.Equal(t, caps, actual)
}

type stubClient struct{}

func (stubClient) GenerateContent(context.Context, *LLMRequest) (*LLMResponse, error) {
	return &LLMResponse{Content: "ok"}, nil
}

func (stubClient) Close() error { return nil }

type stubProvider struct {
	name         core.ProviderName
	client       LLMClient
	err          error
	capabilities ProviderCapabilities
}

func (p *stubProvider) Name() core.ProviderName { return p.name }

func (p *stubProvider) Capabilities() ProviderCapabilities {
	return p.capabilities
}

func (p *stubProvider) NewClient(context.Context, *core.ProviderConfig) (LLMClient, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.client, nil
}
