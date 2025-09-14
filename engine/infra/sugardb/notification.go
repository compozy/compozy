package sugardb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ccache "github.com/compozy/compozy/engine/infra/cache"
	"github.com/echovault/sugardb/sugardb"
)

// SugarNotificationSystem implements cache.NotificationSystem using SugarDB pub/sub.
type SugarNotificationSystem struct {
	db      *sugardb.SugarDB
	metrics ccache.NotificationMetrics
	mmu     sync.RWMutex
	buffer  int
	mu      sync.Mutex
	closed  bool
	closeCh chan struct{}
	wg      sync.WaitGroup
}

const defaultBuffer = 100

func NewSugarNotificationSystem(db *sugardb.SugarDB, cfg *ccache.Config) (*SugarNotificationSystem, error) {
	if db == nil {
		return nil, fmt.Errorf("sugardb instance cannot be nil")
	}
	buf := defaultBuffer
	if cfg != nil && cfg.CacheConfig != nil && cfg.NotificationBufferSize > 0 {
		buf = cfg.NotificationBufferSize
	}
	return &SugarNotificationSystem{db: db, buffer: buf, closeCh: make(chan struct{})}, nil
}

func (ns *SugarNotificationSystem) Publish(_ context.Context, channel string, message any) error {
	start := time.Now()
	payload, err := json.Marshal(message)
	if err != nil {
		ns.mmu.Lock()
		ns.metrics.PublishErrors++
		ns.mmu.Unlock()
		return err
	}
	ok, err := ns.db.Publish(channel, string(payload))
	if err != nil {
		ns.mmu.Lock()
		ns.metrics.PublishErrors++
		ns.mmu.Unlock()
		return err
	}
	if !ok {
		ns.mmu.Lock()
		ns.metrics.PublishErrors++
		ns.mmu.Unlock()
		return fmt.Errorf("publish failed")
	}
	ns.mmu.Lock()
	ns.metrics.MessagesPublished++
	if ns.metrics.AverageLatency == 0 {
		ns.metrics.AverageLatency = time.Since(start)
	} else {
		ns.metrics.AverageLatency = (ns.metrics.AverageLatency + time.Since(start)) / 2
	}
	ns.mmu.Unlock()
	return nil
}

func (ns *SugarNotificationSystem) Subscribe(ctx context.Context, channels ...string) (<-chan ccache.Message, error) {
	if len(channels) == 0 {
		return nil, fmt.Errorf("at least one channel must be specified")
	}
	read, err := ns.db.Subscribe(fmt.Sprintf("sub_%d", time.Now().UnixNano()), channels...)
	if err != nil {
		ns.mmu.Lock()
		ns.metrics.SubscribeErrors++
		ns.mmu.Unlock()
		return nil, err
	}
	out := make(chan ccache.Message, ns.buffer)
	ns.mmu.Lock()
	ns.metrics.ActiveChannels++
	ns.mmu.Unlock()
	ns.wg.Add(1)
	go ns.loopRead(ctx, out, read)
	return out, nil
}

func (ns *SugarNotificationSystem) SubscribePattern(
	ctx context.Context,
	patterns ...string,
) (<-chan ccache.Message, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("at least one pattern must be specified")
	}
	read, err := ns.db.PSubscribe(fmt.Sprintf("psub_%d", time.Now().UnixNano()), patterns...)
	if err != nil {
		ns.mmu.Lock()
		ns.metrics.SubscribeErrors++
		ns.mmu.Unlock()
		return nil, err
	}
	out := make(chan ccache.Message, ns.buffer)
	ns.mmu.Lock()
	ns.metrics.ActiveChannels++
	ns.mmu.Unlock()
	ns.wg.Add(1)
	go ns.loopRead(ctx, out, read)
	return out, nil
}

func (ns *SugarNotificationSystem) loopRead(
	ctx context.Context,
	out chan<- ccache.Message,
	read sugardb.ReadPubSubMessage,
) {
	defer ns.wg.Done()
	defer close(out)
	defer func() { ns.mmu.Lock(); ns.metrics.ActiveChannels--; ns.mmu.Unlock() }()
	for {
		select {
		case <-ns.closeCh:
			return
		case <-ctx.Done():
			return
		default:
			msg := read()
			if len(msg) >= 3 {
				if msg[0] == "message" {
					out <- ccache.Message{Channel: msg[1], Payload: []byte(msg[2]), Time: time.Now()}
					ns.mmu.Lock()
					ns.metrics.MessagesReceived++
					ns.mmu.Unlock()
				}
			} else {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}
}

func (ns *SugarNotificationSystem) Close() error {
	ns.mu.Lock()
	if ns.closed {
		ns.mu.Unlock()
		return nil
	}
	ns.closed = true
	close(ns.closeCh)
	ns.mu.Unlock()
	ns.wg.Wait()
	return nil
}
func (ns *SugarNotificationSystem) GetMetrics() ccache.NotificationMetrics {
	ns.mmu.RLock()
	defer ns.mmu.RUnlock()
	return ccache.NotificationMetrics{
		MessagesPublished: ns.metrics.MessagesPublished,
		MessagesReceived:  ns.metrics.MessagesReceived,
		PublishErrors:     ns.metrics.PublishErrors,
		SubscribeErrors:   ns.metrics.SubscribeErrors,
		ActiveChannels:    ns.metrics.ActiveChannels,
		AverageLatency:    ns.metrics.AverageLatency,
		DroppedMessages:   ns.metrics.DroppedMessages,
	}
}

func (ns *SugarNotificationSystem) PublishWorkflowEvent(
	ctx context.Context,
	workflowID, event, status string,
	data map[string]any,
) error {
	we := ccache.WorkflowEvent{WorkflowID: workflowID, Event: event, Status: status, Data: data, Timestamp: time.Now()}
	return ns.Publish(ctx, fmt.Sprintf("workflow:%s", workflowID), we)
}

func (ns *SugarNotificationSystem) PublishTaskEvent(
	ctx context.Context,
	taskID, workflowID, event, status string,
	data map[string]any,
) error {
	te := ccache.TaskEvent{
		TaskID:     taskID,
		WorkflowID: workflowID,
		Event:      event,
		Status:     status,
		Data:       data,
		Timestamp:  time.Now(),
	}
	if err := ns.Publish(ctx, fmt.Sprintf("task:%s", taskID), te); err != nil {
		return err
	}
	if workflowID != "" {
		return ns.Publish(ctx, fmt.Sprintf("workflow:%s", workflowID), te)
	}
	return nil
}

func (ns *SugarNotificationSystem) SubscribeToWorkflow(
	ctx context.Context,
	workflowID string,
) (<-chan ccache.Message, error) {
	return ns.Subscribe(ctx, fmt.Sprintf("workflow:%s", workflowID))
}
func (ns *SugarNotificationSystem) SubscribeToTask(ctx context.Context, taskID string) (<-chan ccache.Message, error) {
	return ns.Subscribe(ctx, fmt.Sprintf("task:%s", taskID))
}
func (ns *SugarNotificationSystem) SubscribeToAllWorkflows(ctx context.Context) (<-chan ccache.Message, error) {
	return ns.SubscribePattern(ctx, "workflow:*")
}
func (ns *SugarNotificationSystem) SubscribeToAllTasks(ctx context.Context) (<-chan ccache.Message, error) {
	return ns.SubscribePattern(ctx, "task:*")
}
