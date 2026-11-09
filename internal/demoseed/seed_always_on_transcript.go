package demoseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	mainTurnInputPerStep  = 1400
	mainTurnOutputPerStep = 260
	mainTurnUSDPerStep    = 0.006
)

// mainSegment is one block of the main session in time order: a conversation turn or a report.
type mainSegment struct {
	at     time.Time
	steps  []transcriptStep
	report *agentinstance.Report
}

// writeMainSession writes the main session the way it reads after weeks of use: the setup
// conversation, every published report in place, and the turns each pick started.
func writeMainSession(ctx context.Context, db *globaldb.GlobalDB, state *scenario, run *instanceRun) (int, error) {
	segments, err := mainSegments(ctx, db, run)
	if err != nil {
		return 0, err
	}
	dir := mainSessionDir(state, run)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, fmt.Errorf("demo seed: create main session directory for %q: %w", run.story.Agent, err)
	}
	events, usage, err := mainSessionEvents(run, segments)
	if err != nil {
		return 0, err
	}
	persisted, err := recordMainSession(ctx, dir, run, events, usage)
	if err != nil {
		return 0, err
	}
	reports := 0
	for _, event := range persisted {
		reportID, ok := strings.CutPrefix(event.ID, "report:")
		if !ok {
			continue
		}
		if err := db.InstanceReports().
			MarkReportPublished(ctx, reportID, run.story.MainID, event.Sequence); err != nil {
			return 0, fmt.Errorf("demo seed: publish report %q for %q: %w", reportID, run.story.Agent, err)
		}
		reports++
	}
	last := run.story.CreatedAt
	if len(persisted) > 0 {
		last = persisted[len(persisted)-1].Timestamp
	}
	if err := writeMainSessionMeta(dir, run.record, run, last); err != nil {
		return 0, err
	}
	state.writeClock.at(last)
	if err := db.UpdateSessionState(ctx, store.SessionStateUpdate{
		ID: run.story.MainID, State: string(session.StateStopped), UpdatedAt: last,
	}); err != nil {
		return 0, fmt.Errorf("demo seed: stamp main session activity for %q: %w", run.story.Agent, err)
	}
	return reports, nil
}

func mainSegments(ctx context.Context, db *globaldb.GlobalDB, run *instanceRun) ([]mainSegment, error) {
	segments := make([]mainSegment, 0, len(run.story.Turns)+len(run.story.Beats)*2)
	for _, turn := range run.story.Turns {
		segments = append(segments, mainSegment{at: turn.At, steps: turn.Steps})
	}
	for _, beat := range run.story.Beats {
		report, err := reportAt(ctx, db, run.instance.ID, beat.At)
		if err != nil {
			return nil, fmt.Errorf("demo seed: find %q report: %w", run.story.Agent, err)
		}
		segments = append(segments, mainSegment{at: beat.At, report: &report})
		pickedAt := beat.PickAt
		if pickedAt.IsZero() && run.picks[beat.At] {
			pickedAt = beat.At.Add(autoPickDispatchWait)
		}
		if pickedAt.IsZero() {
			continue
		}
		text, err := pickText(run.story, beat)
		if err != nil {
			return nil, err
		}
		steps := append([]transcriptStep{operatorSays(text)}, beat.Reply...)
		segments = append(segments, mainSegment{at: pickedAt.Add(pickReplyDelay / 2), steps: steps})
	}
	slices.SortStableFunc(segments, func(a, b mainSegment) int { return a.at.Compare(b.at) })
	return segments, nil
}

func mainSessionEvents(run *instanceRun, segments []mainSegment) ([]store.SessionEvent, []store.TokenUsage, error) {
	events := []store.SessionEvent{}
	usage := []store.TokenUsage{}
	for turnIndex, segment := range segments {
		if segment.report != nil {
			event, err := reportEvent(run.story.MainID, run.story.Agent, *segment.report)
			if err != nil {
				return nil, nil, err
			}
			events = append(events, event)
			continue
		}
		turnID := fmt.Sprintf("turn_%s_%02d", run.story.MainID, turnIndex+1)
		turnEvents, turnUsage, err := mainTurnEvents(run, turnID, segment)
		if err != nil {
			return nil, nil, err
		}
		events = append(events, turnEvents...)
		usage = append(usage, turnUsage)
	}
	for index := range events {
		if events[index].ID == "" {
			events[index].ID = fmt.Sprintf("evt_%s_%03d", run.story.MainID, index+1)
		}
	}
	return events, usage, nil
}

func mainTurnEvents(
	run *instanceRun,
	turnID string,
	segment mainSegment,
) ([]store.SessionEvent, store.TokenUsage, error) {
	story := sessionStory{ID: run.story.MainID, AgentName: run.story.Agent}
	at := segment.at
	agentEvents := []acp.AgentEvent{}
	for index, step := range segment.steps {
		expanded, err := expandTranscriptStep(story, run.record.ID, turnID, index, step, &at)
		if err != nil {
			return nil, store.TokenUsage{}, err
		}
		agentEvents = append(agentEvents, expanded...)
	}
	steps := int64(len(segment.steps))
	input, output := steps*mainTurnInputPerStep, steps*mainTurnOutputPerStep
	cost := float64(steps) * mainTurnUSDPerStep
	total := input + output
	currency := "USD"
	done := acp.AgentEvent{
		Type: acp.EventTypeDone, SessionID: run.story.MainID, TurnID: turnID,
		StopReason: string(store.StopCompleted), Timestamp: at,
		Usage: &acp.TokenUsage{
			TurnID: turnID, InputTokens: &input, OutputTokens: &output, TotalTokens: &total,
			CostAmount: &cost, CostCurrency: &currency, Timestamp: at,
		},
	}
	agentEvents = append(agentEvents, done)
	events := make([]store.SessionEvent, 0, len(agentEvents))
	for index, event := range agentEvents {
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			return nil, store.TokenUsage{}, fmt.Errorf("demo seed: encode %s event %d: %w", turnID, index, err)
		}
		events = append(events, store.SessionEvent{
			SessionID: run.story.MainID, TurnID: turnID, Type: event.Type, AgentName: run.story.Agent,
			Content: content, Timestamp: event.Timestamp,
		})
	}
	return events, store.TokenUsage{
		TurnID: turnID, InputTokens: &input, OutputTokens: &output, TotalTokens: &total,
		CostAmount: &cost, CostCurrency: &currency, Timestamp: at,
	}, nil
}

func recordMainSession(
	ctx context.Context,
	dir string,
	run *instanceRun,
	events []store.SessionEvent,
	usage []store.TokenUsage,
) (persisted []store.SessionEvent, err error) {
	eventsDB, err := sessiondb.OpenSessionDB(ctx, store.SessionDBOwner{
		SessionID: run.story.MainID, WorkspaceID: run.record.ID,
	}, store.SessionDBFile(dir))
	if err != nil {
		return nil, fmt.Errorf("demo seed: open main session transcript for %q: %w", run.story.Agent, err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if closeErr := eventsDB.Close(closeCtx); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("demo seed: close main session transcript: %w", closeErr))
		}
	}()
	persisted, err = eventsDB.RecordPersistedBatch(ctx, events)
	if err != nil {
		return nil, fmt.Errorf("demo seed: write main session transcript for %q: %w", run.story.Agent, err)
	}
	for _, row := range usage {
		if err := eventsDB.RecordTokenUsage(ctx, row); err != nil {
			return nil, fmt.Errorf("demo seed: write main session usage for %q: %w", run.story.Agent, err)
		}
	}
	return persisted, nil
}
