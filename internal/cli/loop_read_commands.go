package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/spf13/cobra"
)

const (
	loopRunNotFoundCode = "loop_run_not_found"
	loopRoundLabel      = "round"
)

type loopFollowPayloadError struct {
	cause error
}

func (e *loopFollowPayloadError) Error() string {
	return e.cause.Error()
}

func (e *loopFollowPayloadError) Unwrap() error {
	return e.cause
}

func normalizeLoopReadError(runID string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr *daemonAPIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch strings.TrimSpace(apiErr.payload.Code) {
	case loopRunNotFoundCode:
		return withCommandExitCode(1, fmt.Errorf("loop run %q not found", strings.TrimSpace(runID)))
	case looppkg.ErrTimelinePositionBeyondHead.Error():
		return withCommandExitCode(1, errors.New(strings.TrimSpace(apiErr.payload.Error)))
	default:
		return err
	}
}

func loopReadClient(cmd *cobra.Command, deps commandDeps, workspaceRef string) (loopRunReadClient, string, error) {
	client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
	if err != nil {
		return nil, "", err
	}
	reads, ok := client.(loopRunReadClient)
	if !ok {
		return nil, "", errors.New("cli: daemon client does not support loop run reads")
	}
	return reads, workspaceID, nil
}

func newLoopWhyCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use:   "why <run>",
		Short: "Explain one Loop run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := strings.TrimSpace(args[0])
			client, workspaceID, err := loopReadClient(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.GetLoopRunBriefing(
				cmd.Context(), workspaceID, runID,
			)
			if err != nil {
				return normalizeLoopReadError(runID, err)
			}
			return writeCommandOutput(cmd, loopWhyOutputBundle(response))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	return cmd
}

func loopWhyOutputBundle(response contract.LoopBriefingResponse) outputBundle {
	return outputBundle{
		jsonValue: response,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, response)
		},
		human: func() (string, error) {
			label := strings.ToUpper(strings.ReplaceAll(string(response.Status), "-", " "))
			if response.Tone == looppkg.BriefingToneNeedsYou {
				label = loopNeedsYouLabel
			}
			lines := loopWhyHeadline(response, label)
			if response.Detail != "" {
				lines = append(lines, response.Detail)
			}
			if len(response.Blockers) == 0 && !terminalLoopStatus(string(response.Status)) {
				lines = append(lines, fmt.Sprintf(
					"Nothing needs you. %d of %d steps done.",
					response.Progress.StepsDone,
					response.Progress.StepsTotal,
				))
			}
			for _, blocker := range response.Blockers {
				if blocker.Unblocker != "" {
					lines = append(lines, "Unblock: "+blocker.Unblocker)
				}
			}
			if !terminalLoopStatus(string(response.Status)) {
				lines = append(lines, fmt.Sprintf(
					"Watch: compozy loop events %s --follow",
					response.RunID,
				))
			}
			return strings.Join(lines, "\n"), nil
		},
		toon: func() (string, error) {
			raw, err := json.Marshal(response)
			if err != nil {
				return "", fmt.Errorf("cli: encode briefing toon: %w", err)
			}
			return string(raw), nil
		},
	}
}

func loopWhyHeadline(response contract.LoopBriefingResponse, label string) []string {
	if response.Outcome == nil {
		return []string{fmt.Sprintf("%s · round %d — %s", label, response.Progress.Round, response.Headline)}
	}
	rounds := "rounds"
	if response.Progress.Round == 1 {
		rounds = loopRoundLabel
	}
	lines := []string{fmt.Sprintf(
		"%s · finished %s after %d %s (%s)",
		label,
		response.Outcome.At.Local().Format("2006-01-02 15:04"),
		response.Progress.Round,
		rounds,
		stringOrDash(response.Usage.Duration),
	)}
	lines = append(lines, fmt.Sprintf(
		"Spent %s tokens · $%.2f · %.0f%% of budget",
		formatLoopTokenCount(response.Usage.Tokens),
		response.Usage.CostUSD,
		response.Usage.BudgetUsedPct,
	))
	for _, artifact := range response.Artifacts {
		produced := artifact.Name
		if artifact.Output != "" {
			produced += fmt.Sprintf(" (output %q)", artifact.Output)
		}
		lines = append(lines, "Produced: "+produced)
	}
	return lines
}

func formatLoopTokenCount(tokens int64) string {
	if tokens < 1000 {
		return strconv.FormatInt(tokens, 10)
	}
	return strconv.FormatInt(tokens/1000, 10) + "k"
}

func newLoopEventsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, view string
	var after int64
	var limit int
	var follow bool
	cmd := &cobra.Command{
		Use:   "events <run>",
		Short: "Read one Loop run timeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if after < 0 {
				return withCommandExitCode(2, errors.New("--after must be nonnegative"))
			}
			if err := validateLoopPageLimit(
				limit,
				cmd.Flags().Changed("limit"),
				loopRunReadPageLimitMax,
			); err != nil {
				return err
			}
			if view != string(looppkg.TimelineViewNotable) && view != string(looppkg.TimelineViewAll) {
				return withCommandExitCode(2, errors.New("--view must be notable or all"))
			}
			client, workspaceID, err := loopReadClient(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			runID := strings.TrimSpace(args[0])
			page, entries, err := loadLoopTimeline(cmd, client, workspaceID, runID, view, after, limit)
			if err != nil {
				return normalizeLoopReadError(runID, err)
			}
			if err := writeCommandOutput(cmd, loopEventsOutputBundle(page, entries)); err != nil {
				return err
			}
			if !follow {
				return nil
			}
			briefing, err := client.GetLoopRunBriefing(cmd.Context(), workspaceID, runID)
			if err != nil {
				return normalizeLoopReadError(runID, err)
			}
			if terminalLoopStatus(string(briefing.Status)) {
				_, err = drainLoopTimeline(cmd, client, workspaceID, runID, view, page.HeadSeq)
				return normalizeLoopReadError(runID, err)
			}
			return normalizeLoopReadError(
				runID,
				followLoopEvents(cmd, client, workspaceID, runID, view, page.HeadSeq),
			)
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().Int64Var(&after, "after", 0, "Resume after this run sequence")
	cmd.Flags().IntVar(&limit, "limit", 0, "Page size from 1 to 500; defaults to 50")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow new events until the run is terminal")
	cmd.Flags().StringVar(&view, "view", string(looppkg.TimelineViewNotable), "View: notable or all")
	return cmd
}

func loopEventsOutputBundle(page contract.LoopTimelineResponse, entries []looppkg.TimelineEntry) outputBundle {
	copyPage := page
	copyPage.Entries = entries
	bundle := listBundle(
		copyPage,
		entries,
		"Loop events",
		[]string{"SEQ", loopRoundHeader, "EVENT"},
		"loop_events",
		[]string{"seq", resourceKindKey, "generation", loopNodeIDJSONKey, networkTitleKey, "at"},
		loopEventHumanRow,
		loopEventTOONRow,
	)
	bundle.human = func() (string, error) {
		rows := make([][]string, 0, len(entries))
		for _, entry := range entries {
			rows = append(rows, loopEventHumanRow(entry))
		}
		return renderLoopReadTable([]string{"SEQ", loopRoundHeader, "EVENT"}, rows), nil
	}
	return bundle
}

func loopEventHumanRow(entry looppkg.TimelineEntry) []string {
	round := "—"
	if entry.Generation > 0 {
		round = "g" + strconv.FormatInt(entry.Generation, 10)
	}
	return []string{
		strconv.FormatInt(entry.Seq, 10),
		round,
		entry.Title,
	}
}

func loopEventTOONRow(entry looppkg.TimelineEntry) []string {
	return []string{
		strconv.FormatInt(entry.Seq, 10),
		string(entry.Kind),
		strconv.FormatInt(entry.Generation, 10),
		string(entry.NodeID),
		entry.Title,
		entry.At.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func loadLoopTimeline(
	cmd *cobra.Command,
	client loopRunReadClient,
	workspaceID, runID, view string,
	after int64,
	limit int,
) (contract.LoopTimelineResponse, []looppkg.TimelineEntry, error) {
	query := LoopTimelineQuery{View: view, After: after, Limit: limit}
	page, err := client.GetLoopRunTimeline(cmd.Context(), workspaceID, runID, query)
	if err != nil {
		return contract.LoopTimelineResponse{}, nil, err
	}
	entries := append([]looppkg.TimelineEntry(nil), page.Entries...)
	if after > 0 {
		for page.NextCursor != "" {
			query.Cursor = page.NextCursor
			page, err = client.GetLoopRunTimeline(cmd.Context(), workspaceID, runID, query)
			if err != nil {
				return contract.LoopTimelineResponse{}, nil, err
			}
			entries = append(entries, page.Entries...)
		}
	}
	sortTimelineEntries(entries)
	page.Entries = entries
	page.NextCursor = ""
	return page, entries, nil
}

func sortTimelineEntries(entries []looppkg.TimelineEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Seq < entries[j].Seq
	})
}

