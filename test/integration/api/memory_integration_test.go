package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/auth/model"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/config"
	serverrouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	memrouter "github.com/compozy/compozy/engine/memory/router"
	"github.com/compozy/compozy/engine/memory/service"
	memuc "github.com/compozy/compozy/engine/memory/uc"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	memorytest "github.com/compozy/compozy/test/integration/memory"
)

// TestMemoryIntegrationComplete is the comprehensive test for memory integration
// between REST API and agent workflows. This replaces multiple fragmented tests.
func TestMemoryIntegrationComplete(t *testing.T) {
	// Setup memory test environment
	memoryEnv := memorytest.NewTestEnvironment(t)
	defer memoryEnv.Cleanup()

	// Register memory configuration matching examples/memory/memory/user_memory.yaml
	testMemoryConfig := &memory.Config{
		Resource:    "memory",
		ID:          "user_memory",
		Type:        memcore.TokenBasedMemory,
		Description: "User conversation history and personal information",
		MaxTokens:   2000,
		MaxMessages: 50,
		Persistence: memcore.PersistenceConfig{
			Type: memcore.RedisPersistence,
			TTL:  "168h",
		},
	}
	err := memoryEnv.RegisterMemoryConfig(testMemoryConfig)
	require.NoError(t, err)

	memoryManager := memoryEnv.GetMemoryManager()
	require.NotNil(t, memoryManager)

	// Test data
	testUserID := "api_test_user"
	explicitKey := "user:" + testUserID               // Key as stored by REST API
	templateKey := "user:{{.workflow.input.user_id}}" // Template from user_memory.yaml
	testMessage := "Hi, I am api_test_user and I love programming"

	t.Run("Should demonstrate and fix critical memory integration bug", func(t *testing.T) {
		ctx := context.Background()

		// Step 1: Store memory using direct use case (simulating REST API)
		t.Run("Store memory via REST API pattern", func(t *testing.T) {
			memService, err := service.NewMemoryOperationsService(memoryManager, nil, nil, nil, nil)
			require.NoError(t, err)
			messages := []map[string]any{
				{
					"role":    "user",
					"content": testMessage,
				},
			}

			// Write memory using explicit key (REST API approach)
			writeInput := &memuc.WriteMemoryInput{
				Messages: messages,
			}
			writeUC := memuc.NewWriteMemory(memService, "user_memory", explicitKey, writeInput)
			_, err = writeUC.Execute(ctx)
			require.NoError(t, err)
			t.Logf("✓ Memory successfully stored with explicit key: %s", explicitKey)
		})

		// Step 2: Verify we can read it back using the same explicit key
		t.Run("Read memory via REST API pattern", func(t *testing.T) {
			readInput := memuc.ReadMemoryInput{
				MemoryRef: "user_memory",
				Key:       explicitKey,
				Limit:     50,
				Offset:    0,
			}
			readUC := memuc.NewReadMemory(memoryManager, nil, nil)
			readOutput, err := readUC.Execute(ctx, readInput)
			require.NoError(t, err)
			require.Len(t, readOutput.Messages, 1)
			assert.Equal(t, testMessage, readOutput.Messages[0].Content)
			assert.Equal(t, llm.MessageRoleUser, readOutput.Messages[0].Role)
			t.Logf("✓ Memory successfully read back with explicit key")
		})

		// Step 3: Test agent memory access with template key resolution
		t.Run("Agent workflow template resolution", func(t *testing.T) {
			// Create workflow context exactly like the real memory-api.yaml workflow
			workflowContext := map[string]any{
				"workflow": map[string]any{
					"input": map[string]any{
						"user_id": testUserID,
						"message": "What do you know about me?",
					},
				},
			}

			// Resolve template key using template engine (agent approach)
			templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
			resolvedKeyStr, err := templateEngine.RenderString(templateKey, workflowContext)
			require.NoError(t, err)

			t.Logf("Template: %s", templateKey)
			t.Logf("Context: %+v", workflowContext)
			t.Logf("Resolved key: %s", resolvedKeyStr)
			t.Logf("Expected key: %s", explicitKey)

			// Keys should match
			assert.Equal(t, explicitKey, resolvedKeyStr, "Template resolution should match explicit key")

			// Try to read memory using resolved template key (agent approach)
			agentReadInput := memuc.ReadMemoryInput{
				MemoryRef: "user_memory",
				Key:       resolvedKeyStr,
				Limit:     50,
				Offset:    0,
			}
			agentReadUC := memuc.NewReadMemory(memoryManager, nil, nil)
			agentReadOutput, err := agentReadUC.Execute(ctx, agentReadInput)
			require.NoError(t, err)

			// This is the critical test - agent MUST be able to read the memory
			require.NotEmpty(
				t,
				agentReadOutput.Messages,
				"CRITICAL: Agent MUST be able to read memory stored via REST API",
			)
			assert.Len(t, agentReadOutput.Messages, 1)
			assert.Equal(t, testMessage, agentReadOutput.Messages[0].Content)
			assert.Equal(t, llm.MessageRoleUser, agentReadOutput.Messages[0].Role)
			t.Logf("✓ Agent successfully read memory using template key resolution")
		})

		// Step 4: Test through actual memory manager with workflow context
		t.Run("Memory manager with workflow context", func(t *testing.T) {
			// Create memory reference as workflows would
			memRef := core.MemoryReference{
				ID:  "user_memory",
				Key: templateKey, // Use template key, not resolved key
			}

			// Create workflow context matching the actual workflow
			workflowContext := map[string]any{
				"workflow": map[string]any{
					"input": map[string]any{
						"user_id": testUserID,
						"message": "What do you know about me?",
					},
				},
				"project": map[string]any{
					"id": "basic-memory", // Must match examples/memory/compozy.yaml
				},
			}

			// Get memory instance through the manager (simulating workflow access)
			instance, err := memoryManager.GetInstance(ctx, memRef, workflowContext)
			require.NoError(t, err)

			// Read messages through the instance
			messages, err := instance.Read(ctx)
			require.NoError(t, err)

			// Should have the same messages we stored via REST API
			require.Len(t, messages, 1)
			assert.Contains(t, messages[0].Content, testUserID)
			assert.Contains(t, messages[0].Content, "programming")
			t.Logf("✓ Memory manager successfully accessed memory with workflow context")
		})
	})
}

