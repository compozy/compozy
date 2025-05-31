package ref

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Test Setup
// -----------------------------------------------------------------------------

func setupRecursiveTest(t *testing.T) (string, string) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to get current file path")
	testDir := filepath.Dir(filename)
	fixturesDir := filepath.Join(testDir, "fixtures", "recursive")
	// Verify fixtures exist
	complexAppPath := filepath.Join(fixturesDir, "complex-app.yaml")
	_, err := os.Stat(complexAppPath)
	require.NoError(t, err, "recursive fixtures not found at %s", complexAppPath)
	return fixturesDir, complexAppPath
}

// -----------------------------------------------------------------------------
// Basic Recursive Resolution Tests
// -----------------------------------------------------------------------------

func TestRecursiveResolution_BasicFileChain(t *testing.T) {
	fixturesDir, _ := setupRecursiveTest(t)
	taskPath := filepath.Join(fixturesDir, "tasks", "task-1.yaml")

	t.Run("Should resolve nested file references with correct paths", func(t *testing.T) {
		doc := loadYAMLFile(t, taskPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		// Extract the config reference
		data, ok := docData.(map[string]any)
		require.True(t, ok)
		config, ok := data["config"].(map[string]any)
		require.True(t, ok)
		refStr, ok := config["$ref"].(string)
		require.True(t, ok)

		ref, err := parseStringRef(refStr)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := ref.Resolve(ctx, docData, taskPath, fixturesDir)
		require.NoError(t, err)

		// Check that all nested references were resolved
		resultMap, ok := result.(map[string]any)
		require.True(t, ok)

		// Check basic fields
		assert.Equal(t, "bar", resultMap["foo"])
		assert.Equal(t, "Nested task configuration", resultMap["description"])

		// Check that the agent reference was resolved
		agent, ok := resultMap["agent"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "agent", agent["id"])
		assert.Equal(t, "Test Agent", agent["name"])

		// Check that the tool reference within agent was resolved
		tool, ok := agent["tool"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "quote", tool["id"])
		assert.Equal(t, "Quote Tool", tool["name"])

		// Check that the tools array was resolved
		tools, ok := resultMap["tools"].([]any)
		require.True(t, ok)
		require.Len(t, tools, 1)

		firstTool, ok := tools[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "tool1", firstTool["name"])

		toolRef, ok := firstTool["ref"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "quote", toolRef["id"])
		assert.Equal(t, "Quote Tool", toolRef["name"])
	})
}

// -----------------------------------------------------------------------------
// Complex Application Resolution Tests
// -----------------------------------------------------------------------------

func TestRecursiveResolution_ComplexApplication(t *testing.T) {
	fixturesDir, complexAppPath := setupRecursiveTest(t)

	t.Run("Should resolve entire complex application structure", func(t *testing.T) {
		doc := loadYAMLFile(t, complexAppPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		// Create a WithRef instance to test map resolution
		resolver := &WithRef{}
		resolver.SetRefMetadata(complexAppPath, fixturesDir)

		ctx := context.Background()
		resolved, err := resolver.ResolveMapReference(ctx, docData.(map[string]any), docData)
		require.NoError(t, err)

		// Verify main service was resolved
		mainService, ok := resolved["main_service"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "api_service", mainService["id"])
		assert.Equal(t, "User API Service", mainService["name"])

		// Check auth schema was resolved (nested reference)
		auth, ok := mainService["auth"].(map[string]any)
		require.True(t, ok)
		schema, ok := auth["schema"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_schema", schema["id"])

		// Check role within user schema was resolved
		properties, ok := schema["properties"].(map[string]any)
		require.True(t, ok)
		role, ok := properties["role"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "role_schema", role["id"])

		// Check permissions array in role was resolved
		roleProps, ok := role["properties"].(map[string]any)
		require.True(t, ok)
		permissions, ok := roleProps["permissions"].(map[string]any)
		require.True(t, ok)
		items, ok := permissions["items"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "permission", items["id"])

		// Verify workflow with merge mode
		workflow, ok := resolved["primary_workflow"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "User Management Workflow", workflow["name"]) // Ref value wins
		assert.Equal(t, "user_workflow", workflow["id"])              // From reference

		// Check configurations array
		configs, ok := resolved["configurations"].([]any)
		require.True(t, ok)
		require.Len(t, configs, 3)

		// Check override config
		overrideConfig := configs[1].(map[string]any)
		config, ok := overrideConfig["config"].(map[string]any)
		require.True(t, ok)
		// This should have merged base auth config with overrides
		assert.Equal(t, "HS256", config["algorithm"]) // From base

		// Accept both int and float64 for numeric values (depends on YAML/JSON processing)
		timeout := config["timeout"]
		switch v := timeout.(type) {
		case int:
			assert.Equal(t, 300, v)
		case float64:
			assert.Equal(t, float64(300), v)
		default:
			t.Errorf("timeout should be int or float64, got %T", timeout)
		}

		retries := config["retries"]
		switch v := retries.(type) {
		case int:
			assert.Equal(t, 5, v)
		case float64:
			assert.Equal(t, float64(5), v)
		default:
			t.Errorf("retries should be int or float64, got %T", retries)
		}

		// Check handlers array
		handlers, ok := resolved["handlers"].([]any)
		require.True(t, ok)
		require.Len(t, handlers, 3)

		// First handler should be fully resolved
		firstHandler, ok := handlers[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list_users_handler", firstHandler["id"])

		// Check workflow in handler was resolved
		handlerWorkflow, ok := firstHandler["workflow"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list_workflow", handlerWorkflow["id"])
	})
}

// -----------------------------------------------------------------------------
// Path Resolution Edge Cases
// -----------------------------------------------------------------------------

func TestRecursiveResolution_PathStressTest(t *testing.T) {
	fixturesDir, _ := setupRecursiveTest(t)
	pathTestPath := filepath.Join(fixturesDir, "path-stress-test.yaml")

	t.Run("Should handle complex path resolution patterns", func(t *testing.T) {
		doc := loadYAMLFile(t, pathTestPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		resolver := &WithRef{}
		resolver.SetRefMetadata(pathTestPath, fixturesDir)

		ctx := context.Background()
		resolved, err := resolver.ResolveMapReference(ctx, docData.(map[string]any), docData)
		require.NoError(t, err)

		testCases, ok := resolved["test_cases"].(map[string]any)
		require.True(t, ok)

		// Test deep navigation
		deepNav, ok := testCases["deep_navigation"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_transform", deepNav["id"])

		// Test nested property access with file reference
		nestedProp, ok := testCases["nested_property"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_schema", nestedProp["id"])

		// Test array filter with file reference
		filteredArray, ok := testCases["filtered_array"].(map[string]any)
		require.True(t, ok)
		// This should resolve to the workflow config
		assert.Contains(t, filteredArray, "timeout")
		assert.Contains(t, filteredArray, "retries")

		// Test reference chain
		refChain, ok := testCases["reference_chain"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "agent", refChain["id"])
		assert.Equal(t, "Test Agent", refChain["name"])

		// Test merge modes
		modeTets, ok := testCases["mode_tests"].(map[string]any)
		require.True(t, ok)

		// Replace mode should ignore inline data
		replaceMode, ok := modeTets["replace_mode"].(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, replaceMode, "extra")
		assert.Equal(t, "base_llm", replaceMode["id"])

		// Merge mode should keep inline data
		mergeMode, ok := modeTets["merge_mode"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "should be kept", mergeMode["extra"])
		assert.Equal(t, 0.7, mergeMode["temperature"]) // Ref wins over inline
		assert.Equal(t, "base_llm", mergeMode["id"])   // From ref
	})
}

// -----------------------------------------------------------------------------
// Circular Reference Tests
// -----------------------------------------------------------------------------

func TestRecursiveResolution_CircularReferences(t *testing.T) {
	fixturesDir, _ := setupRecursiveTest(t)
	circularPath := filepath.Join(fixturesDir, "circular-test.yaml")

	t.Run("Should detect circular references", func(t *testing.T) {
		doc := loadYAMLFile(t, circularPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		data, ok := docData.(map[string]any)
		require.True(t, ok)

		// Valid chain should work
		validChain, ok := data["valid_chain"].(map[string]any)
		require.True(t, ok)
		refStr, ok := validChain["$ref"].(string)
		require.True(t, ok)

		ref, err := parseStringRef(refStr)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := ref.Resolve(ctx, docData, circularPath, fixturesDir)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Circular reference should be detected
		circularA, ok := data["circular_a"].(map[string]any)
		require.True(t, ok)
		circularRefStr, ok := circularA["$ref"].(string)
		require.True(t, ok)

		circularRef, err := parseStringRef(circularRefStr)
		require.NoError(t, err)

		_, err = circularRef.Resolve(ctx, docData, circularPath, fixturesDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular reference detected")
	})
}

// -----------------------------------------------------------------------------
// Property Reference Tests
// -----------------------------------------------------------------------------

func TestRecursiveResolution_PropertyReferences(t *testing.T) {
	fixturesDir, _ := setupRecursiveTest(t)
	propRefsPath := filepath.Join(fixturesDir, "property-refs.yaml")

	t.Run("Should resolve property references within same document", func(t *testing.T) {
		doc := loadYAMLFile(t, propRefsPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		resolver := &WithRef{}
		resolver.SetRefMetadata(propRefsPath, fixturesDir)

		ctx := context.Background()
		resolved, err := resolver.ResolveMapReference(ctx, docData.(map[string]any), docData)
		require.NoError(t, err)

		// Check schemas
		schemas, ok := resolved["schemas"].(map[string]any)
		require.True(t, ok)

		// Extended user should have merged properties
		extendedUser, ok := schemas["extended_user"].(map[string]any)
		require.True(t, ok)
		properties, ok := extendedUser["properties"].(map[string]any)
		require.True(t, ok)

		// Should have properties from base_user and extended_user
		assert.Contains(t, properties, "id")    // From base
		assert.Contains(t, properties, "name")  // From base
		assert.Contains(t, properties, "email") // From extended

		// Role should be resolved
		role, ok := properties["role"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Administrator", role["name"])

		// Check configs
		configs, ok := resolved["configs"].(map[string]any)
		require.True(t, ok)

		userConfig, ok := configs["user_config"].(map[string]any)
		require.True(t, ok)
		defaultRole, ok := userConfig["default_role"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "User", defaultRole["name"])

		// Check users array with merge mode
		users, ok := resolved["users"].([]any)
		require.True(t, ok)
		require.Len(t, users, 2)

		firstUser, ok := users[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user1", firstUser["id"])
		assert.Equal(t, "John Doe", firstUser["name"])
		assert.Equal(t, "john@example.com", firstUser["email"])
		// Should have type from schema
		assert.Equal(t, "object", firstUser["type"])
	})
}

// -----------------------------------------------------------------------------
// Global Reference Tests
// -----------------------------------------------------------------------------

func TestRecursiveResolution_GlobalReferences(t *testing.T) {
	fixturesDir, _ := setupRecursiveTest(t)
	globalRefsPath := filepath.Join(fixturesDir, "global-refs.yaml")

	// Create a mock compozy.yaml for global references
	compozyCopy := filepath.Join(fixturesDir, "compozy.yaml")
	originalCompozy := filepath.Join(fixturesDir, "..", "compozy.yaml")

	// Copy the original compozy.yaml to our test directory
	data, err := os.ReadFile(originalCompozy)
	require.NoError(t, err)
	err = os.WriteFile(compozyCopy, data, 0644)
	require.NoError(t, err)
	defer os.Remove(compozyCopy)

	t.Run("Should resolve global references", func(t *testing.T) {
		doc := loadYAMLFile(t, globalRefsPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		resolver := &WithRef{}
		resolver.SetRefMetadata(globalRefsPath, fixturesDir)

		ctx := context.Background()
		resolved, err := resolver.ResolveMapReference(ctx, docData.(map[string]any), docData)
		require.NoError(t, err)

		// Check providers
		providers, ok := resolved["providers"].(map[string]any)
		require.True(t, ok)

		mainProvider, ok := providers["main_provider"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "groq_llama", mainProvider["id"])
		assert.Equal(t, float64(0.7), mainProvider["temperature"]) // Ref value wins

		// Check schemas
		schemas, ok := resolved["schemas"].(map[string]any)
		require.True(t, ok)

		errorSchema, ok := schemas["error_schema"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "common_error", errorSchema["id"])

		// Check mixed references
		config, ok := resolved["configuration"].(map[string]any)
		require.True(t, ok)

		// Global reference
		provider, ok := config["provider"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "groq_llama", provider["id"])

		// Local reference
		localService, ok := config["local_service"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "api_service", localService["id"])
	})
}

// -----------------------------------------------------------------------------
// WithRef Struct Resolution Tests
// -----------------------------------------------------------------------------

type ComplexConfig struct {
	WithRef
	ServiceRef any              `json:"service_ref" yaml:"service_ref" is_ref:"true"`
	Workflows  []map[string]any `json:"workflows" yaml:"workflows"`
	Settings   map[string]any   `json:"settings" yaml:"settings"`
	Name       string           `json:"name" yaml:"name"`
}

func TestRecursiveResolution_WithRefStruct(t *testing.T) {
	fixturesDir, _ := setupRecursiveTest(t)

	t.Run("Should resolve references in struct fields", func(t *testing.T) {
		yamlContent := `
name: "Test Config"
service_ref: ./services/api/v1/service.yaml
workflows:
  - name: "workflow1"
    ref:
      $ref: ./workflows/nested/deep/user-workflow.yaml
settings:
  auth:
    $ref: ./services/auth/auth-provider.yaml
  schemas:
    user:
      $ref: ./shared/schemas/user.yaml
`
		var config ComplexConfig
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		configPath := filepath.Join(fixturesDir, "test-config.yaml")
		config.SetRefMetadata(configPath, fixturesDir)

		// Load a dummy current doc
		dummyDoc := map[string]any{"dummy": "data"}

		ctx := context.Background()
		err = config.ResolveReferences(ctx, &config, dummyDoc)
		require.NoError(t, err)

		// ServiceRef should be resolved
		service, ok := config.ServiceRef.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "api_service", service["id"])

		// Resolve nested references in workflows and settings
		resolvedWorkflows := make([]map[string]any, len(config.Workflows))
		for i, wf := range config.Workflows {
			resolved, err := config.ResolveMapReference(ctx, wf, dummyDoc)
			require.NoError(t, err)
			resolvedWorkflows[i] = resolved
		}

		// Check workflow was resolved
		assert.Len(t, resolvedWorkflows, 1)
		wfRef, ok := resolvedWorkflows[0]["ref"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_workflow", wfRef["id"])

		// Resolve settings
		resolvedSettings, err := config.ResolveMapReference(ctx, config.Settings, dummyDoc)
		require.NoError(t, err)

		auth, ok := resolvedSettings["auth"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "auth_provider", auth["id"])

		schemas, ok := resolvedSettings["schemas"].(map[string]any)
		require.True(t, ok)
		user, ok := schemas["user"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_schema", user["id"])
	})
}

// -----------------------------------------------------------------------------
// Performance and Stress Tests
// -----------------------------------------------------------------------------

func TestRecursiveResolution_DeepNesting(t *testing.T) {
	fixturesDir, complexAppPath := setupRecursiveTest(t)

	t.Run("Should handle deeply nested references efficiently", func(t *testing.T) {
		doc := loadYAMLFile(t, complexAppPath)
		docData, err := doc.Get("")
		require.NoError(t, err)

		// Focus on a deeply nested part
		data, ok := docData.(map[string]any)
		require.True(t, ok)
		handlers, ok := data["handlers"].([]any)
		require.True(t, ok)

		// Get the first handler reference
		firstHandler, ok := handlers[0].(map[string]any)
		require.True(t, ok, "First handler should be a direct $ref")
		refStr, ok := firstHandler["$ref"].(string)
		require.True(t, ok)

		ref, err := parseStringRef(refStr)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := ref.Resolve(ctx, docData, complexAppPath, fixturesDir)
		require.NoError(t, err)

		// This should resolve through multiple levels:
		// handler -> workflow -> task -> agent -> tool
		resultMap, ok := result.(map[string]any)
		require.True(t, ok)

		// Check the workflow was resolved
		workflow, ok := resultMap["workflow"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list_workflow", workflow["id"])

		// Check steps in workflow were resolved
		steps, ok := workflow["steps"].([]any)
		require.True(t, ok)
		require.Greater(t, len(steps), 0)

		// The list workflow has simple steps without deep task references
		firstStep, ok := steps[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "validate_params", firstStep["id"])
		assert.Equal(t, "validation", firstStep["type"])
	})
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// loadYAMLFile is already defined in mod_test.go, so we'll use that
