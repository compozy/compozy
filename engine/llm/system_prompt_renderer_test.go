package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemPromptRenderer_WithInstructions(t *testing.T) {
	renderer := NewSystemPromptRenderer()
	instructions := "You are an expert agent orchestrator."

	out, err := renderer.Render(t.Context(), instructions)
	require.NoError(t, err)
	require.Contains(t, out, instructions)
	require.Contains(t, out, "<built-in-tools>")
	require.True(t, strings.HasSuffix(out, "\n"))
}

func TestSystemPromptRenderer_WithoutInstructions(t *testing.T) {
	renderer := NewSystemPromptRenderer()

	out, err := renderer.Render(t.Context(), "   ")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(out, "<built-in-tools>"))
	require.True(t, strings.HasSuffix(out, "\n"))
}
