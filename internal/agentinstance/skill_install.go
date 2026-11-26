package agentinstance

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Approve installs a pending `new` review through the install transaction. Approving a review
// whose install already committed is an idempotent success with no writes. A Warning on the result
// reports a post-commit projection or publication failure; the skill is installed regardless.
func (s *SkillProposalService) Approve(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	id, digest, operator, targetRoot string,
) (_ SkillInstallResult, err error) {
	if !actor.Operator {
		return SkillInstallResult{}, ErrSkillReviewOperatorOnly
	}
	if operator == "" {
		return SkillInstallResult{}, errors.New("skill review operator identity is required")
	}
	if targetRoot == "" {
		return SkillInstallResult{}, ErrSkillReviewNoTarget
	}
	review, draft, err := s.Review(ctx, actor, instance, id)
	if err != nil {
		return SkillInstallResult{}, err
	}
	if review.Digest != digest {
		return SkillInstallResult{}, ErrSkillReviewStale
	}
	if !proposedSkillName.MatchString(draft.Name) || draft.Name != review.SkillName {
		return SkillInstallResult{}, ErrSkillReviewInvalid
	}
	t, err := s.acquireSkill(ctx, targetRoot, draft.Name, true)
	if err != nil {
		return SkillInstallResult{}, err
	}
	defer func() {
		if closeErr := t.close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	return s.approveLocked(ctx, t, instance, id, digest, operator, draft)
}

func (s *SkillProposalService) approveLocked(
	ctx context.Context,
	t *skillTarget,
	instance Instance,
	id, digest, operator string,
	draft SkillDraft,
) (SkillInstallResult, error) {
	blocked, warning, err := s.reconcileSkill(ctx, t)
	if err != nil {
		return SkillInstallResult{}, err
	}
	if blocked != nil {
		return SkillInstallResult{}, skillInstallConflictError(t.name, blocked.OpID)
	}
	// Step 2: re-read the review and the install log after Reconcile projected them.
	review, err := s.reviewRow(ctx, instance, id)
	if err != nil {
		return SkillInstallResult{}, err
	}
	entries, err := readSkillInstallLog(t.root, t.name)
	if err != nil {
		return SkillInstallResult{}, err
	}
	current := currentSkillWriter(entries)
	live, err := skillTreeState(t.dir)
	if errors.Is(err, ErrSkillTreeUnsupported) {
		// Something that is not a skill tree occupies the target name.
		return SkillInstallResult{}, errors.Join(skillTargetExistsError(t.name), err)
	}
	if err != nil {
		return SkillInstallResult{}, err
	}
	committed := current != nil && current.ReviewID == review.ID && live == current.InstalledDigest
	switch {
	case review.Digest != digest:
		return SkillInstallResult{}, ErrSkillReviewStale
	case review.State == "approved" && committed && review.InstalledDigest == live:
		return SkillInstallResult{Warning: warning}, nil
	case review.State == SkillReviewPending && committed:
		// Committed earlier; its projection is still failing and stays reported.
		return SkillInstallResult{Warning: warning}, nil
	case review.State != SkillReviewPending:
		return SkillInstallResult{}, ErrSkillReviewStale
	case review.Kind != SkillProposalNew:
		return SkillInstallResult{}, ErrSkillReviewInvalid
	case live != "":
		return SkillInstallResult{}, skillTargetExistsError(t.name)
	}
	result, err := s.runSkillOp(ctx, t, skillOp{
		reviewID: review.ID, instanceID: instance.ID, action: SkillInstallActionInstall,
		decidedVia: skillDecidedViaOperator, decidedBy: operator,
		changed: skillTargetExistsError(t.name),
		stage:   func(dir string) error { return writeSkillInstallStage(dir, draft) },
		prev:    current,
		next: &SkillInstallMarker{
			ReviewID: review.ID, InstanceID: instance.ID, HomeID: s.HomeID,
			DecidedVia: skillDecidedViaOperator, DecidedBy: operator,
		},
	})
	if err != nil {
		return SkillInstallResult{}, err
	}
	slog.InfoContext(ctx, "skill.review.decided", "instance_id", instance.ID, "review_id", review.ID,
		"decision", "approved", "kind", string(review.Kind), "decided_via", skillDecidedViaOperator,
		"skill_key", string(review.SkillKey))
	result.Warning = errors.Join(warning, result.Warning)
	return result, nil
}

const skillDecidedViaOperator = "operator"

func skillTargetExistsError(name string) error {
	return fmt.Errorf("%w: %s is already installed; reject this review and ask for an edit proposal",
		ErrSkillReviewTargetExists, name)
}

func writeSkillInstallStage(stage string, draft SkillDraft) error {
	for _, file := range draft.Files {
		if !fs.ValidPath(file.Path) || file.Path == "." || strings.ContainsAny(file.Path, "\\\x00:") {
			return ErrSkillReviewInvalid
		}
		path := filepath.Join(stage, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := writeSkillFile(path, []byte(file.Content), 0o600); err != nil {
			return err
		}
	}
	for _, file := range draft.Files {
		if filepath.Base(file.Path) == skillDefinitionFile {
			if _, err := skills.ParseSkillFile(filepath.Join(stage, filepath.FromSlash(file.Path))); err != nil {
				return err
			}
		}
	}
	return nil
}
