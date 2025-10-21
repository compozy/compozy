package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Publisher defines the interface for publishing messages
type Publisher interface {
	Publish(ctx context.Context, channel string, message any) error
	Close() error
}

// Subscriber defines the interface for subscribing to messages
type Subscriber interface {
	Subscribe(ctx context.Context, channels ...string) (<-chan Message, error)
	SubscribePattern(ctx context.Context, patterns ...string) (<-chan Message, error)
	Close() error
}

// Message represents a pub/sub message
type Message struct {
	Channel string    `json:"channel"`
	Payload []byte    `json:"payload"`
	Time    time.Time `json:"time"`
}

// NotificationSystem combines publisher and subscriber functionality
type NotificationSystem interface {
	Publisher
	Subscriber
	GetMetrics() NotificationMetrics

	// Helper methods for common notification patterns
	PublishWorkflowEvent(ctx context.Context, workflowID, event, status string, data map[string]any) error
	PublishTaskEvent(ctx context.Context, taskID, workflowID, event, status string, data map[string]any) error
	SubscribeToWorkflow(ctx context.Context, workflowID string) (<-chan Message, error)
	SubscribeToTask(ctx context.Context, taskID string) (<-chan Message, error)
	SubscribeToAllWorkflows(ctx context.Context) (<-chan Message, error)
	SubscribeToAllTasks(ctx context.Context) (<-chan Message, error)
}

// NotificationMetrics tracks pub/sub operation metrics
type NotificationMetrics struct {
	mu                sync.RWMutex
	MessagesPublished int64         `json:"messages_published"`
	MessagesReceived  int64         `json:"messages_received"`
	PublishErrors     int64         `json:"publish_errors"`
	SubscribeErrors   int64         `json:"subscribe_errors"`
	ActiveChannels    int           `json:"active_channels"`
	AverageLatency    time.Duration `json:"average_latency"`
	DroppedMessages   int64         `json:"dropped_messages"`
}

// RedisNotificationSystem implements the NotificationSystem interface using Redis pub/sub
type RedisNotificationSystem struct {
	client     RedisInterface
	metrics    *NotificationMetrics
	closeCh    chan struct{}
	wg         sync.WaitGroup
	closed     bool
	mu         sync.Mutex
	bufferSize int
}

const DefaultNotificationBufferSize = 100

// NewRedisNotificationSystem creates a new Redis-backed notification system
func NewRedisNotificationSystem(client RedisInterface, config *Config) (*RedisNotificationSystem, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}
	bufferSize := DefaultNotificationBufferSize
	if config != nil && config.NotificationBufferSize > 0 {
		bufferSize = config.NotificationBufferSize
	}
	return &RedisNotificationSystem{
		client:     client,
		metrics:    &NotificationMetrics{},
		closeCh:    make(chan struct{}),
		bufferSize: bufferSize,
	}, nil
}

// Publish sends a message to the specified channel
func (ns *RedisNotificationSystem) Publish(ctx context.Context, channel string, message any) error {
	start := time.Now()
	payload, err := json.Marshal(message)
	if err != nil {
		ns.metrics.mu.Lock()
		ns.metrics.PublishErrors++
		ns.metrics.mu.Unlock()
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	result := ns.client.Publish(ctx, channel, payload)
	if err := result.Err(); err != nil {
		ns.metrics.mu.Lock()
		ns.metrics.PublishErrors++
		ns.metrics.mu.Unlock()
		return fmt.Errorf("failed to publish message: %w", err)
	}
	ns.metrics.mu.Lock()
	ns.metrics.MessagesPublished++
	latency := time.Since(start)
	if ns.metrics.AverageLatency == 0 {
		ns.metrics.AverageLatency = latency
	} else {
		ns.metrics.AverageLatency = (ns.metrics.AverageLatency + latency) / 2
	}
	ns.metrics.mu.Unlock()
	// Note: Message published to channel with subscribers and latency info available in metrics
	return nil
}

// Subscribe subscribes to one or more channels and returns a message channel
func (ns *RedisNotificationSystem) Subscribe(ctx context.Context, channels ...string) (<-chan Message, error) {
	if len(channels) == 0 {
		return nil, fmt.Errorf("at least one channel must be specified")
	}
	pubsub := ns.client.Subscribe(ctx, channels...)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		pubsub.Close()
		ns.metrics.mu.Lock()
		ns.metrics.SubscribeErrors++
		ns.metrics.mu.Unlock()
		return nil, fmt.Errorf("failed to confirm subscription: %w", err)
	}
	msgChan := make(chan Message, ns.bufferSize) // Buffered to prevent blocking
	ns.metrics.mu.Lock()
	ns.metrics.ActiveChannels++
	ns.metrics.mu.Unlock()
	ns.wg.Add(1)
	go ns.receiveMessages(ctx, pubsub, msgChan, channels)
	// Note: Successfully subscribed to channels
	return msgChan, nil
}

