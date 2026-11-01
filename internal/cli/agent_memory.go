package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newAgentMemoryCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "memory", Short: "Inspect and manage agent memory (experimental)"}
	for _, operation := range []string{"inspect", "add", "replace", "remove", "write", "validate", "search", "history", "rollback"} {
		cmd.AddCommand(newAgentMemoryOperation(deps, operation))
	}
	return cmd
}

func newAgentMemoryOperation(deps commandDeps, operation string) *cobra.Command {
	var options agentMemoryOptions
	count := 1
	use := operation + " <agent>"
	switch operation {
	case "add":
		count = 2
		use += " <text>"
	case "replace":
		count = 3
		use += " <match> <text>"
	case "remove":
		count = 2
		use += " <match>"
	case "search":
		count = 2
		use += " <query>"
	case "rollback":
		count = 2
		use += " <revision>"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: strings.ToUpper(operation[:1]) + operation[1:] + " agent memory",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(count)(cmd, args); err != nil {
				return err
			}
			for _, arg := range args {
				if strings.TrimSpace(arg) == "" {
					return errors.New("memory arguments cannot be blank")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query, err := agentQueryFromCommand(cmd, deps, client)
			if err != nil {
				return err
			}
			body, err := options.requestBody(cmd, client, query, operation, args)
			if err != nil {
				return err
			}
			result, err := client.AgentMemory(cmd.Context(), args[0], query, operation, body)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentMemoryBundle(operation, result))
		},
	}
	addWorkspaceFlag(cmd)
	switch operation {
	case "add":
		cmd.Flags().StringVar(&options.until, "until", "", "Optional expiry date (YYYY-MM-DD)")
	case "search":
		cmd.Flags().
			BoolVar(&options.includeSessions, "include-sessions", false, "Include accessible session transcripts")
	case "write", "validate":
		addAuthoredBodyFlags(cmd, &options.input)
	}
	if operation == "write" {
		cmd.Flags().StringVar(&options.expectedDigest, "expected-digest", "", "Digest returned by inspect")
	}
	return cmd
}

func agentMemoryBundle(operation string, raw json.RawMessage) outputBundle {
	human := func() (string, error) {
		switch operation {
		case "inspect":
			var doc contract.AgentMemoryDocument
			if err := json.Unmarshal(raw, &doc); err != nil {
				return "", err
			}
			return fmt.Sprintf(
				"%s\nBudget: %d of %d lines · %.1f of %.0f KiB\nDigest: %s",
				strings.TrimSuffix(doc.Content, "\n"),
				doc.Budget.Lines,
				doc.Budget.MaxLines,
				float64(doc.Budget.Bytes)/1024,
				float64(doc.Budget.MaxBytes)/1024,
				doc.Digest,
			), nil
		case "search":
			var result struct {
				Results []contract.AgentMemorySearchResult `json:"results"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return "", err
			}
			rows := make([]string, 0, len(result.Results))
			for _, item := range result.Results {
				rows = append(rows, fmt.Sprintf("%s:%d %s", item.File, item.Line, item.Text))
			}
			return strings.Join(rows, "\n"), nil
		case "history":
			var result struct {
				Revisions []contract.AgentMemoryRevision `json:"revisions"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return "", err
			}
			rows := make([]string, 0, len(result.Revisions))
			for _, item := range result.Revisions {
				rows = append(rows, item.ID)
			}
			return strings.Join(rows, "\n"), nil
		case "validate":
			return string(raw), nil
		default:
			return "Saved agent memory.", nil
		}
	}
	return outputBundle{jsonValue: raw, human: human, toon: human}
}

type agentMemoryOptions struct {
	input           authoredBodyInput
	until           string
	expectedDigest  string
	includeSessions bool
}

func (options agentMemoryOptions) requestBody(
	cmd *cobra.Command,
	client DaemonClient,
	query AgentQuery,
	operation string,
	args []string,
) (any, error) {
	var body any
	switch operation {
	case "add":
		body = contract.AgentMemoryAddRequest{Text: args[1], Until: options.until}
	case "replace":
		body = contract.AgentMemoryReplaceRequest{Match: args[1], Text: args[2]}
	case "remove":
		body = contract.AgentMemoryRemoveRequest{Match: args[1]}
	case "search":
		body = contract.AgentMemorySearchRequest{Query: args[1], IncludeSessions: options.includeSessions}
	case "rollback":
		body = contract.AgentMemoryRollbackRequest{Revision: args[1]}
	case "write", "validate":
		content, err := readAuthoredBody(cmd, options.input, operation == "write")
		if err != nil {
			return nil, err
		}
		if operation == "write" {
			if !cmd.Flags().Changed("expected-digest") {
				return nil, errors.New("--expected-digest is required; inspect memory before writing")
			}
			body = contract.AgentMemoryWriteRequest{Content: content, ExpectedDigest: options.expectedDigest}
		} else {
			if !cmd.Flags().Changed("file") && !options.input.stdin {
				current, err := client.AgentMemory(cmd.Context(), args[0], query, "inspect", nil)
				if err != nil {
					return nil, err
				}
				var doc contract.AgentMemoryDocument
				if err := json.Unmarshal(current, &doc); err != nil {
					return nil, err
				}
				content = doc.Content
			}
			body = contract.AgentMemoryValidateRequest{Content: content}
		}
	}
	return body, nil
}
