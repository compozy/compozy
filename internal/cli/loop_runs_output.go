package cli

import (
	"fmt"
	"strconv"
	"strings"

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
	bundle := listBundle(
		response,
		response.Runs,
		"Loop runs",
		[]string{"ID", "PROFILE", "LOOP", "STATUS", "COMPLETION", "GENERATION", "BEST GEN", "BEST SCORE"},
		"loop_runs",
		[]string{
			"id",
			"profile_name",
			"loop_name",
			automationStatusKey,
			"completion_state",
			loopGenerationKey,
			"best_generation",
			"best_score",
		},
		loopRunSummaryRow,
		loopRunSummaryRow,
	)
	baseHuman := bundle.human
	bundle.human = func() (string, error) {
		runs, err := baseHuman()
		if err != nil {
			return "", err
		}
		aggregates := renderHumanSection("Aggregates", []keyValue{
			{Label: "Total", Value: strconv.Itoa(response.Aggregates.Total)},
			{Label: cliLiveValue, Value: strconv.Itoa(response.Aggregates.Live)},
			{Label: "Terminal", Value: strconv.Itoa(response.Aggregates.Terminal)},
			{Label: "Succeeded", Value: strconv.Itoa(response.Aggregates.Succeeded)},
			{Label: "Failed", Value: strconv.Itoa(response.Aggregates.Failed)},
		})
		return renderHumanBlocks(runs, aggregates), nil
	}
	return bundle
}

func loopRunSummaryRow(run contract.LoopRunPayload) []string {
	return []string{
		stringOrDash(run.ID),
		stringOrDash(run.ProfileName),
		stringOrDash(run.LoopName),
		stringOrDash(string(run.Status)),
		stringOrDash(string(run.CompletionState)),
		strconv.FormatInt(run.Generation, 10),
		formatOptionalInt64(run.BestGeneration),
		formatOptionalFloat64(run.BestScore),
	}
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