// SubscribePattern subscribes to channels matching patterns
func (ns *RedisNotificationSystem) SubscribePattern(ctx context.Context, patterns ...string) (<-chan Message, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("at least one pattern must be specified")
	}
	pubsub := ns.client.PSubscribe(ctx, patterns...)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		pubsub.Close()
		ns.metrics.mu.Lock()
		ns.metrics.SubscribeErrors++
		ns.metrics.mu.Unlock()
		return nil, fmt.Errorf("failed to confirm pattern subscription: %w", err)
	}
	msgChan := make(chan Message, ns.bufferSize) // Buffered to prevent blocking
	ns.metrics.mu.Lock()
	ns.metrics.ActiveChannels++
	ns.metrics.mu.Unlock()
	ns.wg.Add(1)
	go ns.receiveMessages(ctx, pubsub, msgChan, patterns)
	// Note: Successfully subscribed to patterns
	return msgChan, nil
}

// Close shuts down the notification system
func (ns *RedisNotificationSystem) Close() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.closed {
		return nil // Already closed
	}
	ns.closed = true
	close(ns.closeCh)
	ns.wg.Wait()
	// Note: Notification system closed
	return nil
}

// GetMetrics returns current notification metrics
func (ns *RedisNotificationSystem) GetMetrics() NotificationMetrics {
	ns.metrics.mu.RLock()
	defer ns.metrics.mu.RUnlock()
	return NotificationMetrics{
		MessagesPublished: ns.metrics.MessagesPublished,
		MessagesReceived:  ns.metrics.MessagesReceived,
		PublishErrors:     ns.metrics.PublishErrors,
		SubscribeErrors:   ns.metrics.SubscribeErrors,
		ActiveChannels:    ns.metrics.ActiveChannels,
		AverageLatency:    ns.metrics.AverageLatency,
		DroppedMessages:   ns.metrics.DroppedMessages,
	}
}

// receiveMessages handles incoming messages from Redis pub/sub
func (ns *RedisNotificationSystem) receiveMessages(
	ctx context.Context,
	pubsub *redis.PubSub,
	msgChan chan<- Message,
	_ []string,
) {
	defer ns.wg.Done()
	defer close(msgChan)
	defer pubsub.Close()
	defer ns.decrementActiveChannels()
	ch := pubsub.Channel()
	for {
		select {
		case <-ns.closeCh:
			return
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			ns.handleIncomingMessage(msgChan, msg)
		}
	}
}

// decrementActiveChannels updates active channel metrics when the receiver exits.
func (ns *RedisNotificationSystem) decrementActiveChannels() {
	ns.metrics.mu.Lock()
	ns.metrics.ActiveChannels--
	ns.metrics.mu.Unlock()
}

// handleIncomingMessage converts and forwards a Redis message to subscribers.
func (ns *RedisNotificationSystem) handleIncomingMessage(msgChan chan<- Message, msg *redis.Message) {
	message := Message{
		Channel: msg.Channel,
		Payload: []byte(msg.Payload),
		Time:    time.Now(),
	}
	if ns.tryDeliverMessage(msgChan, message) {
		ns.recordMessageReceived()
		return
	}
	ns.recordDroppedMessage()
}

