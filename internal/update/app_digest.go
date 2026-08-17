package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifyAppArtifact binds a shell attempt to the exact asset recorded in the operation.
func (s *OperationStore) VerifyAppArtifact(
	ctx context.Context,
	operationID string,
	executorGeneration string,
	expectedRevision int64,
	attemptID string,
	path string,
) error {
	if err := s.Fence(ctx, operationID, executorGeneration, expectedRevision); err != nil {
		return err
	}
	operation, err := s.Read(ctx)
	if err != nil {
		return err
	}
	if operation == nil || operation.ID != operationID || operation.App == nil {
		return ErrOperationNotFound
	}
	if operation.App.AttemptID != strings.TrimSpace(attemptID) {
		return s.failAppDigest(ctx, operation, executorGeneration, "app attempt id does not match the operation")
	}
	digest, err := sha256File(path)
	if err != nil {
		return err
	}
	want := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(operation.App.Digest)), "sha256:")
	if !strings.EqualFold(digest, want) {
		return s.failAppDigest(ctx, operation, executorGeneration, "app artifact digest does not match the operation")
	}
	_, err = s.Transition(ctx, operation.ID, executorGeneration, operation.Revision, Transition{
		Kind: TransitionVerifyApp, Actor: ActorShell, Target: TargetApp, Percent: operation.Percent,
	})
	return err
}

func (s *OperationStore) failAppDigest(
	ctx context.Context,
	operation *Operation,
	executorGeneration string,
	reason string,
) error {
	_, transitionErr := s.Transition(ctx, operation.ID, executorGeneration, operation.Revision, Transition{
		Kind: TransitionPhase, Actor: ActorShell, Target: TargetApp, Phase: PhaseFailed,
		Percent: -1, LastError: reason, Outcome: operationOutcomeFailed,
	})
	return errors.Join(errors.New(reason), transitionErr)
}

func sha256File(path string) (digest string, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("update: open artifact for digest verification: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("update: hash artifact: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
