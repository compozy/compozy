package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----
// Test Helper Functions
// -----

// waitForSubscriptionReady waits for a subscription to be ready using a ping/pong mechanism
func waitForSubscriptionReady(
	ctx context.Context,
	t testing.TB,
	publisher interface {
		Publish(ctx context.Context, channel string, message any) error
	},
	channel string,
	msgChan <-chan Message,
) {
	// Create a timeout context for the wait operation
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Create a unique ping message to avoid interference with other tests
	pingMsg := map[string]string{"test_ping": time.Now().Format("20060102150405.000000")}

	// Retry mechanism with exponential backoff
	for attempt := range 10 {
		select {
		case <-waitCtx.Done():
			t.Fatalf("Subscription did not become ready within timeout: %v", waitCtx.Err())
		default:
			// Try to publish the ping message
			if err := publisher.Publish(waitCtx, channel, pingMsg); err != nil {
				// If publish fails, wait a bit and retry
				time.Sleep(time.Millisecond * time.Duration(1<<attempt)) // exponential backoff
				continue
			}

			// Try to receive the ping message with a short timeout
			select {
			case <-msgChan:
				// Successfully received a message, subscription is ready
				return
			case <-time.After(100 * time.Millisecond): // Default poll interval
				// No message received yet, retry
				continue
			}
		}
	}

	t.Fatal("Subscription readiness could not be confirmed after 10 attempts")
}

func TestRedisNotificationSystem_Publish(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	channel := "test-channel"

	t.Run("Should successfully publish message", func(t *testing.T) {
		message := map[string]any{
			"type": "test",
			"data": "hello world",
		}

		err := ns.Publish(ctx, channel, message)
		assert.NoError(t, err)

		metrics := ns.GetMetrics()
		assert.Greater(t, metrics.MessagesPublished, int64(0))
	})

	t.Run("Should handle invalid message serialization", func(t *testing.T) {
		// Function types cannot be serialized to JSON
		invalidMessage := func() {}

		err := ns.Publish(ctx, channel, invalidMessage)
		assert.ErrorContains(t, err, "json")

		metrics := ns.GetMetrics()
		assert.Greater(t, metrics.PublishErrors, int64(0))
	})
}

func TestRedisNotificationSystem_Subscribe(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	channel := "test-channel"

	t.Run("Should successfully subscribe and receive messages", func(t *testing.T) {
		// Subscribe to channel
		msgChan, err := ns.Subscribe(ctx, channel)
		require.NoError(t, err)

		// Wait for subscription to be ready
		waitForSubscriptionReady(ctx, t, ns, channel, msgChan)

		// Publish a test message
		testMessage := map[string]string{"test": "message"}
		err = ns.Publish(ctx, channel, testMessage)
		require.NoError(t, err)

		// Receive the message
		select {
		case msg := <-msgChan:
			assert.Equal(t, channel, msg.Channel)

			// Deserialize and verify content
			var received map[string]string
			err := json.Unmarshal(msg.Payload, &received)
			require.NoError(t, err)
			assert.Equal(t, testMessage, received)
		case <-time.After(time.Second):
			t.Fatal("Did not receive message within timeout")
		}

		metrics := ns.GetMetrics()
		assert.Greater(t, metrics.MessagesReceived, int64(0))
	})

	t.Run("Should fail with empty channel list", func(t *testing.T) {
		_, err := ns.Subscribe(ctx)
		assert.ErrorContains(t, err, "channel")
	})
}

func TestRedisNotificationSystem_SubscribePattern(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	pattern := "test:*"

	t.Run("Should subscribe to pattern and receive matching messages", func(t *testing.T) {
		// Subscribe to pattern
		msgChan, err := ns.SubscribePattern(ctx, pattern)
		require.NoError(t, err)

		// Wait for subscription to be ready using a specific channel that matches the pattern
		waitForSubscriptionReady(ctx, t, ns, "test:readiness", msgChan)

		// Publish messages to matching channels
		channels := []string{"test:channel1", "test:channel2", "other:channel"}
		testMessage := map[string]string{"test": "pattern message"}

		for _, ch := range channels {
			err = ns.Publish(ctx, ch, testMessage)
			require.NoError(t, err)
		}

		// Should receive messages from test:* channels only
		receivedChannels := make(map[string]bool)
		timeout := time.After(time.Second)

		for range 2 { // Expect 2 messages from matching channels
			select {
			case msg := <-msgChan:
				receivedChannels[msg.Channel] = true

				var received map[string]string
				err := json.Unmarshal(msg.Payload, &received)
				require.NoError(t, err)
				assert.Equal(t, testMessage, received)
			case <-timeout:
				t.Fatal("Did not receive expected messages within timeout")
			}
		}

		// Verify we got messages from the right channels
		assert.True(t, receivedChannels["test:channel1"])
		assert.True(t, receivedChannels["test:channel2"])
		assert.False(t, receivedChannels["other:channel"])
	})

	t.Run("Should fail with empty pattern list", func(t *testing.T) {
		_, err := ns.SubscribePattern(ctx)
		assert.ErrorContains(t, err, "pattern")
	})
}

