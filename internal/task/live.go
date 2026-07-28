package task

import (
	"context"

	"sort"
	"strings"
	"sync"
)

const taskStreamBufferSize = 256

const (
	sessionEventTypeToolCall   = "tool_call"
	sessionEventTypeToolResult = "tool_result"
)

type taskLiveContext struct {
	record    Task
	reference Reference
	runsByID  map[string]Run
}

type taskStreamSubscriber struct {
	id        uint64
	taskID    string
	actor     ActorContext
	deliver   chan StreamEvent
	done      chan struct{}
	out       chan StreamEvent
	closeOnce sync.Once
}

func newTaskStreamSubscriber(id uint64, taskID string, actor ActorContext) *taskStreamSubscriber {
	return &taskStreamSubscriber{
		id:      id,
		taskID:  taskID,
		actor:   actor,
		deliver: make(chan StreamEvent, taskStreamBufferSize),
		done:    make(chan struct{}),
		out:     make(chan StreamEvent),
	}
}

func (s *taskStreamSubscriber) enqueue(event StreamEvent) bool {
	select {
	case <-s.done:
		return false
	case s.deliver <- event:
		return true
	default:
		return false
	}
}

func (s *taskStreamSubscriber) stop() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

func (m *Service) Timeline(
	ctx context.Context,
	taskID string,
	query TimelineQuery,
	actor ActorContext,
) ([]TimelineItem, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}
	if err := query.Validate("task_timeline_query"); err != nil {
		return nil, err
	}

	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedTaskID == "" {
		return nil, ErrValidation
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedTaskID, actor); err != nil {
		return nil, err
	}

	records, err := m.store.ListTaskEventRecords(ctx, EventRecordQuery{
		TaskID:        trimmedTaskID,
		AfterSequence: query.AfterSequence,
		Limit:         query.Limit,
	})
	if err != nil {
		return nil, err
	}
	visible, err := m.filterAuthorizedEventRecords(ctx, actor, records)
	if err != nil {
		return nil, err
	}
	return m.timelineItemsFromRecords(ctx, visible)
}

func (m *Service) Stream(
	ctx context.Context,
	taskID string,
	query StreamQuery,
	actor ActorContext,
) (<-chan StreamEvent, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}
	if err := query.Validate("task_stream_query"); err != nil {
		return nil, err
	}

	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedTaskID == "" {
		return nil, ErrValidation
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedTaskID, actor); err != nil {
		return nil, err
	}

	subscriber := m.registerTaskSubscriber(trimmedTaskID, actor)
	backlog, err := m.streamBacklog(ctx, trimmedTaskID, query.AfterSequence, actor)
	if err != nil {
		m.unregisterTaskSubscriber(subscriber.id)
		subscriber.stop()
		close(subscriber.out)
		return nil, err
	}

	go m.runTaskStreamSubscriber(ctx, subscriber, query.AfterSequence, backlog)
	return subscriber.out, nil
}

func (m *Service) Tree(ctx context.Context, taskID string, actor ActorContext) (*TreeView, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}

	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedTaskID == "" {
		return nil, ErrValidation
	}
	tree, err := m.collectTaskTree(ctx, trimmedTaskID)
	if err != nil {
		return nil, err
	}
	if len(tree) == 0 {
		return nil, ErrTaskNotFound
	}
	if err := m.authorizeTaskResource(ctx, actor, tree[0]); err != nil {
		return nil, err
	}
	tree = m.filterAuthorizedTaskTree(ctx, actor, tree)

	nodes, err := m.buildTreeNodes(ctx, tree, actor)
	if err != nil {
		return nil, err
	}
	view := splitTreeView(trimmedTaskID, nodes)
	return &view, nil
}

func (m *Service) buildTreeNodes(
	ctx context.Context,
	tree []Task,
	actor ActorContext,
) ([]TreeNode, error) {
	depthByID := make(map[string]int, len(tree))
	depthByID[tree[0].ID] = 0
	childCountByID := make(map[string]int, len(tree))
	for _, record := range tree[1:] {
		childCountByID[record.ParentTaskID]++
	}

	nodes := make([]TreeNode, 0, len(tree))
	for _, record := range tree {
		node, err := m.buildTreeNode(ctx, record, depthByID, childCountByID, actor)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Depth != nodes[j].Depth {
			return nodes[i].Depth < nodes[j].Depth
		}
		if !nodes[i].LastActivityAt.Equal(nodes[j].LastActivityAt) {
			return nodes[i].LastActivityAt.After(nodes[j].LastActivityAt)
		}
		return nodes[i].Task.ID < nodes[j].Task.ID
	})
	return nodes, nil
}

func (m *Service) buildTreeNode(
	ctx context.Context,
	record Task,
	depthByID map[string]int,
	childCountByID map[string]int,
	actor ActorContext,
) (TreeNode, error) {
	if strings.TrimSpace(record.ParentTaskID) != "" {
		depthByID[record.ID] = depthByID[record.ParentTaskID] + 1
	}

	dependencies, err := m.store.ListDependencies(ctx, record.ID)
	if err != nil {
		return TreeNode{}, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return TreeNode{}, err
	}
	visibleRuns, err := m.filterAuthorizedRuns(ctx, actor, record, runs)
	if err != nil {
		return TreeNode{}, err
	}
	events, err := m.store.ListTaskEvents(ctx, EventQuery{TaskID: record.ID, Limit: 1})
	if err != nil {
		return TreeNode{}, err
	}
	summary, err := m.enrichTaskSummaryFromState(ctx, record, childCountByID[record.ID], dependencies, runs, events)
	if err != nil {
		return TreeNode{}, err
	}

	return TreeNode{
		Task:           taskReferenceFromTask(record, summary.Status),
		ParentTaskID:   record.ParentTaskID,
		Depth:          depthByID[record.ID],
		ChildCount:     childCountByID[record.ID],
		ActiveRun:      activeRunSummary(visibleRuns, record.MaxAttempts),
		LastActivityAt: summary.LastActivityAt,
	}, nil
}

func splitTreeView(rootTaskID string, nodes []TreeNode) TreeView {
	rootIndex := 0
	for idx := range nodes {
		if nodes[idx].Task.ID == rootTaskID {
			rootIndex = idx
			break
		}
	}

	root := nodes[rootIndex]
	descendants := make([]TreeNode, 0, len(nodes)-1)
	for idx, node := range nodes {
		if idx == rootIndex {
			continue
		}
		descendants = append(descendants, node)
	}
	return TreeView{Root: root, Descendants: descendants}
}
