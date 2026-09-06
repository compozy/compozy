package cli

import (
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/spf13/cobra"
)

const sessionRoleKey = "role"

func newSessionSearchCommand(deps commandDeps) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use: "search <id> <query>", Short: "Search the full retained session transcript",
		Example: "  compozy session search sess_1234 lifecycle --limit 50 -o json",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := (transcript.SearchQuery{Query: args[1], Limit: limit}).Normalize()
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.SearchSessionTranscript(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionSearchBundle(result))
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum matches (1 to 1000); omitted matches set truncated")
	return cmd
}

func newSessionOutlineCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "outline <id>", Short: "List sent messages and reply previews from retained session history",
		Example: "  compozy session outline sess_1234 -o json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.GetSessionOutline(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionOutlineBundle(result))
		},
	}
}

func sessionSearchBundle(result contract.SessionTranscriptSearchResponse) outputBundle {
	row := func(match transcript.SearchMatch) []string {
		return []string{strconv.FormatInt(match.Sequence, 10), match.TurnID, match.Role, match.Snippet}
	}
	title := "Session Search"
	if result.Truncated {
		title += " (more matches omitted; increase --limit)"
	}
	return listBundle(
		result,
		result.Matches,
		title,
		[]string{sessionSequenceLabel, sessionTurnLabel, "Role", "Snippet"},
		"matches",
		[]string{sessionSequenceKey, sessionTurnIDKey, sessionRoleKey, "snippet"},
		row,
		row,
	)
}

func sessionOutlineBundle(result contract.SessionTranscriptOutlineResponse) outputBundle {
	row := func(entry transcript.OutlineEntry) []string {
		return []string{
			strconv.FormatInt(entry.Sequence, 10),
			entry.TurnID,
			formatTime(entry.At),
			entry.Preview,
			entry.ReplyPreview,
		}
	}
	return listBundle(
		result,
		result.Entries,
		"Session Outline",
		[]string{sessionSequenceLabel, sessionTurnLabel, "At", "Message", "Reply"},
		"entries",
		[]string{sessionSequenceKey, sessionTurnIDKey, "at", "preview", "reply_preview"},
		row,
		row,
	)
}
