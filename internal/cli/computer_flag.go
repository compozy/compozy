package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	computerFlagName = "computer"
	localComputer    = "local"
)

func configureRootComputerFlag(root *cobra.Command, deps *commandDeps) {
	flags := root.PersistentFlags()
	value := flags.String(computerFlagName, "", "Target a computer by name or ID")
	deps.computerTarget = func() (string, bool) {
		return strings.TrimSpace(*value), flags.Changed(computerFlagName)
	}
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateComputerFlag(cmd)
	}
}

func validateComputerFlag(cmd *cobra.Command) error {
	if !cmd.Flags().Changed(computerFlagName) {
		return nil
	}
	group := cmd
	for group.Parent() != nil && group.Parent() != cmd.Root() {
		group = group.Parent()
	}
	switch group.Name() {
	case "daemon", "service", accountCommandName, computerFlagName, remoteCommandName, "update", "install", "uninstall":
		verb := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
		return &computerFlagError{message: fmt.Sprintf("--computer doesn't apply to compozy %s.", verb)}
	}
	if value, err := cmd.Flags().GetString(computerFlagName); err == nil && strings.TrimSpace(value) == "" {
		return &computerFlagError{message: "--computer requires a name or ID."}
	}
	return nil
}

type computerFlagError struct{ message string }

func (e *computerFlagError) Error() string    { return e.message }
func (e *computerFlagError) cliExitCode() int { return 2 }
func (e *computerFlagError) errorPayload() contract.ErrorPayload {
	return contract.ErrorPayload{Code: "flag_not_applicable", Error: e.message}
}

func bridgeClientTarget(ctx context.Context, deps commandDeps, socketPath, selection string) (ClientTarget, error) {
	target := LocalClientTarget(socketPath)
	if selection == localComputer {
		return target, nil
	}
	if selection == "" {
		return ClientTarget{}, &computerFlagError{message: "--computer requires a name or ID."}
	}
	client, err := deps.newClient(target)
	if err != nil {
		return ClientTarget{}, err
	}
	registry, ok := client.(computerListClient)
	if !ok {
		return ClientTarget{}, errors.New("daemon client does not support selecting a computer")
	}
	item, err := resolveComputer(ctx, registry, selection)
	if err != nil {
		return ClientTarget{}, err
	}
	target.computerID = string(item.ID)
	return target, nil
}

func resolveComputer(ctx context.Context, client computerListClient, selection string) (contract.Computer, error) {
	list, err := client.ListComputers(ctx)
	if err != nil {
		return contract.Computer{}, err
	}
	if idx := slices.IndexFunc(list.Computers, func(item contract.Computer) bool {
		return item.ID == remoteproto.ComputerID(selection)
	}); idx >= 0 {
		return list.Computers[idx], nil
	}
	var found *contract.Computer
	for _, item := range list.Computers {
		if item.Name != selection {
			continue
		}
		if found != nil {
			return contract.Computer{}, &daemonAPIError{statusCode: http.StatusConflict, payload: contract.ErrorPayload{
				Code: "computer_ambiguous", Error: "More than one computer has this name. Use its ID.",
			}}
		}
		found = new(item)
	}
	if found != nil {
		return *found, nil
	}
	return contract.Computer{}, &daemonAPIError{statusCode: http.StatusNotFound, payload: contract.ErrorPayload{
		Code: "computer_not_found", Error: "Computer not found.",
	}}
}