func followLoopEvents(
	cmd *cobra.Command,
	client loopRunReadClient,
	workspaceID, runID, view string,
	after int64,
) error {
	lastSequence := after
	for {
		streamErr := streamLoopEventsOnce(cmd, client, workspaceID, runID, view, &lastSequence)
		terminalObserved := errors.Is(streamErr, errStopSSE)
		if payloadErr, ok := errors.AsType[*loopFollowPayloadError](streamErr); ok {
			return payloadErr
		}
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		if !terminalObserved {
			briefing, err := client.GetLoopRunBriefing(cmd.Context(), workspaceID, runID)
			if err != nil {
				return err
			}
			terminalObserved = terminalLoopStatus(string(briefing.Status))
		}
		var err error
		lastSequence, err = drainLoopTimeline(cmd, client, workspaceID, runID, view, lastSequence)
		if err != nil {
			return err
		}
		if terminalObserved {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-cmd.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
			return cmd.Context().Err()
		case <-timer.C:
		}
	}
}

func drainLoopTimeline(
	cmd *cobra.Command,
	client loopRunReadClient,
	workspaceID, runID, view string,
	after int64,
) (int64, error) {
	page, entries, err := loadLoopTimeline(cmd, client, workspaceID, runID, view, after, 500)
	if err != nil {
		return after, err
	}
	lastSequence := after
	for _, entry := range entries {
		if entry.Seq <= lastSequence {
			continue
		}
		if err := writeFollowTimelineEntry(cmd, entry); err != nil {
			return lastSequence, err
		}
		lastSequence = entry.Seq
	}
	if page.HeadSeq > lastSequence {
		lastSequence = page.HeadSeq
	}
	return lastSequence, nil
}

func streamLoopEventsOnce(
	cmd *cobra.Command,
	client loopRunReadClient,
	workspaceID, runID, view string,
	lastSequence *int64,
) error {
	var callbackErr error
	buffer := loopFollowTimelineBuffer{}
	streamErr := client.StreamLoopRunEvents(
		cmd.Context(),
		workspaceID,
		runID,
		*lastSequence,
		func(event SSEEvent) (err error) {
			defer func() {
				if err != nil && !errors.Is(err, errStopSSE) {
					callbackErr = err
				}
			}()
			var payload contract.LoopRunEventPayload
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return fmt.Errorf("cli: decode loop event: %w", err)
			}
			if payload.Seq <= *lastSequence {
				return nil
			}
			entry, err := looppkg.ProjectTimelineEvent(looppkg.RunEvent{
				ID:          payload.ID,
				LoopRunID:   looppkg.RunID(payload.LoopRunID),
				WorkspaceID: looppkg.WorkspaceID(payload.WorkspaceID),
				Seq:         payload.Seq,
				Kind:        string(payload.Kind),
				Payload:     payload.Payload,
				At:          payload.At,
			}, looppkg.TimelineView(view))
			if err != nil {
				return err
			}
			if entry != nil {
				for _, ready := range buffer.Push(*entry) {
					if err := writeFollowTimelineEntry(cmd, ready); err != nil {
						return err
					}
				}
			}
			*lastSequence = payload.Seq
			if payload.Kind == contract.LoopRunEventStatusChanged {
				var state struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal(payload.Payload, &state); err != nil {
					return fmt.Errorf("cli: decode loop terminal status: %w", err)
				}
				if terminalLoopStatus(state.Status) {
					return errStopSSE
				}
			}
			return nil
		},
	)
	if pending, ok := buffer.Flush(); ok && callbackErr == nil {
		if err := writeFollowTimelineEntry(cmd, pending); err != nil {
			callbackErr = err
		}
	}
	if callbackErr != nil {
		return &loopFollowPayloadError{cause: callbackErr}
	}
	return streamErr
}

func writeFollowTimelineEntry(cmd *cobra.Command, entry looppkg.TimelineEntry) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		return writeJSONLineWithoutWorkspaceResolution(cmd, entry)
	}
	return writeRawCommandOutput(cmd, strings.Join(loopEventHumanRow(entry), "\t"))
}

func terminalLoopStatus(status string) bool {
	switch status {
	case string(looppkg.StatusDone),
		string(looppkg.StatusNoOp),
		string(looppkg.StatusBlocked),
		string(looppkg.StatusFailed),
		string(looppkg.StatusExhausted),
		string(looppkg.StatusStalled),
		string(looppkg.StatusCanceled):
		return true
	default:
		return false
	}
}