func TestRedisNotificationSystem_WorkflowEvents(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	workflowID := "workflow-123"

	t.Run("Should publish and receive workflow events", func(t *testing.T) {
		// Subscribe to workflow events
		msgChan, err := ns.SubscribeToWorkflow(ctx, workflowID)
		require.NoError(t, err)

		// Wait for subscription to be ready
		waitForSubscriptionReady(ctx, t, ns, fmt.Sprintf("workflow:%s", workflowID), msgChan)

		// Publish a workflow event
		data := map[string]any{"step": "validation"}
		err = ns.PublishWorkflowEvent(ctx, workflowID, "started", "running", data)
		require.NoError(t, err)

		// Receive the event
		select {
		case msg := <-msgChan:
			var event WorkflowEvent
			err := json.Unmarshal(msg.Payload, &event)
			require.NoError(t, err)

			assert.Equal(t, workflowID, event.WorkflowID)
			assert.Equal(t, "started", event.Event)
			assert.Equal(t, "running", event.Status)
			assert.Equal(t, data, event.Data)
		case <-time.After(time.Second):
			t.Fatal("Did not receive workflow event within timeout")
		}
	})
}

func TestRedisNotificationSystem_TaskEvents(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	taskID := "task-456"
	workflowID := "workflow-123"

	t.Run("Should publish and receive task events on multiple channels", func(t *testing.T) {
		// Subscribe to both task and workflow channels
		taskMsgChan, err := ns.SubscribeToTask(ctx, taskID)
		require.NoError(t, err)

		workflowMsgChan, err := ns.SubscribeToWorkflow(ctx, workflowID)
		require.NoError(t, err)

		// Wait for subscriptions to be ready
		waitForSubscriptionReady(ctx, t, ns, fmt.Sprintf("task:%s", taskID), taskMsgChan)
		waitForSubscriptionReady(ctx, t, ns, fmt.Sprintf("workflow:%s", workflowID), workflowMsgChan)

		// Publish a task event
		data := map[string]any{"result": "success"}
		err = ns.PublishTaskEvent(ctx, taskID, workflowID, "completed", "done", data)
		require.NoError(t, err)

		// Should receive event on task channel
		select {
		case msg := <-taskMsgChan:
			var event TaskEvent
			err := json.Unmarshal(msg.Payload, &event)
			require.NoError(t, err)

			assert.Equal(t, taskID, event.TaskID)
			assert.Equal(t, workflowID, event.WorkflowID)
			assert.Equal(t, "completed", event.Event)
			assert.Equal(t, "done", event.Status)
		case <-time.After(time.Second):
			t.Fatal("Did not receive task event on task channel")
		}

		// Should also receive event on workflow channel
		select {
		case msg := <-workflowMsgChan:
			var event TaskEvent
			err := json.Unmarshal(msg.Payload, &event)
			require.NoError(t, err)

			assert.Equal(t, taskID, event.TaskID)
			assert.Equal(t, workflowID, event.WorkflowID)
		case <-time.After(time.Second):
			t.Fatal("Did not receive task event on workflow channel")
		}
	})
}

func TestRedisNotificationSystem_PatternSubscriptions(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()

	t.Run("Should receive all workflow events via pattern subscription", func(t *testing.T) {
		// Subscribe to all workflows
		msgChan, err := ns.SubscribeToAllWorkflows(ctx)
		require.NoError(t, err)

		// Wait for subscription to be ready using a channel that matches the workflow:* pattern
		waitForSubscriptionReady(ctx, t, ns, "workflow:readiness", msgChan)

		// Publish events for multiple workflows
		workflows := []string{"workflow-1", "workflow-2", "workflow-3"}
		for _, wfID := range workflows {
			err = ns.PublishWorkflowEvent(ctx, wfID, "started", "running", nil)
			require.NoError(t, err)
		}

		// Should receive all events
		receivedWorkflows := make(map[string]bool)
		timeout := time.After(time.Second)

		for range len(workflows) {
			select {
			case msg := <-msgChan:
				var event WorkflowEvent
				err := json.Unmarshal(msg.Payload, &event)
				require.NoError(t, err)

				receivedWorkflows[event.WorkflowID] = true
			case <-timeout:
				t.Fatal("Did not receive all workflow events within timeout")
			}
		}

		// Verify we received events from all workflows
		for _, wfID := range workflows {
			assert.True(t, receivedWorkflows[wfID], "Did not receive event for workflow %s", wfID)
		}
	})

	t.Run("Should receive all task events via pattern subscription", func(t *testing.T) {
		// Subscribe to all tasks
		msgChan, err := ns.SubscribeToAllTasks(ctx)
		require.NoError(t, err)

		// Wait for subscription to be ready using a channel that matches the task:* pattern
		waitForSubscriptionReady(ctx, t, ns, "task:readiness", msgChan)

		// Publish events for multiple tasks
		tasks := []string{"task-1", "task-2", "task-3"}
		for _, taskID := range tasks {
			err = ns.PublishTaskEvent(ctx, taskID, "", "completed", "done", nil)
			require.NoError(t, err)
		}

		// Should receive all events
		receivedTasks := make(map[string]bool)
		timeout := time.After(time.Second)

		for range len(tasks) {
			select {
			case msg := <-msgChan:
				var event TaskEvent
				err := json.Unmarshal(msg.Payload, &event)
				require.NoError(t, err)

				receivedTasks[event.TaskID] = true
			case <-timeout:
				t.Fatal("Did not receive all task events within timeout")
			}
		}

		// Verify we received events from all tasks
		for _, taskID := range tasks {
			assert.True(t, receivedTasks[taskID], "Did not receive event for task %s", taskID)
		}
	})
}

