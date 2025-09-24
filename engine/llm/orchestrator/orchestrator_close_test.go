package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
)

type closableRegistry struct{ closeErr error }

func (c closableRegistry) Find(_ context.Context, _ string) (RegistryTool, bool) { return nil, false }
func (c closableRegistry) ListAll(_ context.Context) ([]RegistryTool, error)     { return nil, nil }
func (c closableRegistry) Close() error                                          { return c.closeErr }

type promptNoop struct{}

func (promptNoop) Build(_ context.Context, act *agent.ActionConfig) (string, error) {
	return act.Prompt, nil
}
func (promptNoop) EnhanceForStructuredOutput(_ context.Context, p string, _ *schema.Schema, _ bool) string {
	return p
}
func (promptNoop) ShouldUseStructuredOutput(_ string, _ *agent.ActionConfig, _ []tool.Config) bool {
	return false
}

func TestOrchestrator_Close_ErrorPropagation(t *testing.T) {
	orc, err := New(Config{ToolRegistry: closableRegistry{closeErr: errors.New("bye")}, PromptBuilder: promptNoop{}})
	assert.NoError(t, err)
	assert.Error(t, orc.Close())
}

func TestOrchestrator_Close_NoRegistry(t *testing.T) {
	orc, err := New(Config{ToolRegistry: closableRegistry{closeErr: nil}, PromptBuilder: promptNoop{}})
	assert.NoError(t, err)
	assert.NoError(t, orc.Close())
}
