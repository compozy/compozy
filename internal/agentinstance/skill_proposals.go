package agentinstance

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var ErrSkillReviewNoTarget = errors.New("skill_review_no_target")

type SkillProposalInput struct {
	Name        string
	Description string
	Body        string
	// Files is the full set for new; for edit, the added or replaced text files.
	Files []SkillDraftFile
	// RemovePaths deletes base files of an edit; refused with mode new.
	RemovePaths []string
	// Mode defaults to new.
	Mode               SkillProposalMode
	Lesson             SkillLesson
	EvidenceSessionIDs []string
}

type SkillProposalService struct {
	Store     SkillReviewStore
	Evidence  EvidenceSessions
	Authority Authority
	DraftRoot string
	ScanRoots []string
	// HomeID is the SHA-256 of this CompozyOS home's canonical directory; install-log entries carry
	// it so each home projects only its own reviews.
	HomeID string
	// Publisher refreshes the catalog after every committed change; nil publishes nothing.
	Publisher SkillPublisher
	// Locks is the daemon's keyed mutex per physical skill; nil uses a process-wide default.
	Locks *SkillInstallLocks
	// crashAt is a test seam: returning true at a named transaction step stops the transaction there
	// as a crash would, leaving disk state for the next Reconcile.
	crashAt func(step string) bool
}

type SkillProposalResult struct {
	Review SkillReview
	// Duplicate is true when a pending review with the same identity was returned unchanged.
	Duplicate bool
}

func (s *SkillProposalService) Propose(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	targetRoot string,
	input SkillProposalInput,
) (_ SkillProposalResult, err error) {
	lesson, evidence, err := prepareProposalClaim(input)
	if err != nil {
		return SkillProposalResult{}, err
	}
	if s.Store == nil {
		return SkillProposalResult{}, errors.New("skill proposal store is required")
	}
	if err := s.Authority.Authorize(ctx, actor, instance, "skill_propose"); err != nil {
		return SkillProposalResult{}, err
	}
	if targetRoot == "" {
		return SkillProposalResult{}, ErrSkillReviewNoTarget
	}
	draft, err := prepareProposalDraft(input)
	if err != nil {
		return SkillProposalResult{}, err
	}
	// Checked before the lock too, whose path resolution would reject a symlink leaving the root
	// before the name is judged.
	if err := refuseNewInstalledName(targetRoot, draft); err != nil {
		return SkillProposalResult{}, err
	}
	if err := s.checkEvidenceSessions(ctx, instance, evidence); err != nil {
		return SkillProposalResult{}, err
	}
	t, blocked, err := s.lockReconciledSkill(ctx, targetRoot, draft.Name)
	if err != nil {
		return SkillProposalResult{}, err
	}
	defer func() { err = errors.Join(err, t.close()) }()
	// Checked again after Reconcile, so an interrupted install is settled before name or base is judged.
	edit, err := checkProposalTarget(t, targetRoot, draft, blocked)
	if err != nil {
		return SkillProposalResult{}, err
	}
	id, report, err := newSkillReviewRecord(draft.Name, evidence)
	if err != nil {
		return SkillProposalResult{}, err
	}
	path, digest, snapshot, err := s.writeProposal(id, draft, edit)
	if err != nil {
		return SkillProposalResult{}, err
	}
	review, duplicate, err := s.Store.CreateReview(ctx, SkillReview{
		ID:                 id,
		InstanceID:         instance.ID,
		ProfileID:          instance.ProfileID,
		WorkspaceID:        instance.WorkspaceID,
		AgentName:          instance.AgentName,
		SkillName:          input.Name,
		Digest:             digest,
		DraftPath:          path,
		SourceSessionID:    actor.SessionID,
		Kind:               draft.kind(),
		SkillKey:           skillKeyFor(instance, t.root, draft.Name),
		BaseDigest:         edit.baseDigest,
		SnapshotPath:       snapshot,
		ProposedDigest:     edit.proposedDigest,
		Lesson:             lesson,
		EvidenceSessionIDs: evidence,
	}, report)
	result := SkillProposalResult{Review: review, Duplicate: duplicate}
	if errors.Is(err, ErrSkillReviewLimit) || errors.Is(err, ErrSkillProposePreviouslyRejected) ||
		(err == nil && review.DraftPath != path) {
		return result, errors.Join(err, os.Remove(path), removeSkillSnapshot(snapshot))
	}
	if err == nil {
		slog.InfoContext(ctx, "skill.review.created", "instance_id", instance.ID, "review_id", review.ID,
			"kind", string(review.Kind), "decided_via", review.DecidedVia)
	}
	// An uncertain database commit must retain the inert draft for recovery.
	return result, err
}

// prepareProposalClaim validates the lesson and the 1–20 distinct evidence sessions.
func prepareProposalClaim(input SkillProposalInput) (SkillLesson, []string, error) {
	lesson, err := prepareSkillLesson(input.Lesson)
	if err != nil {
		return SkillLesson{}, nil, err
	}
	evidence, err := distinctEvidenceSessions(input.EvidenceSessionIDs)
	return lesson, evidence, err
}

