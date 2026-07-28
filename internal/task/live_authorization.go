package task

import (
	"context"

	"sort"
	"strings"
)

func (m *Service) streamBacklog(
	ctx context.Context,
	rootTaskID string,
	afterSequence int64,
	actor ActorContext,
) ([]StreamEvent, error) {
	records, err := m.listTaskTreeEventRecords(ctx, rootTaskID, afterSequence, actor)
	if err != nil {
		return nil, err
	}
	items, err := m.timelineItemsFromRecords(ctx, records)
	if err != nil {
		return nil, err
	}

	events := make([]StreamEvent, 0, len(items))
	for _, item := range items {
		events = append(events, StreamEvent{
			Sequence: item.Sequence,
			Type:     item.EventType,
			Timeline: item,
		})
	}
	return events, nil
}

func (m *Service) listTaskTreeEventRecords(
	ctx context.Context,
	rootTaskID string,
	afterSequence int64,
	actor ActorContext,
) ([]EventRecord, error) {
	tree, err := m.collectTaskTree(ctx, rootTaskID)
	if err != nil {
		return nil, err
	}
	tree = m.filterAuthorizedTaskTree(ctx, actor, tree)

	records := make([]EventRecord, 0)
	for _, record := range tree {
		taskRecords, err := m.store.ListTaskEventRecords(ctx, EventRecordQuery{
			TaskID:        record.ID,
			AfterSequence: afterSequence,
		})
		if err != nil {
			return nil, err
		}
		records = append(records, taskRecords...)
	}

	sort.SliceStable(records, func(i, j int) bool {
		if !records[i].Event.Timestamp.Equal(records[j].Event.Timestamp) {
			return records[i].Event.Timestamp.Before(records[j].Event.Timestamp)
		}
		if records[i].Sequence != records[j].Sequence {
			return records[i].Sequence < records[j].Sequence
		}
		return records[i].Event.ID < records[j].Event.ID
	})
	return m.filterAuthorizedEventRecords(ctx, actor, records)
}

func (m *Service) filterAuthorizedTaskTree(
	ctx context.Context,
	actor ActorContext,
	tree []Task,
) []Task {
	if isTaskOperator(actor) {
		return tree
	}
	hidden := make(map[string]struct{})
	visible := make([]Task, 0, len(tree))
	for _, record := range tree {
		if _, parentHidden := hidden[strings.TrimSpace(record.ParentTaskID)]; parentHidden {
			hidden[record.ID] = struct{}{}
			continue
		}
		if err := m.authorizeTaskResource(ctx, actor, record); err != nil {
			hidden[record.ID] = struct{}{}
			continue
		}
		visible = append(visible, record)
	}
	return visible
}

func (m *Service) filterAuthorizedEventRecords(
	ctx context.Context,
	actor ActorContext,
	records []EventRecord,
) ([]EventRecord, error) {
	if isTaskOperator(actor) {
		return records, nil
	}
	visible := make([]EventRecord, 0, len(records))
	for _, record := range records {
		allowed, err := m.authorizeEventRecord(ctx, actor, record)
		if err != nil {
			return nil, err
		}
		if allowed {
			visible = append(visible, record)
		}
	}
	return visible, nil
}

func (m *Service) authorizeEventRecord(
	ctx context.Context,
	actor ActorContext,
	record EventRecord,
) (bool, error) {
	taskRecord, err := m.store.GetTask(ctx, record.Event.TaskID)
	if err != nil {
		return false, err
	}
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
		return false, nil
	}
	if strings.TrimSpace(record.Event.RunID) == "" {
		return true, nil
	}
	run, err := m.store.GetTaskRun(ctx, record.Event.RunID)
	if err != nil {
		return false, err
	}
	if err := m.runReadAuthorizer.AuthorizeRunRead(ctx, actor, run, &taskRecord); err != nil {
		return false, nil
	}
	return true, nil
}