// tryDeliverMessage attempts a non-blocking send to the consumer channel.
func (ns *RedisNotificationSystem) tryDeliverMessage(msgChan chan<- Message, message Message) bool {
	select {
	case msgChan <- message:
		return true
	default:
		return false
	}
}

// recordMessageReceived increments the received message counter.
func (ns *RedisNotificationSystem) recordMessageReceived() {
	ns.metrics.mu.Lock()
	ns.metrics.MessagesReceived++
	ns.metrics.mu.Unlock()
}

// recordDroppedMessage increments the dropped message counter.
func (ns *RedisNotificationSystem) recordDroppedMessage() {
	ns.metrics.mu.Lock()
	ns.metrics.DroppedMessages++
	ns.metrics.mu.Unlock()
}

// WorkflowEvent represents a workflow status change event
type WorkflowEvent struct {
	WorkflowID string         `json:"workflow_id"`
	Event      string         `json:"event"`
	Status     string         `json:"status"`
	Data       map[string]any `json:"data,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// TaskEvent represents a task status change event
type TaskEvent struct {
	TaskID     string         `json:"task_id"`
	WorkflowID string         `json:"workflow_id,omitempty"`
	Event      string         `json:"event"`
	Status     string         `json:"status"`
	Data       map[string]any `json:"data,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Helper functions for common notification patterns

// PublishWorkflowEvent publishes a workflow status change event
func (ns *RedisNotificationSystem) PublishWorkflowEvent(
	ctx context.Context,
	workflowID, event, status string,
	data map[string]any,
) error {
	workflowEvent := WorkflowEvent{
		WorkflowID: workflowID,
		Event:      event,
		Status:     status,
		Data:       data,
		Timestamp:  time.Now(),
	}
	channel := fmt.Sprintf("workflow:%s", workflowID)
	return ns.Publish(ctx, channel, workflowEvent)
}

// PublishTaskEvent publishes a task status change event
func (ns *RedisNotificationSystem) PublishTaskEvent(
	ctx context.Context,
	taskID, workflowID, event, status string,
	data map[string]any,
) error {
	taskEvent := TaskEvent{
		TaskID:     taskID,
		WorkflowID: workflowID,
		Event:      event,
		Status:     status,
		Data:       data,
		Timestamp:  time.Now(),
	}
	taskChannel := fmt.Sprintf("task:%s", taskID)
	if err := ns.Publish(ctx, taskChannel, taskEvent); err != nil {
		return err
	}
	if workflowID != "" {
		workflowChannel := fmt.Sprintf("workflow:%s", workflowID)
		return ns.Publish(ctx, workflowChannel, taskEvent)
	}
	return nil
}

// SubscribeToWorkflow subscribes to all events for a specific workflow
func (ns *RedisNotificationSystem) SubscribeToWorkflow(ctx context.Context, workflowID string) (<-chan Message, error) {
	channel := fmt.Sprintf("workflow:%s", workflowID)
	return ns.Subscribe(ctx, channel)
}

// SubscribeToTask subscribes to all events for a specific task
func (ns *RedisNotificationSystem) SubscribeToTask(ctx context.Context, taskID string) (<-chan Message, error) {
	channel := fmt.Sprintf("task:%s", taskID)
	return ns.Subscribe(ctx, channel)
}

// SubscribeToAllWorkflows subscribes to all workflow events using pattern matching
func (ns *RedisNotificationSystem) SubscribeToAllWorkflows(ctx context.Context) (<-chan Message, error) {
	return ns.SubscribePattern(ctx, "workflow:*")
}

// SubscribeToAllTasks subscribes to all task events using pattern matching
func (ns *RedisNotificationSystem) SubscribeToAllTasks(ctx context.Context) (<-chan Message, error) {
	return ns.SubscribePattern(ctx, "task:*")
}
