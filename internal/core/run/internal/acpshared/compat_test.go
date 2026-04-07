package acpshared

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/run/transcript"
)

func newSessionUpdateHandler(
	ctx context.Context,
	index int,
	agentID string,
	sessionID string,
	logger *slog.Logger,
	runID string,
	outWriter io.Writer,
	errWriter io.Writer,
	runJournal *journal.Journal,
	jobUsage *model.Usage,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	activity *activityMonitor,
) *SessionUpdateHandler {
	return NewSessionUpdateHandler(
		ctx,
		index,
		agentID,
		sessionID,
		logger,
		runID,
		outWriter,
		errWriter,
		runJournal,
		jobUsage,
		aggregateUsage,
		aggregateMu,
		activity,
	)
}

const transcriptEntryAssistantMessage = transcript.EntryKindAssistantMessage
