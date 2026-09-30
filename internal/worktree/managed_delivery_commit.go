package worktree

import (
	"context"
	"errors"
	"strings"
)

func (s *Service) commitManagedDelivery(
	ctx context.Context,
	path string,
	j *managedDeliveryJournal,
	operation ExitOperation,
) error {
	if j.Phase != "prepared" && j.Phase != deliveryPhaseCommitting {
		return nil
	}
	head, err := s.deliveryGitValue(ctx, j.Item, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	message := strings.TrimSpace(j.Request.Message)
	if message == "" {
		message = "Update worktree"
	}
	message += "\n\nCompozy-Delivery: " + j.Request.DeliveryID
	if head == j.OriginalHead {
		if err := s.createManagedDeliveryCommit(ctx, path, j, operation, message); err != nil {
			return err
		}
		head, err = s.deliveryGitValue(ctx, j.Item, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
	}

	if head != j.OriginalHead {
		if err := s.verifyInterruptedDeliveryCommit(ctx, j, message, head); err != nil {
			return err
		}
	}

	j.Head = head
	j.Phase = "committed"
	j.Result.Steps = append(j.Result.Steps, ExitStepResult{Phase: ExitPhaseCommit, State: exitStepCompleted, SHA: head})
	return saveDeliveryJournal(path, j)
}

func (s *Service) reconcileManagedPR(ctx context.Context, j *managedDeliveryJournal) error {
	if s.forge == nil {
		return ErrForgeUnavailable
	}
	status, err := s.forge.Status(
		ctx,
		ForgeStatusRequest{
			WorkspaceID: j.Item.WorkspaceID,
			WorktreeID:  j.Item.ID,
			RemoteURLs:  j.RemoteURLs,
			Branch:      j.Item.Branch,
			Base:        j.Request.Base,
			HeadSHA:     j.Head,
		},
	)
	if err != nil {
		return err
	}
	if status == nil {
		return ErrForgeUnavailable
	}
	if status.PRNumber != nil {
		if status.Head != j.Item.Branch || status.Base != j.Request.Base || status.HeadSHA != j.Head ||
			status.Draft == nil ||
			!*status.Draft ||
			status.PRState == nil ||
			*status.PRState != "open" {
			return refusal(ErrSafetyCheckFailed, "Existing PR does not match the intended draft delivery.")
		}
		url, err := sanitizeForgeWebURL(status.PRURL)
		if err != nil {
			return err
		}
		j.Result.Steps = append(
			j.Result.Steps,
			ExitStepResult{
				Phase:    ExitPhasePR,
				State:    exitStepCompleted,
				PRStatus: "existing",
				PRNumber: *status.PRNumber,
				URL:      url,
			},
		)
		j.Result.CTA = &ExitCTA{Action: ExitActionViewPR, Label: exitViewPRLabel, URL: url}
		return s.store.SaveForgeStatus(ctx, j.Item.WorkspaceID, j.Item.ID, *status)
	}
	// Persisted phase remains "pr" until success; ambiguous responses are always
	// reconciled by the exact provider query above before any subsequent create.
	capabilities, err := s.forge.Capabilities(ctx, j.RemoteURLs)
	if err != nil {
		return err
	}
	if capabilities == nil || !capabilities.SupportsDraft {
		return ErrForgeUnavailable
	}
	prRequest := j.Request
	prRequest.ExpectedHead = j.Head
	step, err := s.runExitPR(
		ctx,
		j.Item,
		&ExitPlan{Base: j.Request.Base, RemoteURLs: j.RemoteURLs, Forge: capabilities},
		prRequest,
	)
	if err != nil {
		return err
	}
	if step.PRStatus == exitPRStatusBrowser {
		return errors.New("managed delivery requires a native forge provider")
	}
	j.Result.Steps = append(j.Result.Steps, step)
	j.Result.CTA = &ExitCTA{Action: ExitActionViewPR, Label: exitViewPRLabel, URL: step.URL}
	return nil
}

func (s *Service) verifyInterruptedDeliveryCommit(
	ctx context.Context,
	j *managedDeliveryJournal,
	message, head string,
) error {
	tree, err := s.deliveryGitValue(ctx, j.Item, "rev-parse", head+"^{tree}")
	if err != nil {
		return err
	}
	parent, err := s.deliveryGitValue(ctx, j.Item, "rev-parse", head+"^")
	if err != nil {
		return err
	}
	recordedMessage, err := s.deliveryGitValue(ctx, j.Item, "log", "-1", "--format=%B", head)
	if err != nil {
		return err
	}
	if j.Phase != deliveryPhaseCommitting || j.Tree == "" || tree != j.Tree || parent != j.OriginalHead ||
		recordedMessage != message {
		return refusal(ErrSafetyCheckFailed, "Interrupted commit does not match delivery intent.")
	}

	return nil
}

func (s *Service) createManagedDeliveryCommit(
	ctx context.Context, path string, j *managedDeliveryJournal, operation ExitOperation, message string,
) error {
	snapshot, err := s.deliveryIntentSnapshot(ctx, j.Item, j.Request.IncludePaths)
	if err != nil {
		return err
	}
	if snapshot != j.Snapshot {
		return refusal(ErrSafetyCheckFailed, "Delivery inputs changed after authorization.")
	}
	tree, err := s.deliveryExpectedTree(ctx, j.Item, j.Request.IncludePaths)
	if err != nil {
		return err
	}
	originalTree, err := s.deliveryGitValue(ctx, j.Item, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return err
	}
	if tree != originalTree {
		if j.Tree != "" && j.Tree != tree {
			return ErrSafetyCheckFailed
		}
		j.Tree = tree
		j.Phase = deliveryPhaseCommitting
		if err := saveDeliveryJournal(path, j); err != nil {
			return err
		}
		if len(j.Request.IncludePaths) > 0 {
			if err := s.stageSelectedCommitPaths(ctx, j.Item.Path, j.Request.IncludePaths); err != nil {
				return err
			}
			args := append(
				[]string{gitLiteralPathspecs, "commit", "--only", "-m", message, "--"},
				j.Request.IncludePaths...)
			if _, _, err := s.runExitCommitArgs(
				ctx,
				operation,
				ExitActionDeliver,
				j.Item.Path,
				args...); err != nil {
				return err
			}
		} else {
			if _, _, err := s.runner.Run(ctx, j.Item.Path, "add", "-A"); err != nil {
				return err
			}
			if _, _, err := s.runExitCommitCommand(
				ctx,
				operation,
				ExitActionDeliver,
				j.Item.Path,
				message,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
