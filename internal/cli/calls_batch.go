package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newCallBatchCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "batch <json-or-@file>",
		Short: "Create a bounded batch of agent calls",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := readCallBatch(args[0])
			if err != nil {
				return withCommandExitCode(2, err)
			}
			client, workspaceID, err := resolveCallClient(cmd, deps, workspace)
			if err != nil {
				return err
			}
			items, err := client.CreateCallBatch(cmd.Context(), workspaceID, contract.CreateCallRequest{
				Tasks: tasks, TasksPresent: true,
			})
			if err != nil {
				return withCallCommandExit(err)
			}
			return writeCommandOutput(cmd, callBatchBundle(items))
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Override workspace name or id; omit for global scope")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func readCallBatch(raw string) ([]contract.CreateCallItemRequest, error) {
	bytes := []byte(strings.TrimSpace(raw))
	if path, ok := strings.CutPrefix(strings.TrimSpace(raw), "@"); ok {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil, errors.New("cli: call batch @file path is required")
		}
		var err error
		bytes, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cli: read call batch file %q: %w", path, err)
		}
	}
	var tasks []contract.CreateCallItemRequest
	if err := json.Unmarshal(bytes, &tasks); err != nil {
		return nil, fmt.Errorf("cli: call batch must be a JSON array: %w", err)
	}
	if len(tasks) == 0 {
		return nil, errors.New("cli: call batch must contain at least one task")
	}
	return tasks, nil
}
