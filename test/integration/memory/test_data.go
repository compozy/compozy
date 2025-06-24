package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// TestDataManager manages test data for integration tests
type TestDataManager struct {
	mu            sync.Mutex
	testInstances map[string][]string // test name -> instance IDs
	env           *TestEnvironment
}

// NewTestDataManager creates a new test data manager
func NewTestDataManager(env *TestEnvironment) *TestDataManager {
	return &TestDataManager{
		testInstances: make(map[string][]string),
		env:           env,
	}
}

// RegisterInstance registers a memory instance for cleanup
func (m *TestDataManager) RegisterInstance(testName string, instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.testInstances[testName] = append(m.testInstances[testName], instanceID)
}

// CleanupTest cleans up all data for a specific test
func (m *TestDataManager) CleanupTest(t *testing.T, testName string) {
	m.mu.Lock()
	instances := m.testInstances[testName]
	delete(m.testInstances, testName)
	m.mu.Unlock()
	if len(instances) == 0 {
		return
	}
	ctx := context.Background()
	redis := m.env.GetRedis()
	for _, instanceID := range instances {
		// Clean up memory data
		key := fmt.Sprintf("compozy:test-project:memory:%s", instanceID)
		metaKey := fmt.Sprintf("%s:metadata", key)
		pendingKey := fmt.Sprintf("__compozy_internal__:flush_pending:%s", key)
		// Delete all related keys
		keys := []string{key, metaKey, pendingKey}
		err := redis.Del(ctx, keys...).Err()
		if err != nil {
			t.Logf("Failed to cleanup instance %s: %v", instanceID, err)
		}
	}
	t.Logf("Cleaned up %d instances for test %s", len(instances), testName)
}

// CleanupAll cleans up all test data
func (m *TestDataManager) CleanupAll(t *testing.T) {
	m.mu.Lock()
	allTests := make([]string, 0, len(m.testInstances))
	for testName := range m.testInstances {
		allTests = append(allTests, testName)
	}
	m.mu.Unlock()
	for _, testName := range allTests {
		m.CleanupTest(t, testName)
	}
}

// TestDataSet represents a set of test data
type TestDataSet struct {
	Name        string
	Messages    []llm.Message
	MemoryRef   core.MemoryReference
	WorkflowCtx map[string]any
}

// StandardTestDataSets returns commonly used test data sets
func StandardTestDataSets() []TestDataSet {
	timestamp := time.Now().Unix()
	return []TestDataSet{
		{
			Name: "Basic Conversation",
			Messages: []llm.Message{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Hello!"},
				{Role: "assistant", Content: "Hi there! How can I help you today?"},
			},
			MemoryRef: core.MemoryReference{
				ID:  "customer-support",
				Key: "basic-conv-{{.test.id}}",
			},
			WorkflowCtx: map[string]any{
				"project.id": "test-project",
				"test.id":    fmt.Sprintf("basic-%d", timestamp),
			},
		},
		{
			Name:     "Long Conversation",
			Messages: generateLongConversation(50),
			MemoryRef: core.MemoryReference{
				ID:  "shared-memory",
				Key: "long-conv-{{.test.id}}",
			},
			WorkflowCtx: map[string]any{
				"project.id": "test-project",
				"test.id":    fmt.Sprintf("long-%d", timestamp),
			},
		},
		{
			Name: "Multilingual Content",
			Messages: []llm.Message{
				{Role: "user", Content: "Hello! 你好! مرحبا! Bonjour!"},
				{Role: "assistant", Content: "I can help in multiple languages!"},
				{Role: "user", Content: "日本語も話せますか？"},
				{Role: "assistant", Content: "Yes, I can communicate in Japanese too!"},
			},
			MemoryRef: core.MemoryReference{
				ID:  "customer-support",
				Key: "multilingual-{{.test.id}}",
			},
			WorkflowCtx: map[string]any{
				"project.id": "test-project",
				"test.id":    fmt.Sprintf("multi-%d", timestamp),
			},
		},
		{
			Name: "Technical Support",
			Messages: []llm.Message{
				{Role: "system", Content: "You are a technical support specialist."},
				{Role: "user", Content: "My application is showing error code 500"},
				{Role: "assistant", Content: "Error 500 indicates a server error. Let me help you troubleshoot."},
				{Role: "user", Content: "I see 'database connection failed' in the logs"},
				{
					Role:    "assistant",
					Content: "That suggests the database server might be down or unreachable. Let's check the connection settings.",
				},
			},
			MemoryRef: core.MemoryReference{
				ID:  "customer-support",
				Key: "tech-support-{{.test.id}}",
			},
			WorkflowCtx: map[string]any{
				"project.id": "test-project",
				"test.id":    fmt.Sprintf("tech-%d", timestamp),
			},
		},
	}
}

