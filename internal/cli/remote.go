package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/api/contract"
)

const remoteCommandName = "remote"

func newRemoteCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: remoteCommandName, Short: "Manage access to this computer"}
	cmd.AddCommand(
		newRemoteLinkCommand(deps),
		remoteVerb(deps, "status", "Show remote access status", cobra.NoArgs, runRemoteStatus),
		remoteVerb(deps, "enable", "Turn on remote access", cobra.NoArgs, changeRemoteEnabled(true)),
		remoteVerb(deps, "disable", "Turn off remote access", cobra.NoArgs, changeRemoteEnabled(false)),
		remoteVerb(deps, "unlink", "Unlink this computer from the account", cobra.NoArgs, runRemoteUnlink),
		newRemoteDevicesCommand(deps),
		newRemoteCloudRouteCommand(deps),
	)
	return cmd
}

func remoteVerb(deps commandDeps, use, short string, arguments cobra.PositionalArgs,
	run func(*cobra.Command, remoteCommandClient, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short, Args: func(cmd *cobra.Command, args []string) error {
		if err := arguments(cmd, args); err != nil {
			return newCLIUsageError(err.Error())
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		client, err := remoteClientFromDeps(deps)
		if err == nil {
			err = run(cmd, client, args)
		}
		if err != nil {
			return accountExecutionError{err}
		}
		return nil
	}}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return newCLIUsageError(err.Error()) })
	return cmd
}

func newRemoteLinkCommand(deps commandDeps) *cobra.Command {
	var tokenStdin bool
	cmd := remoteVerb(deps, "link", "Link this computer to your account", cobra.NoArgs,
		func(cmd *cobra.Command, client remoteCommandClient, _ []string) error {
			var result contract.RemoteLinkResponse
			var err error
			if tokenStdin {
				result, err = linkRemoteWithToken(cmd, client)
			} else {
				result, err = linkRemoteInteractively(cmd, client)
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				return fmt.Sprintf("Linked as %s (%s). Remote access is on.", result.Name, result.ComputerID), nil
			}})
		})
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read a single-use link token from standard input")
	return cmd
}

func linkRemoteWithToken(cmd *cobra.Command, client remoteCommandClient) (contract.RemoteLinkResponse, error) {
	const maxToken = 4 << 10
	raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), maxToken+1))
	if err != nil {
		return contract.RemoteLinkResponse{}, err
	}
	token := strings.TrimSpace(string(raw))
	if token == "" || len(raw) > maxToken || strings.ContainsAny(token, " \t\r\n") {
		return contract.RemoteLinkResponse{}, errors.New("a single link token is required on standard input")
	}
	return client.LinkRemote(cmd.Context(), contract.RemoteLinkRequest{LinkToken: token})
}

func linkRemoteInteractively(
	cmd *cobra.Command, client remoteCommandClient,
) (result contract.RemoteLinkResponse, retErr error) {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	stream, err := client.OpenAccountSignIn(ctx)
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, stream.Close()) }()
	result, err = client.LinkRemote(ctx, contract.RemoteLinkRequest{})
	if err != nil || result.AttemptID == "" {
		return result, err
	}
	attemptID := result.AttemptID
	complete := false
	defer func() {
		if !complete {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			retErr = errors.Join(retErr, client.CancelAccountLogin(cleanup, attemptID))
		}
	}()
	if err := printRemoteActivation(cmd, result); err != nil {
		return result, err
	}
	if _, err := awaitAccountLogin(ctx, stream, attemptID); err != nil {
		return result, err
	}
	complete = true
	return client.LinkRemote(ctx, contract.RemoteLinkRequest{})
}

func printRemoteActivation(cmd *cobra.Command, attempt contract.RemoteLinkResponse) error {
	if !validAccountBrowserURL(attempt.VerificationURI) {
		return errors.New("invalid activation URL")
	}
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	writer := cmd.OutOrStdout()
	if mode != OutputHuman {
		writer = cmd.ErrOrStderr()
	}
	_, err = fmt.Fprintf(writer, "Open %s and enter code %s\n", attempt.VerificationURI, attempt.UserCode)
	return err
}

func runRemoteStatus(cmd *cobra.Command, client remoteCommandClient, _ []string) error {
	status, err := client.RemoteStatus(cmd.Context())
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{jsonValue: status, human: func() (string, error) {
		if status.ComputerID == "" {
			return "This computer is not linked. Run compozy remote link.", nil
		}
		if !status.Enabled {
			return fmt.Sprintf("Remote access is off. Linked as %s.", status.ComputerID), nil
		}
		return fmt.Sprintf("Remote access is on. Linked as %s.\nListener   %s\nControl    %s",
			status.ComputerID, status.Listener, status.ControlLink), nil
	}})
}

func changeRemoteEnabled(enabled bool) func(*cobra.Command, remoteCommandClient, []string) error {
	return func(cmd *cobra.Command, client remoteCommandClient, _ []string) error {
		status, err := client.PatchRemoteStatus(cmd.Context(), contract.RemotePatchRequest{Enabled: new(enabled)})
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, outputBundle{jsonValue: status, human: func() (string, error) {
			if status.Enabled {
				return "Remote access is on.", nil
			}
			return "Remote access is off.", nil
		}})
	}
}

func runRemoteUnlink(cmd *cobra.Command, client remoteCommandClient, _ []string) error {
	if err := client.UnlinkRemote(cmd.Context()); err != nil {
		return err
	}
	return writeCommandOutput(
		cmd,
		outputBundle{jsonValue: map[string]bool{"unlinked": true}, human: func() (string, error) {
			return "This computer was unlinked.", nil
		}},
	)
}
