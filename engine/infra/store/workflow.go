package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/hashicorp/go-memdb"
)

// Schema for the MemDB
const (
	workflowTable       = "workflow"
	taskTable           = "task"
	workflowIDIndex     = "workflow_id"
	workflowExecIDIndex = "workflow_exec_id"
	workflowStatusIndex = "status"
	taskIDIndex         = "task_id"
	taskExecIDIndex     = "task_exec_id"
	taskAgentIDIndex    = "agent_id"
	taskToolIDIndex     = "tool_id"
	taskStatusIndex     = "task_status"
)

// MemDBRepository implements the Repository interface using go-memdb
type MemDBRepository struct {
	db *memdb.MemDB
}

// workflowEntry is a wrapper for workflow.State that provides fields for memdb indexing
type workflowEntry struct {
	WorkflowExecID string
	WorkflowID     string
	Status         string
	*workflow.State
}

// NewMemDBRepository creates a new MemDBRepository
func NewMemDBRepository() (*MemDBRepository, error) {
	schema := createMemDBSchema()
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	return &MemDBRepository{db: db}, nil
}

// createMemDBSchema creates the database schema for the MemDB
func createMemDBSchema() *memdb.DBSchema {
	return &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			workflowTable: createWorkflowTableSchema(),
			taskTable:     createTaskTableSchema(),
		},
	}
}

// createWorkflowTableSchema creates the workflow table schema
func createWorkflowTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: workflowTable,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:   "id",
				Unique: true,
				Indexer: &memdb.StringFieldIndex{
					Field: "WorkflowExecID",
				},
			},
			workflowIDIndex: {
				Name: workflowIDIndex,
				Indexer: &memdb.StringFieldIndex{
					Field: "WorkflowID",
				},
			},
			workflowStatusIndex: {
				Name: workflowStatusIndex,
				Indexer: &memdb.StringFieldIndex{
					Field: "Status",
				},
			},
		},
	}
}

// createTaskTableSchema creates the task table schema
func createTaskTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: taskTable,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:   "id",
				Unique: true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{Field: "WorkflowExecID"},
						&memdb.StringFieldIndex{Field: "TaskExecID"},
					},
				},
			},
			taskIDIndex: {
				Name:   taskIDIndex,
				Unique: false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{Field: "WorkflowExecID"},
						&memdb.StringFieldIndex{Field: "TaskID"},
					},
				},
			},
			taskAgentIDIndex: {
				Name: taskAgentIDIndex,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{Field: "WorkflowExecID"},
						&memdb.StringFieldIndex{Field: "AgentID"},
					},
				},
				AllowMissing: true,
			},
			taskToolIDIndex: {
				Name: taskToolIDIndex,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{Field: "WorkflowExecID"},
						&memdb.StringFieldIndex{Field: "ToolID"},
					},
				},
				AllowMissing: true,
			},
			taskStatusIndex: {
				Name: taskStatusIndex,
				Indexer: &memdb.StringFieldIndex{
					Field: "Status",
				},
			},
		},
	}
}

// Workflow State Operations

func (r *MemDBRepository) UpsertState(_ context.Context, state *workflow.State) error {
	txn := r.db.Txn(true)
	defer txn.Abort()

	entry := &workflowEntry{
		WorkflowExecID: string(state.StateID.WorkflowExec),
		WorkflowID:     state.StateID.WorkflowID,
		Status:         string(state.Status),
		State:          state,
	}

	if err := txn.Insert(workflowTable, entry); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

func (r *MemDBRepository) GetState(_ context.Context, stateID workflow.StateID) (*workflow.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(workflowTable, "id", string(stateID.WorkflowExec))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("workflow state not found")
	}

	val, ok := raw.(*workflowEntry)
	if !ok {
		return nil, fmt.Errorf("failed to cast workflow state")
	}
	return val.State, nil
}

