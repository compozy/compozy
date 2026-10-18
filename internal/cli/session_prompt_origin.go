package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/spf13/cobra"
)

const (
	// fromSessionAuto sends as the managed session the CLI runs in (its daemon-issued environment).
	fromSessionAuto = "auto"
	// fromSessionNone sends as operator input; it is also the default this release.
	fromSessionNone = "none"
)

// fromSessionHint tells a CLI caller running inside a managed session how to send as that session. Stamping
// is opt-in this release so operator semantics (interrupt, slash commands, human priority) do not change
// silently for existing scripts.
const fromSessionHint = "Hint: this prompt is sent as operator input. Pass --from-session auto to send it as " +
	"this session (%s) so the receiver sees the sender and can reply; that becomes the default in a later release."

type sessionPromptAsClient interface {
	SendSessionPromptAs(
		context.Context,
		string,
		SessionPromptRequest,
		agentidentity.Credentials,
	) (SessionPromptRecord, error)
}

type sessionPromptStreamAsClient interface {
	StreamPromptSessionAs(
		context.Context,
		string,
		SessionPromptRequest,
		agentidentity.Credentials,
		SSEHandler,
	) error
}

var (
	errPromptReplyRequiresSession = errors.New(
		"cli: --in-reply-to requires a calling agent session (--from-session auto or --from-session <id>)",
	)
	errPromptSupersedeRequiresSession = errors.New(
		"cli: --supersede requires a calling agent session (--from-session auto or --from-session <id>)",
	)
	errFromSessionAutoOutsideSession = errors.New(
		"cli: --from-session auto requires a managed session (" + agentidentity.EnvSessionID + " is not set)",
	)
)

func bindSessionPromptOriginFlags(cmd *cobra.Command, flags *sessionPromptCommandFlags) {
	cmd.Flags().StringVar(
		&flags.fromSession,
		"from-session",
		"",
		"Send as an agent session so the receiver sees who wrote it and can reply: a session id, or "+
			"\"auto\" for the managed session this runs in ("+agentidentity.EnvSessionID+"). "+
			"Omitted or \"none\" sends as operator input",
	)
	cmd.Flags().StringVar(
		&flags.inReplyTo,
		"in-reply-to",
		"",
		"Message id this prompt answers; requires a calling agent session",
	)
	cmd.Flags().BoolVar(
		&flags.supersede,
		"supersede",
		false,
		"Replace your own older queued prompt to the same session; requires a calling agent session",
	)
}

// originCredentials resolves which agent session, if any, the prompt is sent as. Stamping is opt-in this
// release: only --from-session auto (the daemon-issued environment) or --from-session <id> (validated by the
// daemon like every agent identity) sends as a session. The daemon trusts the local session id it receives;
// identity is not protected against a local operator who names another session. Without the flag a caller
// inside a managed session gets a one-line stderr hint and the prompt stays operator input.
func (flags *sessionPromptCommandFlags) originCredentials(
	cmd *cobra.Command,
	deps commandDeps,
	client any,
) (agentidentity.Credentials, error) {
	fromSession := strings.TrimSpace(flags.fromSession)
	env := agentCredentialsFromEnv(deps)
	switch fromSession {
	case "":
		if env.SessionID != "" {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), fromSessionHint+"\n", env.SessionID); err != nil {
				return agentidentity.Credentials{}, err
			}
		}
		return agentidentity.Credentials{}, flags.requireNoReply()
	case fromSessionNone:
		return agentidentity.Credentials{}, flags.requireNoReply()
	case fromSessionAuto:
		if env.SessionID == "" {
			return agentidentity.Credentials{}, errFromSessionAutoOutsideSession
		}
		return env, nil
	case env.SessionID:
		return env, nil
	}
	lookup, ok := client.(agentSessionClient)
	if !ok {
		return agentidentity.Credentials{}, errors.New("cli: --from-session requires a daemon session lookup")
	}
	record, err := lookup.GetSession(cmd.Context(), fromSession)
	if err != nil {
		return agentidentity.Credentials{}, fmt.Errorf("cli: resolve --from-session %q: %w", fromSession, err)
	}
	return agentidentity.Credentials{SessionID: fromSession, AgentName: strings.TrimSpace(record.AgentName)}, nil
}

func (flags *sessionPromptCommandFlags) requireNoReply() error {
	if strings.TrimSpace(flags.inReplyTo) != "" {
		return errPromptReplyRequiresSession
	}
	if flags.supersede {
		return errPromptSupersedeRequiresSession
	}
	return nil
}

func (flags *sessionPromptCommandFlags) validateOriginFlags(cmd *cobra.Command) error {
	if cmd.Flags().Changed("from-session") && strings.TrimSpace(flags.fromSession) == "" {
		return errors.New("cli: --from-session must not be blank")
	}
	if cmd.Flags().Changed("in-reply-to") && strings.TrimSpace(flags.inReplyTo) == "" {
		return errors.New("cli: --in-reply-to must not be blank")
	}
	if flags.steer && (cmd.Flags().Changed("from-session") || cmd.Flags().Changed("in-reply-to") || flags.supersede) {
		return errors.New("cli: --from-session, --in-reply-to, and --supersede apply only to submitted prompts")
	}
	return nil
}

func hasOriginCredentials(credentials agentidentity.Credentials) bool {
	return strings.TrimSpace(credentials.SessionID) != ""
}

func sendSessionPromptWithOrigin(
	ctx context.Context,
	client sessionPromptClient,
	id string,
	request SessionPromptRequest,
	credentials agentidentity.Credentials,
) (SessionPromptRecord, error) {
	if asClient, ok := client.(sessionPromptAsClient); ok && hasOriginCredentials(credentials) {
		return asClient.SendSessionPromptAs(ctx, id, request, credentials)
	}
	return client.SendSessionPrompt(ctx, id, request)
}

func streamSessionPromptWithOrigin(
	ctx context.Context,
	client sessionPromptStreamClient,
	id string,
	request SessionPromptRequest,
	credentials agentidentity.Credentials,
	handler SSEHandler,
) error {
	if asClient, ok := client.(sessionPromptStreamAsClient); ok && hasOriginCredentials(credentials) {
		return asClient.StreamPromptSessionAs(ctx, id, request, credentials, handler)
	}
	return client.StreamPromptSession(ctx, id, request, handler)
}
