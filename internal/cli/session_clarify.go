package cli

import (
	"errors"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	sessionClarifyChoiceFlag = "choice"
	sessionClarifyTextFlag   = "text"
)

func newSessionClarifyCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clarify",
		Short: "Manage live session questions",
	}
	cmd.AddCommand(newSessionClarifyPendingCommand(deps))
	cmd.AddCommand(newSessionClarifyAnswerCommand(deps))
	return cmd
}

func newSessionClarifyPendingCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "pending <session-id>",
		Short:   "List live questions awaiting an operator answer",
		Args:    exactOneNonBlankArg(),
		Example: "  agh session clarify pending sess_123 -o json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListSessionClarifications(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionClarificationsBundle(response))
		},
	}
}

func newSessionClarifyAnswerCommand(deps commandDeps) *cobra.Command {
	var (
		choice int
		text   string
	)
	cmd := &cobra.Command{
		Use:   "answer <session-id> <request-id>",
		Short: "Answer one live session question",
		Args:  exactTwoNonBlankArgs(),
		Example: `  # Answer with the first offered choice
  agh session clarify answer sess_123 req_123 --choice 1

  # Answer a free-text question
  agh session clarify answer sess_123 req_123 --text "Use the staging workspace"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := sessionClarificationAnswerRequest(cmd, choice, text)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			answer, err := client.AnswerSessionClarification(
				cmd.Context(),
				strings.TrimSpace(args[0]),
				strings.TrimSpace(args[1]),
				request,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionClarificationAnswerBundle(answer))
		},
	}
	cmd.Flags().IntVar(&choice, sessionClarifyChoiceFlag, 0, "One-based choice number")
	cmd.Flags().StringVar(&text, sessionClarifyTextFlag, "", "Free-text answer")
	return cmd
}

func sessionClarificationAnswerRequest(
	cmd *cobra.Command,
	choice int,
	text string,
) (ClarificationAnswerRequest, error) {
	hasChoice := cmd.Flags().Changed(sessionClarifyChoiceFlag)
	hasText := cmd.Flags().Changed(sessionClarifyTextFlag)
	if hasChoice == hasText {
		return ClarificationAnswerRequest{}, errors.New("cli: exactly one of --choice or --text is required")
	}
	if hasChoice {
		if choice < 1 {
			return ClarificationAnswerRequest{}, errors.New("cli: --choice must be at least 1")
		}
		wireChoice := choice - 1
		return ClarificationAnswerRequest{ChoiceIndex: &wireChoice}, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ClarificationAnswerRequest{}, errors.New("cli: --text cannot be blank")
	}
	return ClarificationAnswerRequest{Text: text}, nil
}

func sessionClarificationsBundle(response ClarificationsRecord) outputBundle {
	return listBundle(
		response,
		response.Clarifications,
		"Pending Session Questions",
		[]string{"REQUEST ID", "AGENT", "QUESTION", "CHOICES", "DEADLINE"},
		"clarifications",
		[]string{"request_id", "agent_name", "question", "choices", "deadline"},
		func(item ClarificationPendingRecord) []string {
			return []string{
				item.RequestID,
				item.AgentName,
				item.Question,
				stringOrDash(strings.Join(item.Choices, " | ")),
				formatTime(item.Deadline),
			}
		},
		func(item ClarificationPendingRecord) []string {
			return []string{
				item.RequestID,
				item.AgentName,
				item.Question,
				strings.Join(item.Choices, " | "),
				formatTime(item.Deadline),
			}
		},
	)
}

func sessionClarificationAnswerBundle(answer ClarificationAnswerRecord) outputBundle {
	return outputBundle{
		jsonValue: answer,
		human: func() (string, error) {
			return renderHumanSection("Session Question Answered", []keyValue{
				{Label: "Choice", Value: clarificationHumanChoice(answer)},
				{Label: "Text", Value: stringOrDash(answer.Text)},
				{Label: "Fallback", Value: strconv.FormatBool(answer.Fallback)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"clarification_answer",
				[]string{"choice", sessionClarifyTextFlag, "fallback"},
				[]string{clarificationWireChoice(answer), answer.Text, strconv.FormatBool(answer.Fallback)},
			), nil
		},
	}
}

func clarificationHumanChoice(answer ClarificationAnswerRecord) string {
	if answer.Choice == nil {
		return "-"
	}
	return strconv.Itoa(*answer.Choice + 1)
}

func clarificationWireChoice(answer ClarificationAnswerRecord) string {
	if answer.Choice == nil {
		return ""
	}
	return strconv.Itoa(*answer.Choice)
}
