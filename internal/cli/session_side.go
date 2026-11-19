package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type SessionSideClient interface {
	OpenSessionSide(context.Context, SessionDeriveTarget, contract.OpenSessionSideRequest) (SessionDeriveRecord, error)
	ListSessionSides(context.Context, string, bool) (contract.SessionSidesResponse, error)
}

var _ SessionSideClient = (*daemonClient)(nil)

func newSessionSideCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "side", Short: "Open and list side chats attached to a session"}
	cmd.AddCommand(newSessionSideOpenCommand(deps), newSessionSideListCommand(deps))
	return cmd
}

func newSessionSideOpenCommand(deps commandDeps) *cobra.Command {
	var name, key string
	cmd := &cobra.Command{
		Use: "open <session-id> <message...>", Short: "Ask a side question without interrupting the session",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			sideClient, ok := client.(SessionSideClient)
			if !ok {
				return errors.New("cli: side chats are unavailable")
			}
			identity, err := deriveIdempotencyKey(key)
			if err != nil {
				return err
			}
			var epoch, generation, maxSequence *int64
			target, err := resolveSessionDeriveTarget(cmd, deps, client, args[0], &epoch, &generation, &maxSequence)
			if err != nil {
				return err
			}
			response, err := sideClient.OpenSessionSide(cmd.Context(), target, contract.OpenSessionSideRequest{
				Message: strings.TrimSpace(
					strings.Join(args[1:], " "),
				),
				Name:           strings.TrimSpace(name),
				IdempotencyKey: identity,
			})
			if err != nil {
				return sessionSideFailure(err, identity, strings.TrimSpace(key) == "")
			}
			bundle := sessionDeriveBundle(&response)
			bundle.human = func() (string, error) {
				d := response.Derived
				return sideOpenedText(d.ChildSessionID, d.SourceSessionID, d.Seed, d.SourceTurnInProgress), nil
			}
			return writeCommandOutput(cmd, bundle)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Title instead of the automatic title")
	cmd.Flags().StringVar(&key, "idempotency-key", "", "Stable retry key (generated when omitted)")
	return cmd
}

func newSessionSideListCommand(deps commandDeps) *cobra.Command {
	var archived bool
	cmd := &cobra.Command{Use: "list <session-id>", Short: "List a session's side chats", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			sideClient, ok := client.(SessionSideClient)
			if !ok {
				return errors.New("cli: side chats are unavailable")
			}
			response, err := sideClient.ListSessionSides(cmd.Context(), args[0], archived)
			if err != nil {
				return sessionSideFailure(err, "", false)
			}
			return writeCommandOutput(cmd, sessionSidesBundle(args[0], response, deps.now))
		},
	}
	cmd.Flags().BoolVar(&archived, "archived", false, "Include closed side chats")
	return cmd
}

func sideOpenedText(id, source, seed string, inProgress bool) string {
	location := ""
	if source != "" {
		location = " on " + source
	}
	progress := ""
	if inProgress {
		progress = ", main turn in progress"
	}
	return fmt.Sprintf("Opened side chat %s%s (seed: %s%s).", id, location, seed, progress)
}

func sessionSidesBundle(source string, response contract.SessionSidesResponse, now func() time.Time) outputBundle {
	if response.Sides == nil {
		response.Sides = []contract.SessionSideItemPayload{}
	}
	return outputBundle{jsonValue: response,
		human: func() (string, error) {
			if len(response.Sides) == 0 {
				return "No side chats on " + source + ".", nil
			}
			rows := make([][]string, 0, len(response.Sides))
			for _, side := range response.Sides {
				rows = append(rows, []string{side.ID, side.Name, side.State, formatAge(now, side.UpdatedAt)})
			}
			return renderHumanTable("Side chats", []string{"ID", cliTitleHeader, "STATE", "UPDATED"}, rows), nil
		},
		toon: func() (string, error) {
			rows := make([][]string, 0, len(response.Sides))
			for _, side := range response.Sides {
				rows = append(rows, []string{side.ID, side.Name, side.State})
			}
			return renderToonArray("sides", []string{"id", automationNameKey, "state"}, rows), nil
		},
	}
}

func sessionSideFailure(err error, key string, generated bool) error {
	failure := err
	childID := ""
	if derived, ok := errors.AsType[*sessionDeriveAPIError](err); ok {
		childID = derived.payload.ChildSessionID
		failure = sessionDeriveCommandFailure(err, key)
	} else if api, ok := errors.AsType[*daemonAPIError](err); ok && api.payload.Code != "" {
		failure = &sessionDeriveCommandError{message: api.payload.Error + " (" + api.payload.Code + ")", err: err}
	}
	if generated && childID == "" {
		return &sessionDeriveCommandError{
			message: failure.Error() + "\nretry with --idempotency-key " + key + " to reuse the same side chat request",
			err:     failure,
		}
	}
	return failure
}

func (c *daemonClient) OpenSessionSide(
	ctx context.Context, target SessionDeriveTarget, request contract.OpenSessionSideRequest,
) (SessionDeriveRecord, error) {
	return c.deriveSession(ctx, target, "/sides", request)
}

func (c *daemonClient) ListSessionSides(
	ctx context.Context,
	id string,
	archived bool,
) (contract.SessionSidesResponse, error) {
	path, err := c.sessionScopedPath(ctx, id, "/sides")
	if err != nil {
		return contract.SessionSidesResponse{}, fmt.Errorf("cli: resolve side list scope: %w", err)
	}
	var response contract.SessionSidesResponse
	query := url.Values{}
	if archived {
		query.Set("archived", "true")
	}
	err = c.doJSON(ctx, http.MethodGet, path, query, nil, &response)
	if err != nil {
		return contract.SessionSidesResponse{}, fmt.Errorf("cli: list side chats: %w", err)
	}
	return response, nil
}
