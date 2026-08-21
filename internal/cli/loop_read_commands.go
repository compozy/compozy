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

type loopFollowPayloadError struct {
	cause error
}

func (e *loopFollowPayloadError) Error() string {
	return e.cause.Error()
}

func (e *loopFollowPayloadError) Unwrap() error {
	return e.cause
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
			client, workspaceID, err := loopReadClient(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.GetLoopRunBriefing(
				cmd.Context(), workspaceID, strings.TrimSpace(args[0]),
			)
			if err != nil {
				return err
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
			lines := []string{fmt.Sprintf(
				"%s · round %d — %s",
				label,
				response.Progress.Round,
				response.Headline,
			)}
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
			if (cmd.Flags().Changed("limit") && limit < 1) || limit > 500 {
				return withCommandExitCode(2, errors.New("--limit must be between 1 and 500"))
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
				return err
			}
			if err := writeCommandOutput(cmd, loopEventsOutputBundle(page, entries)); err != nil {
				return err
			}
			if !follow {
				return nil
			}
			briefing, err := client.GetLoopRunBriefing(cmd.Context(), workspaceID, runID)
			if err != nil {
				return err
			}
			if terminalLoopStatus(string(briefing.Status)) {
				return nil
			}
			return followLoopEvents(cmd, client, workspaceID, runID, view, page.HeadSeq)
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
		if errors.Is(streamErr, errStopSSE) {
			return nil
		}
		var payloadErr *loopFollowPayloadError
		if errors.As(streamErr, &payloadErr) {
			return payloadErr
		}
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		page, entries, err := loadLoopTimeline(cmd, client, workspaceID, runID, view, lastSequence, 500)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Seq <= lastSequence {
				continue
			}
			if err := writeFollowTimelineEntry(cmd, entry); err != nil {
				return err
			}
			lastSequence = entry.Seq
		}
		briefing, err := client.GetLoopRunBriefing(cmd.Context(), workspaceID, runID)
		if err != nil {
			return err
		}
		if terminalLoopStatus(string(briefing.Status)) {
			return nil
		}
		if page.HeadSeq > lastSequence {
			lastSequence = page.HeadSeq
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

func streamLoopEventsOnce(
	cmd *cobra.Command,
	client loopRunReadClient,
	workspaceID, runID, view string,
	lastSequence *int64,
) error {
	var callbackErr error
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
				if err := writeFollowTimelineEntry(cmd, *entry); err != nil {
					return err
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
	case promptDoneEventName,
		"no-op",
		"blocked",
		string(bootstrapPhaseFailed),
		"exhausted",
		"stalled",
		updateOutcomeCanceled:
		return true
	default:
		return false
	}
}
