package core

import "fmt"

// -----------------------------------------------------------------------------
// Store
// -----------------------------------------------------------------------------

type Store struct {
	ExecID   string
	Workflow *State
	Tasks    *StateMap
	Tools    *StateMap
	Agents   *StateMap
}

func NewStore(execID string) *Store {
	return &Store{
		ExecID: execID,
		Tasks:  &StateMap{},
		Tools:  &StateMap{},
		Agents: &StateMap{},
	}
}

func (c *Store) SetWorkflow(state State) {
	c.Workflow = &state
}

func (c *Store) UpsertTask(state *State) error {
	if c.Tasks == nil {
		c.Tasks = &StateMap{}
	}
	if state == nil {
		return fmt.Errorf("failed to upsert task: state is nil")
	}
	id := (*state).ID()
	if _, exists := c.Tasks.Get(id); exists {
		c.Tasks.Remove(id)
	}
	c.Tasks.Add(*state)
	return nil
}

func (c *Store) UpsertTool(state *State) error {
	if c.Tools == nil {
		c.Tools = &StateMap{}
	}
	if state == nil {
		return fmt.Errorf("failed to upsert tool: state is nil")
	}
	id := (*state).ID()
	if _, exists := c.Tools.Get(id); exists {
		c.Tools.Remove(id)
	}
	c.Tools.Add(*state)
	return nil
}

func (c *Store) UpsertAgent(state *State) error {
	if c.Agents == nil {
		c.Agents = &StateMap{}
	}
	if state == nil {
		return fmt.Errorf("failed to upsert agent: state is nil")
	}
	id := (*state).ID()
	if _, exists := c.Agents.Get(id); exists {
		c.Agents.Remove(id)
	}
	c.Agents.Add(*state)
	return nil
}

func (c *Store) GetTask(id StateID) (State, bool) {
	if c.Tasks == nil {
		return nil, false
	}
	return c.Tasks.Get(id)
}

func (c *Store) GetTool(id StateID) (State, bool) {
	if c.Tools == nil {
		return nil, false
	}
	return c.Tools.Get(id)
}

func (c *Store) GetAgent(id StateID) (State, bool) {
	if c.Agents == nil {
		return nil, false
	}
	return c.Agents.Get(id)
}
