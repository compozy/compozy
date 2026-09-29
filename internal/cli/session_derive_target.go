package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

// SessionDerivePreviewRecord is the daemon continue/fork preview response.
type SessionDerivePreviewRecord = contract.SessionDerivePreviewResponse

// SessionDeriveTarget names the source session of a continue or fork and the
// workspace whose route serves it.
type SessionDeriveTarget struct {
	SessionID    string
	WorkspaceRef string
}

// resolveSessionDeriveTarget finds the source's workspace and, when the operator set no
// fences, the source transcript fences, so the derive refuses a transcript that moved in
// between. Both reads are the non-repairing ones (the owner lookup and the derive
// preview): a continue or fork never writes its source, not even to repair an inactive
// session's metadata.
//
// A source that cannot be read is not fatal for a retry: a rerun with the same
// --idempotency-key must reach the daemon, which replays its recorded outcome before it
// reads the source. The command workspace (flag/env/cwd) then names the route when the
// owner is unknown, and no fences are sent when the preview failed; the derive itself
// reports any refusal of a request that has no receipt.
func resolveSessionDeriveTarget(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	sessionID string,
	epochOut, generationOut, maxSequenceOut **int64,
) (SessionDeriveTarget, error) {
	ctx := cmd.Context()
	target := SessionDeriveTarget{SessionID: strings.TrimSpace(sessionID)}
	owner, err := client.GetSessionOwner(ctx, target.SessionID)
	switch {
	case err == nil:
		target.WorkspaceRef = strings.TrimSpace(owner.WorkspaceID)
		if target.WorkspaceRef == "" {
			return SessionDeriveTarget{}, fmt.Errorf("cli: session %q has no workspace_id", sessionID)
		}
	case isDaemonNotFound(err) || cmd.Flags().Changed("idempotency-key"):
		resolution, resolveErr := resolveCommandWorkspace(ctx, cmd, deps, client, workspaceResolutionRequest{})
		if resolveErr != nil {
			return SessionDeriveTarget{}, fmt.Errorf(
				"cli: resolve session %q workspace: %w",
				sessionID,
				errors.Join(err, resolveErr),
			)
		}
		target.WorkspaceRef = strings.TrimSpace(resolution.ID)
		return target, nil
	default:
		return SessionDeriveTarget{}, fmt.Errorf("cli: resolve session %q workspace: %w", sessionID, err)
	}
	if *epochOut != nil {
		return target, nil
	}
	// Without a preview the derive runs unfenced: it reads the source itself and
	// reports its own refusal, or replays the receipt of a same-key retry.
	if preview, previewErr := client.PreviewSessionDerive(ctx, target); previewErr == nil {
		fences := preview.Transcript
		epoch, generation, maxSequence := fences.Epoch, fences.Generation, fences.MaxSequence
		*epochOut, *generationOut, *maxSequenceOut = &epoch, &generation, &maxSequence
	}
	return target, nil
}

func isDaemonNotFound(err error) bool {
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	return ok && apiErr != nil && apiErr.statusCode == http.StatusNotFound
}

// GetSessionOwner resolves the workspace that owns a session without repairing it.
func (c *daemonClient) GetSessionOwner(ctx context.Context, id string) (contract.SessionOwner, error) {
	sessionID, err := requirePathValue("session_id", id)
	if err != nil {
		return contract.SessionOwner{}, err
	}
	var response contract.SessionOwner
	if err := c.doJSON(
		ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(sessionID)+"/owner", nil, nil, &response,
	); err != nil {
		return contract.SessionOwner{}, err
	}
	return response, nil
}

// PreviewSessionDerive reads what a continue of the source would carry, including its
// transcript fences; the preview never writes the source.
func (c *daemonClient) PreviewSessionDerive(
	ctx context.Context,
	target SessionDeriveTarget,
) (SessionDerivePreviewRecord, error) {
	path, err := c.sessionDerivePath(ctx, target, "/derive/preview")
	if err != nil {
		return SessionDerivePreviewRecord{}, err
	}
	var response SessionDerivePreviewRecord
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SessionDerivePreviewRecord{}, err
	}
	return response, nil
}
