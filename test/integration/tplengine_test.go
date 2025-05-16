package test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/pkg/tplengine"
)

// normalizeWhitespace removes excess whitespace from a string to make tests more robust
// against whitespace differences in template output
func normalizeWhitespace(s string) string {
	// Trim the whole string first
	s = strings.TrimSpace(s)

	// Replace multiple consecutive newlines with a single newline
	s = regexp.MustCompile(`\n\s*\n+`).ReplaceAllString(s, "\n")

	// Replace multiple spaces with a single space on each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// Trim each line
		line = strings.TrimSpace(line)
		// Replace multiple spaces with a single space
		line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
		lines[i] = line
	}

	// Join lines back together
	return strings.Join(lines, "\n")
}

type TestCase struct {
	Name     string `yaml:"name"`
	Template string `yaml:"template"`
	Expected string `yaml:"expected"`
	Context  any    `yaml:"context,omitempty"`
}

type TestFixture struct {
	StringExamples      []TestCase `yaml:"string_examples,omitempty"`
	NumberExamples      []TestCase `yaml:"number_examples,omitempty"`
	DateExamples        []TestCase `yaml:"date_examples,omitempty"`
	CollectionExamples  []TestCase `yaml:"collection_examples,omitempty"`
	ConditionalExamples []TestCase `yaml:"conditional_examples,omitempty"`
	ComplexExamples     []TestCase `yaml:"complex_examples,omitempty"`
}

func loadTestFixture(t *testing.T, filename string) TestFixture {
	fixtureFile := filepath.Join("..", "fixtures", "tplengine", filename)
	data, err := os.ReadFile(fixtureFile)
	require.NoError(t, err, "Failed to read fixture file: %s", fixtureFile)

	var fixture TestFixture
	err = yaml.Unmarshal(data, &fixture)
	require.NoError(t, err, "Failed to unmarshal fixture data from: %s", fixtureFile)

	return fixture
}

func runTestCases(t *testing.T, testCases []TestCase) {
	engine := tplengine.NewEngine(tplengine.FormatYAML)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			var ctx map[string]any
			if tc.Context != nil {
				// Convert the context to a map
				ctxBytes, err := yaml.Marshal(tc.Context)
				require.NoError(t, err, "Failed to marshal context")

				ctx = make(map[string]any)
				err = yaml.Unmarshal(ctxBytes, &ctx)
				require.NoError(t, err, "Failed to unmarshal context")
			} else {
				ctx = make(map[string]any)
			}

			result, err := engine.RenderString(tc.Template, ctx)
			require.NoError(t, err, "Failed to render template")

			// Use normalized whitespace for comparison
			normalizedResult := normalizeWhitespace(result)
			normalizedExpected := normalizeWhitespace(tc.Expected)
			assert.Equal(t, normalizedExpected, normalizedResult)
		})
	}
}

func runDateTestCases(t *testing.T, testCases []TestCase) {
	// Create a custom template engine for date tests with a fixed date
	fixedTime, _ := time.Parse(time.RFC3339, "2023-01-02T15:04:05Z")

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			var ctx map[string]any
			if tc.Context != nil {
				// Convert the context to a map
				ctxBytes, err := yaml.Marshal(tc.Context)
				require.NoError(t, err, "Failed to marshal context")

				ctx = make(map[string]any)
				err = yaml.Unmarshal(ctxBytes, &ctx)
				require.NoError(t, err, "Failed to unmarshal context")
			} else {
				ctx = make(map[string]any)
			}

			// Create a custom function map with a fixed now function
			funcMap := sprig.TxtFuncMap()
			funcMap["now"] = func() time.Time {
				return fixedTime
			}

			// Parse and execute the template directly
			tmpl, err := template.New("test").Funcs(funcMap).Parse(tc.Template)
			require.NoError(t, err, "Failed to parse template")

			var buf strings.Builder
			err = tmpl.Execute(&buf, ctx)
			require.NoError(t, err, "Failed to execute template")

			result := buf.String()

			// Use normalized whitespace for comparison
			normalizedResult := normalizeWhitespace(result)
			normalizedExpected := normalizeWhitespace(tc.Expected)
			assert.Equal(t, normalizedExpected, normalizedResult)
		})
	}
}

func TestStringHelpers(t *testing.T) {
	fixture := loadTestFixture(t, "tplengine_string.yaml")
	runTestCases(t, fixture.StringExamples)
}

func TestNumberHelpers(t *testing.T) {
	fixture := loadTestFixture(t, "tplengine_number.yaml")
	runTestCases(t, fixture.NumberExamples)
}

func TestDateHelpers(t *testing.T) {
	fixture := loadTestFixture(t, "tplengine_date.yaml")
	runDateTestCases(t, fixture.DateExamples)
}

func TestCollectionHelpers(t *testing.T) {
	fixture := loadTestFixture(t, "tplengine_collection.yaml")
	runTestCases(t, fixture.CollectionExamples)
}

func TestConditionals(t *testing.T) {
	fixture := loadTestFixture(t, "tplengine_conditional.yaml")
	runTestCases(t, fixture.ConditionalExamples)
}

func TestComplexTemplates(t *testing.T) {
	fixture := loadTestFixture(t, "tplengine_complex.yaml")
	runTestCases(t, fixture.ComplexExamples)
}

func TestEngineBasics(t *testing.T) {
	t.Run("NewEngine", func(t *testing.T) {
		engine := tplengine.NewEngine(tplengine.FormatYAML)
		assert.NotNil(t, engine)
	})

	t.Run("WithFormat", func(t *testing.T) {
		engine := tplengine.NewEngine(tplengine.FormatYAML)
		// Test that the engine works with the default format
		result, err := engine.RenderString("{{ upper \"hello\" }}", nil)
		assert.NoError(t, err)
		assert.Equal(t, "HELLO", result)

		// Change format and verify it still works
		engine = engine.WithFormat(tplengine.FormatJSON)
		result, err = engine.RenderString("{{ upper \"hello\" }}", nil)
		assert.NoError(t, err)
		assert.Equal(t, "HELLO", result)
	})

	t.Run("HasTemplate", func(t *testing.T) {
		assert.True(t, tplengine.HasTemplate("Hello, {{ .name }}!"))
		assert.False(t, tplengine.HasTemplate("Hello, World!"))
	})

	t.Run("AddTemplate", func(t *testing.T) {
		engine := tplengine.NewEngine(tplengine.FormatYAML)
		err := engine.AddTemplate("greeting", "Hello, {{ .name }}!")
		assert.NoError(t, err)

		result, err := engine.Render("greeting", map[string]any{"name": "World"})
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!", result)

		_, err = engine.Render("non-existent", nil)
		assert.Error(t, err)
	})
}
