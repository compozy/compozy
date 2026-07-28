package task

import (
	"context"

	"strings"
)

func (m *Service) registerTaskSubscriber(taskID string, actor ActorContext) *taskStreamSubscriber {
	m.liveMu.Lock()
	defer m.liveMu.Unlock()

	m.nextSubscriberID++
	subscriber := newTaskStreamSubscriber(m.nextSubscriberID, taskID, actor)
	m.liveSubscribers[subscriber.id] = subscriber
	return subscriber
}

func (m *Service) unregisterTaskSubscriber(id uint64) {
	m.liveMu.Lock()
	defer m.liveMu.Unlock()
	delete(m.liveSubscribers, id)
}

func (m *Service) runTaskStreamSubscriber(
	ctx context.Context,
	subscriber *taskStreamSubscriber,
	afterSequence int64,
	backlog []StreamEvent,
) {
	defer func() {
		m.unregisterTaskSubscriber(subscriber.id)
		subscriber.stop()
		close(subscriber.out)
	}()

	nextSequence := afterSequence
	emit := func(event StreamEvent) bool {
		if event.Sequence <= nextSequence {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case subscriber.out <- event:
			nextSequence = event.Sequence
			return true
		}
	}

	for _, event := range backlog {
		if !emit(event) {
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-subscriber.done:
			return
		case event := <-subscriber.deliver:
			if !emit(event) {
				return
			}
		}
	}
}

func (m *Service) emitTaskLiveEventBestEffort(ctx context.Context, eventID string) {
	record, err := m.store.GetTaskEventRecord(ctx, eventID)
	if err != nil {
		return
	}
	m.emitTaskLiveRecordBestEffort(ctx, record)
}

func (m *Service) emitTaskLiveRecordBestEffort(ctx context.Context, record EventRecord) {
	roots, err := m.ancestorTaskIDs(ctx, record.Event.TaskID)
	if err != nil {
		return
	}

	subscribers := m.taskSubscribersForRoots(roots)
	eligible := make([]*taskStreamSubscriber, 0, len(subscribers))
	for _, subscriber := range subscribers {
		allowed, authorizeErr := m.authorizeEventRecord(ctx, subscriber.actor, record)
		if authorizeErr == nil && allowed {
			eligible = append(eligible, subscriber)
		}
	}
	if len(eligible) == 0 {
		return
	}
	items, err := m.timelineItemsFromRecords(ctx, []EventRecord{record})
	if err != nil || len(items) == 0 {
		return
	}
	event := StreamEvent{Sequence: items[0].Sequence, Type: items[0].EventType, Timeline: items[0]}
	for _, subscriber := range eligible {
		if subscriber.enqueue(event) {
			continue
		}
		m.unregisterTaskSubscriber(subscriber.id)
		subscriber.stop()
	}
}

func (m *Service) ancestorTaskIDs(ctx context.Context, taskID string) ([]string, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0, 4)

	currentID := strings.TrimSpace(taskID)
	for currentID != "" {
		if _, ok := seen[currentID]; ok {
			break
		}
		seen[currentID] = struct{}{}
		ids = append(ids, currentID)

		record, err := m.store.GetTask(ctx, currentID)
		if err != nil {
			return nil, err
		}
		currentID = strings.TrimSpace(record.ParentTaskID)
	}

	return ids, nil
}

func (m *Service) taskSubscribersForRoots(rootTaskIDs []string) []*taskStreamSubscriber {
	rootSet := make(map[string]struct{}, len(rootTaskIDs))
	for _, rootTaskID := range rootTaskIDs {
		rootSet[rootTaskID] = struct{}{}
	}

	m.liveMu.Lock()
	defer m.liveMu.Unlock()

	subscribers := make([]*taskStreamSubscriber, 0, len(rootSet))
	for _, subscriber := range m.liveSubscribers {
		if _, ok := rootSet[subscriber.taskID]; ok {
			subscribers = append(subscribers, subscriber)
		}
	}
	return subscribers
}