// TestMemoryRESTAPIWithRealWorkflow tests the complete end-to-end flow using actual
// HTTP server and workflow execution to ensure true integration
//
// To run this test locally:
//  1. Ensure PostgreSQL is running with the auth schema:
//     make start-docker
//     make migrate-up
//  2. Run the test with:
//     go test -v ./test/integration/api -run TestMemoryRESTAPIWithRealWorkflow
//
// Note: This test requires a full database setup and is skipped by default in CI.
// For CI environments, use TestMemoryRESTAPIWithAuthMiddleware which uses mocks.
func TestMemoryRESTAPIWithRealWorkflow(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST_DB") != "true" {
		t.Skip("Skipping test that requires full database setup. Set INTEGRATION_TEST_DB=true to run.")
	}
	// Setup test environment
	ctx := context.Background()
	log := logger.NewForTests()

	// Create memory test environment
	memoryEnv := memorytest.NewTestEnvironment(t)
	defer memoryEnv.Cleanup()

	// Get project root - use the examples directory
	projectRoot, err := filepath.Abs("../../../examples/memory")
	require.NoError(t, err)

	// Load project configuration
	configService := config.NewService(log, "")
	projectConfig, workflows, configRegistry, err := configService.LoadProject(ctx, projectRoot, "compozy.yaml")
	require.NoError(t, err)

	// Override cache config to use Mini Redis from memory test environment
	memoryRedis := memoryEnv.GetRedis()
	projectConfig.CacheConfig = &cache.Config{
		Host:     "localhost",
		Port:     strings.Split(memoryRedis.Options().Addr, ":")[1], // Extract port from Mini Redis
		Password: "",
		DB:       0,
	}

	// Create store (using test database config)
	storeConfig := &store.Config{
		Host:     "localhost",
		Port:     "5434", // Test DB port
		User:     "postgres",
		Password: "postgres",
		DBName:   "compozy_test",
		SSLMode:  "disable",
	}
	appStore, err := store.SetupStore(ctx, storeConfig)
	require.NoError(t, err)
	defer func() {
		if appStore.DB != nil {
			appStore.DB.Close(ctx)
		}
	}()

	// Create Temporal config for local testing
	temporalConfig := &worker.TemporalConfig{
		HostPort:  "localhost:7233",
		Namespace: "default",
		TaskQueue: "", // Will use generated task queue from project name
	}

	// Create base dependencies
	baseDeps := appstate.NewBaseDeps(projectConfig, workflows, appStore, temporalConfig)

	// Setup monitoring (disabled for testing)
	monitoringService, _ := monitoring.NewMonitoringService(ctx, projectConfig.MonitoringConfig)

	// Create worker
	workerInstance, err := setupWorker(ctx, baseDeps, monitoringService, configRegistry)
	require.NoError(t, err)

	// Create app state
	appState, err := appstate.NewState(baseDeps, workerInstance)
	require.NoError(t, err)

	// Create test HTTP server
	router := setupTestRouter(ctx, appState)
	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("Complete REST API and Workflow Integration", func(t *testing.T) {
		baseURL := server.URL + "/api/v0"
		userID := "workflow_test_user_" + fmt.Sprintf("%d", time.Now().Unix())
		memoryKey := "user:" + userID
		client := server.Client()

		// Create a test user and API key for authentication
		authRepo := appState.Store.NewAuthRepo()

		// Create test user
		testUserID := core.MustNewID()
		testUser := &model.User{
			ID:    testUserID,
			Email: "test@example.com",
			Role:  model.RoleUser,
		}
		err := authRepo.CreateUser(ctx, testUser)
		require.NoError(t, err)

		// Generate API key using the use case
		generateKeyUC := authuc.NewGenerateAPIKey(authRepo, testUserID)
		testAPIKey, err := generateKeyUC.Execute(ctx)
		require.NoError(t, err)

		// Step 1: Store memory via REST API
		t.Run("Store memory via REST API", func(t *testing.T) {
			messages := []llm.Message{
				{
					Role:    llm.MessageRoleUser,
					Content: "Hi, I am " + userID + " and I work as a software engineer. I love Go programming.",
				},
				{
					Role:    llm.MessageRoleAssistant,
					Content: "Nice to meet you, " + userID + ". I will remember that you are a software engineer who loves Go programming.",
				},
			}

			payload := map[string]any{
				"key":      memoryKey,
				"messages": messages,
			}
			body, err := json.Marshal(payload)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				baseURL+"/memory/user_memory/write",
				bytes.NewReader(body),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+testAPIKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			t.Logf("✓ Memory successfully written via REST API with key: %s", memoryKey)
		})

		// Step 2: Verify memory through workflow context access
		t.Run("Verify memory through workflow context", func(t *testing.T) {
			// Get memory manager from worker
			memManager := workerInstance.GetMemoryManager()
			require.NotNil(t, memManager)

			// Create memory reference with template key (not resolved)
			memRef := core.MemoryReference{
				ID:  "user_memory",
				Key: "user:{{.workflow.input.user_id}}", // Template from user_memory.yaml
			}

			// Create workflow context matching memory-api.yaml structure
			workflowContext := map[string]any{
				"workflow": map[string]any{
					"input": map[string]any{
						"user_id": userID,
						"message": "What do you remember about me?",
					},
				},
				"project": map[string]any{
					"id": projectConfig.Name, // Must match the actual project
				},
			}

			// Get memory instance through the manager
			instance, err := memManager.GetInstance(ctx, memRef, workflowContext)
			require.NoError(t, err)

			// Read messages through the instance
			messages, err := instance.Read(ctx)
			require.NoError(t, err)

			// Should have the same messages we stored via REST API
			require.Len(t, messages, 2)
			assert.Contains(t, messages[0].Content, userID)
			assert.Contains(t, messages[0].Content, "software engineer")
			assert.Contains(t, messages[1].Content, "Go programming")

			t.Logf("✓ Workflow context successfully accessed REST API stored memory")
		})

		// Step 3: Test memory stats consistency
		t.Run("Memory stats consistency", func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				baseURL+"/memory/user_memory/stats?key="+memoryKey,
				http.NoBody,
			)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+testAPIKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			var result struct {
				Data memuc.StatsMemoryOutput `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

			assert.Equal(t, memoryKey, result.Data.Key)
			assert.Equal(t, 2, result.Data.MessageCount)
			assert.Greater(t, result.Data.TokenCount, 0)
			t.Logf("✓ Memory stats show correct counts: %d messages, %d tokens",
				result.Data.MessageCount, result.Data.TokenCount)
		})
	})
}

// TestMemoryKeyTemplateVariations tests different memory key template patterns
func TestMemoryKeyTemplateVariations(t *testing.T) {
	memoryEnv := memorytest.NewTestEnvironment(t)
	defer memoryEnv.Cleanup()

	testMemoryConfig := &memory.Config{
		Resource:    "memory",
		ID:          "user_memory",
		Type:        memcore.TokenBasedMemory,
		Description: "User conversation history and personal information",
		MaxTokens:   2000,
		MaxMessages: 50,
		Persistence: memcore.PersistenceConfig{
			Type: memcore.RedisPersistence,
			TTL:  "168h",
		},
	}
	err := memoryEnv.RegisterMemoryConfig(testMemoryConfig)
	require.NoError(t, err)

	memoryManager := memoryEnv.GetMemoryManager()
	require.NotNil(t, memoryManager)

	t.Run("Template variations consistency", func(t *testing.T) {
		ctx := context.Background()
		memService, err := service.NewMemoryOperationsService(memoryManager, nil, nil, nil, nil)
		require.NoError(t, err)

		testCases := []struct {
			name        string
			userID      string
			template    string
			context     map[string]any
			expectedKey string
		}{
			{
				name:     "Standard workflow.input pattern",
				userID:   "user1",
				template: "user:{{.workflow.input.user_id}}",
				context: map[string]any{
					"workflow": map[string]any{
						"input": map[string]any{
							"user_id": "user1",
						},
					},
				},
				expectedKey: "user:user1",
			},
			{
				name:     "Direct input reference",
				userID:   "user2",
				template: "user:{{.user_id}}",
				context: map[string]any{
					"user_id": "user2",
				},
				expectedKey: "user:user2",
			},
			{
				name:     "Complex multi-field template",
				userID:   "user3",
				template: "{{.org_id}}:user:{{.workflow.input.user_id}}",
				context: map[string]any{
					"org_id": "org123",
					"workflow": map[string]any{
						"input": map[string]any{
							"user_id": "user3",
						},
					},
				},
				expectedKey: "org123:user:user3",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Store memory with explicit key
				messages := []map[string]any{
					{
						"role":    "user",
						"content": fmt.Sprintf("Hello from %s", tc.userID),
					},
				}

				writeInput := &memuc.WriteMemoryInput{
					Messages: messages,
				}
				writeUC := memuc.NewWriteMemory(memService, "user_memory", tc.expectedKey, writeInput)
				_, err := writeUC.Execute(ctx)
				require.NoError(t, err)

				// Resolve template key
				templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
				resolvedKey, err := templateEngine.RenderString(tc.template, tc.context)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedKey, resolvedKey)

				// Read memory using resolved key
				readInput := memuc.ReadMemoryInput{
					MemoryRef: "user_memory",
					Key:       resolvedKey,
					Limit:     50,
					Offset:    0,
				}
				readUC := memuc.NewReadMemory(memoryManager, nil, nil)
				readOutput, err := readUC.Execute(ctx, readInput)
				require.NoError(t, err)
				require.Len(t, readOutput.Messages, 1)
				assert.Equal(t, fmt.Sprintf("Hello from %s", tc.userID), readOutput.Messages[0].Content)
			})
		}
	})
}

// Helper functions from the original E2E test

func setupWorker(
	ctx context.Context,
	deps appstate.BaseDeps,
	monitoringService *monitoring.Service,
	configRegistry *autoload.ConfigRegistry,
) (*worker.Worker, error) {
	log := logger.FromContext(ctx)

	workerConfig := &worker.Config{
		WorkflowRepo: func() workflow.Repository {
			return deps.Store.NewWorkflowRepo()
		},
		TaskRepo: func() task.Repository {
			return deps.Store.NewTaskRepo()
		},
		MonitoringService: monitoringService,
		ResourceRegistry:  configRegistry,
	}

	workerInstance, err := worker.NewWorker(
		ctx,
		workerConfig,
		deps.ClientConfig,
		deps.ProjectConfig,
		deps.Workflows,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}

	if err := workerInstance.Setup(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup worker: %w", err)
	}

	log.Info("Worker setup completed for testing")
	return workerInstance, nil
}

func setupTestRouter(_ context.Context, state *appstate.State) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(appstate.StateMiddleware(state))
	router.Use(serverrouter.ErrorHandler())

	// Register memory routes with auth factory
	apiGroup := router.Group("/api/v0")
	authRepo := state.Store.NewAuthRepo()
	authFactory := authuc.NewFactory(authRepo)
	memrouter.Register(apiGroup, authFactory)

	return router
}