func (r *MemDBRepository) DeleteState(_ context.Context, stateID workflow.StateID) error {
	txn := r.db.Txn(true)
	defer txn.Abort()

	// Get the entry first to delete it
	raw, err := txn.First(workflowTable, "id", string(stateID.WorkflowExec))
	if err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("workflow state not found")
	}

	if err := txn.Delete(workflowTable, raw); err != nil {
		return err
	}

	// Delete associated tasks by iterating
	iter, err := txn.LowerBound(taskTable, taskIDIndex, string(stateID.WorkflowExec), "")
	if err != nil {
		return fmt.Errorf("failed to get tasks for deletion with lowerbound: %w", err)
	}
	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		taskEntry, ok := obj.(*taskEntry)
		if !ok {
			return fmt.Errorf("failed to cast task entry")
		}
		// Ensure we only delete tasks belonging to the current workflow exec ID
		if taskEntry.WorkflowExecID == string(stateID.WorkflowExec) {
			if err := txn.Delete(taskTable, obj); err != nil {
				return fmt.Errorf("failed to delete task: %w", err)
			}
		} else {
			// Stop iterating once we are past the desired WorkflowExecID
			break
		}
	}

	txn.Commit()
	return nil
}

func (r *MemDBRepository) ListStates(_ context.Context, filter *workflow.StateFilter) ([]*workflow.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	var results []*workflow.State
	var iter memdb.ResultIterator
	var err error

	switch {
	case filter.Status != nil:
		iter, err = txn.Get(workflowTable, workflowStatusIndex, string(*filter.Status))
	case filter.WorkflowID != nil:
		iter, err = txn.Get(workflowTable, workflowIDIndex, *filter.WorkflowID)
	default:
		iter, err = txn.Get(workflowTable, "id")
	}
	if err != nil {
		return nil, err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		entry, ok := raw.(*workflowEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast workflow entry")
		}
		results = append(results, entry.State)
	}
	return results, nil
}

// Task Management Operations

type taskEntry struct {
	WorkflowExecID string
	TaskID         string
	TaskExecID     string
	AgentID        string
	ToolID         string
	Status         string
	*task.State
}

