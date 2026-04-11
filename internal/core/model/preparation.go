package model

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type SolvePreparation struct {
	Jobs         []Job
	RunArtifacts RunArtifacts
	// RunScope carries the runtime resources that were allocated before
	// planning began. Kernel/runtime flows retain responsibility for closing it.
	RunScope         RunScope
	InputDir         string
	InputDirPath     string
	ResolvedName     string
	ResolvedPR       string
	ResolvedProvider string
	ResolvedRound    int
}

func (p *SolvePreparation) Journal() *journal.Journal {
	if p == nil || p.RunScope == nil {
		return nil
	}
	return p.RunScope.RunJournal()
}

func (p *SolvePreparation) EventBus() *events.Bus[events.Event] {
	if p == nil || p.RunScope == nil {
		return nil
	}
	return p.RunScope.RunEventBus()
}

func (p *SolvePreparation) RuntimeManager() RuntimeManager {
	if p == nil || p.RunScope == nil {
		return nil
	}
	return p.RunScope.RunManager()
}

func (p *SolvePreparation) SetJournal(j *journal.Journal) {
	p.SetRunScope(&BaseRunScope{Journal: j})
}

func (p *SolvePreparation) SetRunScope(scope RunScope) {
	if p == nil {
		return
	}
	if scope == nil {
		return
	}
	if p.RunScope != nil {
		return
	}
	p.RunScope = scope
}

func (p *SolvePreparation) CloseJournal(ctx context.Context) error {
	if p == nil || p.RunScope == nil {
		return nil
	}
	handle := p.RunScope
	if err := handle.Close(ctx); err != nil {
		return fmt.Errorf("close preparation journal: %w", err)
	}
	p.RunScope = nil
	return nil
}

type Job struct {
	CodeFiles     []string
	Groups        map[string][]IssueEntry
	TaskTitle     string
	TaskType      string
	SafeName      string
	Prompt        []byte
	SystemPrompt  string
	MCPServers    []MCPServer
	OutPromptPath string
	OutLog        string
	ErrLog        string
}

func (j Job) IssueCount() int {
	total := 0
	for _, items := range j.Groups {
		total += len(items)
	}
	return total
}
