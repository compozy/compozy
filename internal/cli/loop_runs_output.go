package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func loopStatusOutputBundle(response *contract.LoopRunResponse) outputBundle {
	bundle := loopOutputBundle(response, fmt.Sprintf("Loop run %s is %s", response.Run.ID, response.Run.Status))
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLine(cmd, response)
	}
	bundle.human = func() (string, error) {
		run := renderHumanSection("Loop run", []keyValue{
			{Label: "ID", Value: stringOrDash(response.Run.ID)},
			{Label: "Loop", Value: stringOrDash(response.Run.LoopName)},
			{Label: automationStatusValue, Value: stringOrDash(string(response.Run.Status))},
			{Label: "Completion", Value: stringOrDash(string(response.Run.CompletionState))},
			{Label: cliGenerationValue, Value: strconv.FormatInt(response.Run.Generation, 10)},
			{Label: "Best Generation", Value: formatOptionalInt64(response.Run.BestGeneration)},
			{Label: "Best Score", Value: formatOptionalFloat64(response.Run.BestScore)},
		})
		rows := make([][]string, 0, len(response.Generations))
		for _, generation := range response.Generations {
			rows = append(rows, []string{
				strconv.FormatInt(generation.Generation, 10),
				strconv.FormatInt(generation.ParentGeneration, 10),
				string(generation.Origin),
				formatLoopVerdicts(generation.Verdicts),
				strconv.Itoa(len(generation.Outputs)),
			})
		}
		generations := renderHumanTable(
			"Generations",
			[]string{"GENERATION", "PARENT", "ORIGIN", "VERDICTS", "OUTPUTS"},
			rows,
		)
		return renderHumanBlocks(run, generations), nil
	}
	return bundle
}

func loopRunsOutputBundle(response contract.LoopRunsResponse) outputBundle {
	items := loopRunCLIJSONItems(response.Runs)
	jsonResponse := struct {
		Items      []loopRunCLIJSONItem `json:"items"`
		NextCursor string               `json:"next_cursor"`
	}{Items: items, NextCursor: response.NextCursor}
	bundle := listBundle(
		response,
		response.Runs,
		"Loop runs",
		[]string{cliStatusHeader, taskLoopColumn, "PROGRESS", "STARTED", loopDurationHeader},
		"loop_runs",
		[]string{
			"id",
			"loop_name",
			automationStatusKey,
			configAttentionKey,
			"progress",
			"completion_state",
			loopGenerationKey,
			"best_generation",
			"best_score",
		},
		loopRunSummaryRow,
		loopRunSummaryTOONRow,
	)
	bundle.jsonValue = jsonResponse
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, items)
	}
	bundle.human = func() (string, error) {
		rows := make([][]string, 0, len(response.Runs))
		for _, run := range response.Runs {
			rows = append(rows, loopRunSummaryRow(run))
		}
		return renderLoopReadTable(
			[]string{cliStatusHeader, taskLoopColumn, "PROGRESS", "STARTED", loopDurationHeader},
			rows,
		), nil
	}
	return bundle
}

type loopRunCLIJSONItem struct {
	payload contract.LoopRunPayload
}

func loopRunCLIJSONItems(runs []contract.LoopRunPayload) []loopRunCLIJSONItem {
	items := make([]loopRunCLIJSONItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, loopRunCLIJSONItem{payload: run})
	}
	return items
}

func (item loopRunCLIJSONItem) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(item.payload)
	if err != nil {
		return nil, fmt.Errorf("encode Loop run payload: %w", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode Loop run payload: %w", err)
	}
	delete(fields, "id")
	fields["run_id"] = item.payload.ID
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode Loop runs CLI item: %w", err)
	}
	return encoded, nil
}

func loopRunSummaryRow(run contract.LoopRunPayload) []string {
	status := string(run.Status)
	if run.Attention != nil {
		status = loopNeedsYouLabel
	}
	progress := "—"
	if run.Progress.StepsTotal > 0 && !terminalLoopStatus(string(run.Status)) {
		progress = fmt.Sprintf(
			"step %d/%d · r%d",
			run.Progress.StepsDone,
			run.Progress.StepsTotal,
			run.Progress.Round,
		)
	}
	duration := "—"
	if !run.StartedAt.IsZero() && !run.LastProgressAt.Before(run.StartedAt) {
		duration = formatLoopReadDuration(run.LastProgressAt.Sub(run.StartedAt))
	}
	if attention := loopRunAttentionText(run.Attention); attention != "—" {
		duration += "   " + attention
	}
	return []string{
		status,
		stringOrDash(run.LoopName),
		progress,
		run.StartedAt.Local().Format("15:04"),
		duration,
	}
}

func formatLoopReadDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)
	if duration%time.Minute == 0 {
		return fmt.Sprintf("%dm", int64(duration/time.Minute))
	}
	return duration.String()
}

func loopRunSummaryTOONRow(run contract.LoopRunPayload) []string {
	return []string{
		stringOrDash(run.ID),
		stringOrDash(run.LoopName),
		stringOrDash(string(run.Status)),
		loopRunAttentionText(run.Attention),
		fmt.Sprintf("%d/%d", run.Progress.StepsDone, run.Progress.StepsTotal),
		stringOrDash(string(run.CompletionState)),
		strconv.FormatInt(run.Generation, 10),
		formatOptionalInt64(run.BestGeneration),
		formatOptionalFloat64(run.BestScore),
	}
}

func loopRunAttentionText(attention *contract.LoopRunAttention) string {
	if attention == nil {
		return "—"
	}
	return fmt.Sprintf("%s (%d)", attention.Kind, attention.Count)
}

func formatLoopVerdicts(verdicts []contract.LoopGateVerdictPayload) string {
	if len(verdicts) == 0 {
		return "--"
	}
	items := make([]string, 0, len(verdicts))
	for _, verdict := range verdicts {
		value := fmt.Sprintf("%s=%s", verdict.GateID, verdict.Outcome)
		if verdict.Score != nil {
			value += "@" + strconv.FormatFloat(*verdict.Score, 'f', -1, 64)
		}
		items = append(items, value)
	}
	return strings.Join(items, ", ")
}

func formatOptionalInt64(value *int64) string {
	if value == nil {
		return "--"
	}
	return strconv.FormatInt(*value, 10)
}

func formatOptionalFloat64(value *float64) string {
	if value == nil {
		return "--"
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