func (r *MemDBRepository) AddTaskToWorkflow(
	ctx context.Context,
	workflowStateID workflow.StateID,
	task *task.State,
) error {
	txn := r.db.Txn(true)
	defer txn.Abort()

	// Update workflow state
	workflow, err := r.GetState(ctx, workflowStateID)
	if err != nil {
		return err
	}
	workflow.AddTask(task)

	entry := &workflowEntry{
		WorkflowExecID: string(workflow.StateID.WorkflowExec),
		WorkflowID:     workflow.StateID.WorkflowID,
		Status:         string(workflow.Status),
		State:          workflow,
	}

	if err := txn.Insert(workflowTable, entry); err != nil {
		return err
	}

	// Insert task
	taskEntry := &taskEntry{
		WorkflowExecID: string(workflowStateID.WorkflowExec),
		TaskID:         task.StateID.TaskID,
		TaskExecID:     string(task.StateID.TaskExecID),
		Status:         string(task.Status),
		State:          task,
		AgentID:        "",
		ToolID:         "",
	}

	// Handle nil pointers for AgentID and ToolID
	if task.AgentID != nil {
		taskEntry.AgentID = *task.AgentID
	}
	if task.ToolID != nil {
		taskEntry.ToolID = *task.ToolID
	}

	if err := txn.Insert(taskTable, taskEntry); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

func (r *MemDBRepository) RemoveTaskFromWorkflow(
	ctx context.Context,
	workflowStateID workflow.StateID,
	taskStateID task.StateID,
) error {
	txn := r.db.Txn(true)
	defer txn.Abort()

	// Update workflow state
	workflow, err := r.GetState(ctx, workflowStateID)
	if err != nil {
		return err
	}
	delete(workflow.Tasks, taskStateID.String())

	entry := &workflowEntry{
		WorkflowExecID: string(workflow.StateID.WorkflowExec),
		WorkflowID:     workflow.StateID.WorkflowID,
		Status:         string(workflow.Status),
		State:          workflow,
	}

	if err := txn.Insert(workflowTable, entry); err != nil {
		return err
	}

	// Delete task - find it first
	raw, err := txn.First(taskTable, taskIDIndex, string(workflowStateID.WorkflowExec), taskStateID.TaskID)
	if err != nil {
		return err
	}
	if raw != nil {
		if err := txn.Delete(taskTable, raw); err != nil {
			return err
		}
	}

	txn.Commit()
	return nil
}

func (r *MemDBRepository) UpdateTaskState(
	ctx context.Context,
	workflowStateID workflow.StateID,
	taskStateID task.StateID,
	task *task.State,
) error {
	txn := r.db.Txn(true)
	defer txn.Abort()

	// Update workflow state
	workflow, err := r.GetState(ctx, workflowStateID)
	if err != nil {
		return err
	}
	workflow.Tasks[taskStateID.String()] = task

	wfEntry := &workflowEntry{
		WorkflowExecID: string(workflow.StateID.WorkflowExec),
		WorkflowID:     workflow.StateID.WorkflowID,
		Status:         string(workflow.Status),
		State:          workflow,
	}

	if err := txn.Insert(workflowTable, wfEntry); err != nil {
		return err
	}

	// Update task
	tEntry := &taskEntry{
		WorkflowExecID: string(workflowStateID.WorkflowExec),
		TaskID:         task.StateID.TaskID,
		TaskExecID:     string(task.StateID.TaskExecID),
		Status:         string(task.Status),
		State:          task,
		AgentID:        "",
		ToolID:         "",
	}

	// Handle nil pointers for AgentID and ToolID
	if task.AgentID != nil {
		tEntry.AgentID = *task.AgentID
	}
	if task.ToolID != nil {
		tEntry.ToolID = *task.ToolID
	}

	if err := txn.Insert(taskTable, tEntry); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

// Task Query Operations

func (r *MemDBRepository) GetTaskState(
	_ context.Context,
	workflowStateID workflow.StateID,
	taskStateID task.StateID,
) (*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(taskTable, taskIDIndex, string(workflowStateID.WorkflowExec), taskStateID.TaskID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("task state not found")
	}

	entry, ok := raw.(*taskEntry)
	if !ok {
		return nil, fmt.Errorf("failed to cast task entry")
	}
	return entry.State, nil
}

func (r *MemDBRepository) GetTaskByID(
	_ context.Context,
	workflowStateID workflow.StateID,
	taskID string,
) (*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(taskTable, taskIDIndex, string(workflowStateID.WorkflowExec), taskID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("task not found")
	}

	entry, ok := raw.(*taskEntry)
	if !ok {
		return nil, fmt.Errorf("failed to cast task entry")
	}
	return entry.State, nil
}

func (r *MemDBRepository) GetTaskByExecID(
	_ context.Context,
	workflowStateID workflow.StateID,
	taskExecID core.ID,
) (*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(taskTable, taskExecIDIndex, string(workflowStateID.WorkflowExec), string(taskExecID))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("task not found")
	}

	entry, ok := raw.(*taskEntry)
	if !ok {
		return nil, fmt.Errorf("failed to cast task entry")
	}
	return entry.State, nil
}

func (r *MemDBRepository) GetTaskByAgentID(
	_ context.Context,
	workflowStateID workflow.StateID,
	agentID string,
) (*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.LowerBound(taskTable, taskAgentIDIndex, string(workflowStateID.WorkflowExec), agentID)
	if err != nil {
		return nil, err
	}
	raw := iter.Next() // Get the first item >= the query
	if raw != nil {
		entry, ok := raw.(*taskEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast task entry")
		}
		// Check if it's an exact match
		if entry.WorkflowExecID == string(workflowStateID.WorkflowExec) && entry.AgentID == agentID {
			return entry.State, nil
		}
	}

	return nil, fmt.Errorf("task not found")
}

func (r *MemDBRepository) GetTaskByToolID(
	_ context.Context,
	workflowStateID workflow.StateID,
	toolID string,
) (*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.LowerBound(taskTable, taskToolIDIndex, string(workflowStateID.WorkflowExec), toolID)
	if err != nil {
		return nil, err
	}
	raw := iter.Next() // Get the first item >= the query
	if raw != nil {
		entry, ok := raw.(*taskEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast task entry")
		}
		// Check if it's an exact match
		if entry.WorkflowExecID == string(workflowStateID.WorkflowExec) && entry.ToolID == toolID {
			return entry.State, nil
		}
	}

	return nil, fmt.Errorf("task not found")
}

// Task List Operations

func (r *MemDBRepository) ListTasksInWorkflow(
	_ context.Context,
	workflowStateID workflow.StateID,
) (map[string]*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	results := make(map[string]*task.State)
	iter, err := txn.LowerBound(taskTable, taskIDIndex, string(workflowStateID.WorkflowExec), "")
	if err != nil {
		return nil, err
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		entry, ok := raw.(*taskEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast task entry")
		}
		// Ensure we only list tasks belonging to the current workflow exec ID
		if entry.WorkflowExecID == string(workflowStateID.WorkflowExec) {
			results[entry.StateID.String()] = entry.State
		} else {
			// Stop iterating once we are past the desired WorkflowExecID
			break
		}
	}

	return results, nil
}

func (r *MemDBRepository) ListTasksByStatus(
	_ context.Context,
	workflowStateID workflow.StateID,
	status core.StatusType,
) ([]*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(taskTable, taskStatusIndex, string(status))
	if err != nil {
		return nil, err
	}

	var results []*task.State
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		entry, ok := raw.(*taskEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast task entry")
		}
		if entry.WorkflowExecID == string(workflowStateID.WorkflowExec) {
			results = append(results, entry.State)
		}
	}

	return results, nil
}