// prepareProposalDraft validates the proposal contents; an edit draft also records its mode and
// remove_paths so they are part of the draft digest.
func prepareProposalDraft(input SkillProposalInput) (SkillDraft, error) {
	switch input.Mode {
	case "", SkillProposalNew, SkillProposalEdit:
	default:
		return SkillDraft{}, fmt.Errorf("skill proposal mode %q must be new or edit", input.Mode)
	}
	removals, err := prepareSkillRemovePaths(input.Mode, input.RemovePaths, input.Files)
	if err != nil {
		return SkillDraft{}, err
	}
	draft, err := PrepareSkillDraft(input.Name, input.Description, input.Body, input.Files)
	if err != nil {
		return SkillDraft{}, err
	}
	if input.Mode == SkillProposalEdit {
		draft.Mode, draft.RemovePaths = SkillProposalEdit, removals
	}
	return draft, nil
}

// checkProposalTarget applies Business Rule 4 under the skill locks: a new skill needs a free name; an
// edit needs a settled, supported installed base and returns it with the resulting tree.
func checkProposalTarget(
	t *skillTarget,
	targetRoot string,
	draft SkillDraft,
	blocked *SkillInstallPayload,
) (skillEdit, error) {
	if draft.Mode != SkillProposalEdit {
		return skillEdit{}, refuseNewInstalledName(targetRoot, draft)
	}
	if blocked != nil {
		return skillEdit{}, skillInstallConflictError(draft.Name, blocked.OpID)
	}
	return prepareSkillEdit(t.dir, draft)
}

// writeProposal writes the inert draft and, for an edit, the base snapshot beside it.
func (s *SkillProposalService) writeProposal(
	id string,
	draft SkillDraft,
	edit skillEdit,
) (path, digest, snapshot string, err error) {
	path, digest, err = WriteSkillDraft(s.DraftRoot, s.ScanRoots, draft)
	if err != nil || draft.Mode != SkillProposalEdit {
		return path, digest, "", err
	}
	snapshot = skillSnapshotPath(path, id)
	if err := writeSkillSnapshot(snapshot, edit.base, edit.baseDigest); err != nil {
		return "", "", "", errors.Join(err, os.Remove(path))
	}
	return path, digest, snapshot, nil
}

func removeSkillSnapshot(path string) error {
	if path == "" {
		return nil
	}
	return os.RemoveAll(path)
}

// refuseNewInstalledName applies refuseInstalledName to `new` proposals; an edit requires the name.
func refuseNewInstalledName(targetRoot string, draft SkillDraft) error {
	if draft.Mode == SkillProposalEdit {
		return nil
	}
	return refuseInstalledName(targetRoot, draft.Name)
}

// refuseInstalledName applies Business Rule 4 for a new skill: any entry already at the name in the
// target root, a symlink included, is refused, so re-proposing installed contents never opens another
// review. The name is validated by PrepareSkillDraft and cannot leave the root.
func refuseInstalledName(targetRoot, name string) error {
	if _, err := os.Lstat(filepath.Join(targetRoot, name)); err == nil {
		return fmt.Errorf("%w: a skill named %s is already installed for this instance", ErrSkillProposeNameTaken, name)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// newSkillReviewRecord allocates the review ID and its proposal report.
func newSkillReviewRecord(name string, evidence []string) (string, ReportInput, error) {
	id, err := store.NewID("skr")
	if err != nil {
		return "", ReportInput{}, err
	}
	report, err := skillReviewReport(id, name, evidence)
	return id, report, err
}

// skillReviewReport is the proposal report linking the operator's report feed to the review.
func skillReviewReport(id, name string, evidence []string) (ReportInput, error) {
	payload, err := json.Marshal(struct {
		SkillReviewID      string              `json:"skill_review_id"`
		EvidenceSessionIDs []string            `json:"evidence_session_ids,omitempty"`
		Proposals          []map[string]string `json:"proposals"`
	}{SkillReviewID: id, EvidenceSessionIDs: evidence, Proposals: []map[string]string{{
		"title": "Review proposed skill: " + name,
		"body":  "Inspect the exact contents before approving or rejecting this skill.",
	}}})
	if err != nil {
		return ReportInput{}, err
	}
	report := ReportInput{SourceKey: "skill_review:" + id, Kind: ReportKindProposal, Payload: payload}
	return report, report.Validate()
}

// skillKeyFor names the physical skill root/name: workspace targets by workspace ID, agent-local
// targets by the canonical agent directory that owns the skills root.
func skillKeyFor(instance Instance, root, name string) SkillKey {
	if instance.WorkspaceID != "" {
		return SkillKey("workspace:" + instance.WorkspaceID + "/" + name)
	}
	return SkillKey("agent-dir:" + filepath.ToSlash(filepath.Dir(root)) + "/" + name)
}

// lockReconciledSkill holds both skill locks and runs Reconcile before a review is created, so a
// skill another process is changing refuses the proposal (skill_install_busy) with no review.
// The returned payload is the skill's unresolved conflict, if any.
func (s *SkillProposalService) lockReconciledSkill(
	ctx context.Context,
	targetRoot, name string,
) (*skillTarget, *SkillInstallPayload, error) {
	t, err := s.acquireSkill(ctx, targetRoot, name, false)
	if err != nil {
		return nil, nil, err
	}
	blocked, warning, err := s.reconcileSkill(ctx, t)
	if err != nil {
		return nil, nil, errors.Join(err, t.close())
	}
	if warning != nil {
		slog.WarnContext(ctx, "skill.install.projection_failed", "skill", name, "error", warning)
	}
	return t, blocked, nil
}