func TestRedisNotificationSystem_ConcurrentOperations(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	channel := "concurrent-test"

	t.Run("Should handle concurrent publishers and subscribers", func(t *testing.T) {
		const numPublishers = 5
		const numSubscribers = 3
		const messagesPerPublisher = 10

		// Start subscribers
		var subscriberWg sync.WaitGroup
		var receivedCount int64
		var receivedMu sync.Mutex

		for i := range numSubscribers {
			subscriberWg.Add(1)
			go func(_ int) {
				defer subscriberWg.Done()

				msgChan, err := ns.Subscribe(ctx, channel)
				require.NoError(t, err)

				// Count received messages
				for msg := range msgChan {
					receivedMu.Lock()
					receivedCount++
					receivedMu.Unlock()

					// Verify message structure
					var payload map[string]any
					err := json.Unmarshal(msg.Payload, &payload)
					assert.NoError(t, err)
				}
			}(i)
		}

		// Give subscriptions time to establish
		time.Sleep(50 * time.Millisecond)

		// Start publishers
		var publisherWg sync.WaitGroup
		for i := range numPublishers {
			publisherWg.Add(1)
			go func(pubID int) {
				defer publisherWg.Done()

				for j := range messagesPerPublisher {
					message := map[string]any{
						"publisher": pubID,
						"message":   j,
					}
					err := ns.Publish(ctx, channel, message)
					assert.NoError(t, err)
				}
			}(i)
		}

		// Wait for all publishers to finish
		publisherWg.Wait()

		// Wait for all messages to be received with timeout
		expectedMessages := int64(numPublishers * messagesPerPublisher * numSubscribers)
		timeout := time.After(2 * time.Second)

	waitLoop:
		for {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for all messages to be received")
			default:
				receivedMu.Lock()
				current := receivedCount
				receivedMu.Unlock()

				if current >= expectedMessages {
					break waitLoop
				}
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Close notification system to stop subscribers
		ns.Close()
		subscriberWg.Wait()

		// Verify metrics
		metrics := ns.GetMetrics()
		expectedPublished := int64(numPublishers * messagesPerPublisher)
		assert.Equal(t, expectedPublished, metrics.MessagesPublished)

		// Each subscriber should receive all messages
		expectedReceived := expectedPublished * int64(numSubscribers)
		receivedMu.Lock()
		assert.Equal(t, expectedReceived, receivedCount)
		receivedMu.Unlock()
	})
}

func TestRedisNotificationSystem_Metrics(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ns, err := NewRedisNotificationSystem(client, nil)
	require.NoError(t, err)
	defer ns.Close()

	ctx := context.Background()
	channel := "metrics-test"

	t.Run("Should track metrics correctly", func(t *testing.T) {
		// Subscribe to channel
		msgChan, err := ns.Subscribe(ctx, channel)
		require.NoError(t, err)

		// Wait for subscription to be ready
		waitForSubscriptionReady(ctx, t, ns, channel, msgChan)

		// Publish messages
		for i := range 5 {
			err = ns.Publish(ctx, channel, map[string]int{"count": i})
			require.NoError(t, err)
		}

		// Receive messages
		for range 5 {
			select {
			case <-msgChan:
			case <-time.After(time.Second):
				t.Fatal("Did not receive message within timeout")
			}
		}

		// Check metrics
		metrics := ns.GetMetrics()
		// Readiness ping may be included depending on timing
		assert.Contains(t, []int64{5, 6}, metrics.MessagesPublished,
			"Expected exactly 5 (no ping) or 6 (with ping) published messages")
		assert.Contains(t, []int64{5, 6}, metrics.MessagesReceived,
			"Expected exactly 5 (no ping) or 6 (with ping) received messages")
		assert.Equal(t, int64(0), metrics.PublishErrors)
		assert.Equal(t, int64(0), metrics.SubscribeErrors)
		assert.Greater(t, metrics.ActiveChannels, 0)
		assert.Greater(t, metrics.AverageLatency, time.Duration(0))
	})
}
