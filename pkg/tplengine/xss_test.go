package tplengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEngine_XSSPrevention(t *testing.T) {
	t.Run("Should escape HTML in template output", func(t *testing.T) {
		// Arrange
		engine := NewEngine(FormatText)
		context := map[string]any{
			"userInput": `<script>alert('XSS')</script>`,
			"userName":  `John "Danger" O'Brien`,
		}

		// Test HTML escaping
		tmpl := `<div>{{ .userInput | htmlEscape }}</div>`

		// Act
		result, err := engine.RenderString(tmpl, context)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, `<div>&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;</div>`, result)
	})

	t.Run("Should escape HTML attributes", func(t *testing.T) {
		// Arrange
		engine := NewEngine(FormatText)
		context := map[string]any{
			"userInput": `" onclick="alert('XSS')`,
		}

		// Test attribute escaping
		tmpl := `<input value="{{ .userInput | htmlAttrEscape }}">`

		// Act
		result, err := engine.RenderString(tmpl, context)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, `<input value="&#34; onclick=&#34;alert(&#39;XSS&#39;)">`, result)
	})

	t.Run("Should escape JavaScript strings", func(t *testing.T) {
		// Arrange
		engine := NewEngine(FormatText)
		context := map[string]any{
			"userInput": `'; alert('XSS'); var dummy='`,
			"newlines":  "line1\nline2\rline3\tindented",
			"tags":      `<script>alert('XSS')</script>`,
		}

		// Test JavaScript escaping
		tmpl := `<script>
var userValue = '{{ .userInput | jsEscape }}';
var multiline = '{{ .newlines | jsEscape }}';
var htmlInJs = '{{ .tags | jsEscape }}';
</script>`

		// Act
		result, err := engine.RenderString(tmpl, context)

		// Assert
		require.NoError(t, err)
		// Check that jsEscape properly escapes JavaScript content using Unicode escaping
		assert.Contains(t, result, `\'; alert(\'XSS\'); var dummy\u003D\'`)
		assert.Contains(t, result, `line1\u000Aline2\u000Dline3\u0009indented`)
		assert.Contains(t, result, `\u003Cscript\u003Ealert(\'XSS\')\u003C/script\u003E`)
	})

	t.Run("Should handle nested contexts correctly", func(t *testing.T) {
		// Arrange
		engine := NewEngine(FormatText)
		context := map[string]any{
			"userInput": `<img src="x" onerror="alert('XSS')">`,
		}

		// Test combining escaping functions
		tmpl := `
<div title="{{ .userInput | htmlAttrEscape }}">
	{{ .userInput | htmlEscape }}
</div>
<script>
	var data = '{{ .userInput | jsEscape }}';
</script>`

		// Act
		result, err := engine.RenderString(tmpl, context)

		// Assert
		require.NoError(t, err)
		// Check for proper escaping in different contexts
		assert.Contains(t, result, `title="&lt;img src=&#34;x&#34; onerror=&#34;alert(&#39;XSS&#39;)&#34;&gt;"`)
		assert.Contains(t, result, `&lt;img src=&#34;x&#34; onerror=&#34;alert(&#39;XSS&#39;)&#34;&gt;`)
		assert.Contains(t, result, `\u003Cimg src\u003D\"x\" onerror\u003D\"alert(\'XSS\')\"\u003E`)
	})

	t.Run("Should prevent double escaping", func(t *testing.T) {
		// Arrange
		engine := NewEngine(FormatText)
		context := map[string]any{
			"alreadyEscaped": `&lt;script&gt;`,
		}

		// Test that we don't double-escape
		tmpl := `{{ .alreadyEscaped | htmlEscape }}`

		// Act
		result, err := engine.RenderString(tmpl, context)

		// Assert
		require.NoError(t, err)
		// Should escape the & in &lt; to &amp;lt;
		assert.Equal(t, `&amp;lt;script&amp;gt;`, result)
	})
}