func (r *MemDBRepository) ListTasksByAgent(
	_ context.Context,
	workflowStateID workflow.StateID,
	agentID string,
) ([]*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	var results []*task.State
	// Use LowerBound with the full compound key
	iter, err := txn.LowerBound(taskTable, taskAgentIDIndex, string(workflowStateID.WorkflowExec), agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks by agent with lowerbound: %w", err)
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		entry, ok := raw.(*taskEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast task entry")
		}
		// Ensure we only list tasks matching the full compound key since LowerBound gives >=
		if entry.WorkflowExecID == string(workflowStateID.WorkflowExec) && entry.AgentID == agentID {
			results = append(results, entry.State)
		} else {
			// If WorkflowExecID or AgentID no longer matches, we've passed the relevant records.
			break
		}
	}
	return results, nil
}

func (r *MemDBRepository) ListTasksByTool(
	_ context.Context,
	workflowStateID workflow.StateID,
	toolID string,
) ([]*task.State, error) {
	txn := r.db.Txn(false)
	defer txn.Abort()

	var results []*task.State
	iter, err := txn.LowerBound(taskTable, taskToolIDIndex, string(workflowStateID.WorkflowExec), toolID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks by tool with lowerbound: %w", err)
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		entry, ok := raw.(*taskEntry)
		if !ok {
			return nil, fmt.Errorf("failed to cast task entry")
		}
		// Ensure we only list tasks matching the full compound key since LowerBound gives >=
		if entry.WorkflowExecID == string(workflowStateID.WorkflowExec) && entry.ToolID == toolID {
			results = append(results, entry.State)
		} else {
			// If WorkflowExecID or ToolID no longer matches, we've passed the relevant records.
			break
		}
	}
	return results, nil
}