// generateLongConversation generates a long conversation for testing
func generateLongConversation(messageCount int) []llm.Message {
	messages := make([]llm.Message, 0, messageCount)
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: "You are a helpful assistant engaged in a long conversation.",
	})
	for i := 1; i < messageCount; i++ {
		if i%2 == 1 {
			messages = append(messages, llm.Message{
				Role: "user",
				Content: fmt.Sprintf(
					"This is user message %d. Tell me something interesting about the number %d.",
					i/2+1,
					i/2+1,
				),
			})
		} else {
			messages = append(messages, llm.Message{
				Role: "assistant",
				Content: fmt.Sprintf("Here's an interesting fact about %d: it has %d divisors "+
					"and its binary representation is %b.", i/2, countDivisors(i/2), i/2),
			})
		}
	}
	return messages
}

// countDivisors counts the number of divisors of n
func countDivisors(n int) int {
	count := 0
	for i := 1; i <= n; i++ {
		if n%i == 0 {
			count++
		}
	}
	return count
}

// TestDataHelper provides helper functions for test data
type TestDataHelper struct {
	manager *TestDataManager
}

// NewTestDataHelper creates a new test data helper
func NewTestDataHelper(env *TestEnvironment) *TestDataHelper {
	return &TestDataHelper{
		manager: NewTestDataManager(env),
	}
}

// PopulateMemory populates a memory instance with test data
func (h *TestDataHelper) PopulateMemory(
	ctx context.Context,
	t *testing.T,
	instance memcore.Memory,
	messages []llm.Message,
) error {
	t.Helper()
	for _, msg := range messages {
		if err := instance.Append(ctx, msg); err != nil {
			return fmt.Errorf("failed to append message: %w", err)
		}
	}
	// Register for cleanup
	h.manager.RegisterInstance(t.Name(), instance.GetID())
	return nil
}

// CreateAndPopulateMemory creates a memory instance and populates it with data
func (h *TestDataHelper) CreateAndPopulateMemory(
	t *testing.T,
	env *TestEnvironment,
	dataSet *TestDataSet,
) (memcore.Memory, error) {
	t.Helper()
	ctx := context.Background()
	// Create instance
	instance, err := env.GetMemoryManager().GetInstance(ctx, dataSet.MemoryRef, dataSet.WorkflowCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}
	// Populate with data
	if err := h.PopulateMemory(ctx, t, instance, dataSet.Messages); err != nil {
		return nil, err
	}
	return instance, nil
}

// Cleanup cleans up test data
func (h *TestDataHelper) Cleanup(t *testing.T) {
	h.manager.CleanupTest(t, t.Name())
}

// CleanupAll cleans up all test data
func (h *TestDataHelper) CleanupAll(t *testing.T) {
	h.manager.CleanupAll(t)
}

// VerifyMemoryContent verifies memory content matches expected
func VerifyMemoryContent(
	ctx context.Context,
	t *testing.T,
	instance memcore.Memory,
	expectedMessages []llm.Message,
) {
	t.Helper()
	messages, err := instance.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read messages: %v", err)
	}
	if len(messages) != len(expectedMessages) {
		t.Fatalf("Message count mismatch: got %d, want %d", len(messages), len(expectedMessages))
	}
	for i, msg := range messages {
		if msg.Role != expectedMessages[i].Role {
			t.Errorf("Message %d role mismatch: got %s, want %s", i, msg.Role, expectedMessages[i].Role)
		}
		if msg.Content != expectedMessages[i].Content {
			t.Errorf("Message %d content mismatch: got %s, want %s", i, msg.Content, expectedMessages[i].Content)
		}
	}
}

// GenerateTestMessages generates test messages with specific characteristics
func GenerateTestMessages(count int, avgTokensPerMessage int) []llm.Message {
	messages := make([]llm.Message, count)
	// Approximate tokens with words (roughly 1.3 tokens per word)
	wordsPerMessage := avgTokensPerMessage * 3 / 4
	for i := 0; i < count; i++ {
		role := "user"
		if i%3 == 2 {
			role = "assistant"
		} else if i%10 == 0 {
			role = "system"
		}
		content := generateContent(wordsPerMessage, i)
		messages[i] = llm.Message{
			Role:    llm.MessageRole(role),
			Content: content,
		}
	}
	return messages
}

// generateContent generates content with approximately the specified word count
func generateContent(wordCount int, seed int) string {
	words := []string{
		"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog",
		"and", "runs", "through", "forest", "with", "great", "speed", "today",
		"while", "thinking", "about", "complex", "problems", "in", "computer", "science",
	}
	result := ""
	for i := 0; i < wordCount; i++ {
		if i > 0 {
			result += " "
		}
		result += words[(i+seed)%len(words)]
	}
	return result
}
