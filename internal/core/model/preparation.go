package model

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/core/run/journal"
)

type JournalHandle interface {
	Journal() *journal.Journal
	Close(context.Context) error
}

var _ JournalHandle = (*journal.Owner)(nil)

type SolvePreparation struct {
	Jobs         []Job
	RunArtifacts RunArtifacts
	// JournalHandle carries the run journal while keeping cleanup ownership
	// explicit. Kernel/runtime flows retain responsibility for closing it.
	JournalHandle    JournalHandle
	InputDir         string
	InputDirPath     string
	ResolvedName     string
	ResolvedPR       string
	ResolvedProvider string
	ResolvedRound    int
}

func (p *SolvePreparation) Journal() *journal.Journal {
	if p == nil || p.JournalHandle == nil {
		return nil
	}
	return p.JournalHandle.Journal()
}

func (p *SolvePreparation) SetJournal(j *journal.Journal) {
	if p == nil {
		return
	}
	if j == nil {
		return
	}
	if p.JournalHandle != nil {
		return
	}
	p.JournalHandle = journal.NewOwner(j)
}

func (p *SolvePreparation) CloseJournal(ctx context.Context) error {
	if p == nil || p.JournalHandle == nil {
		return nil
	}
	handle := p.JournalHandle
	if err := handle.Close(ctx); err != nil {
		return fmt.Errorf("close preparation journal: %w", err)
	}
	p.JournalHandle = nil
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
