package agentinstance

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (f *skillTxFixture) revert(
	t *testing.T,
	svc *SkillProposalService,
	review SkillReview,
) (SkillInstallResult, error) {
	t.Helper()
	return svc.Revert(t.Context(), skillOperator, f.instance, review.ID, review.Digest, "operator:prof-a", f.root)
}

// mustApprove commits review and returns its projected row.
func (f *skillTxFixture) mustApprove(t *testing.T, svc *SkillProposalService, review SkillReview) SkillReview {
	t.Helper()
	if result, err := f.approve(t, svc, review); err != nil || !result.Committed {
		t.Fatalf("Approve(%s) = %+v, %v", review.ID, result, err)
	}
	return requireReviewState(t, svc, review.ID, skillReviewApproved)
}

func (f *skillTxFixture) mustRevert(t *testing.T, svc *SkillProposalService, review SkillReview) SkillReview {
	t.Helper()
	if result, err := f.revert(t, svc, review); err != nil || !result.Committed || result.Warning != nil {
		t.Fatalf("Revert(%s) = %+v, %v", review.ID, result, err)
	}
	return requireReviewState(t, svc, review.ID, SkillReviewReverted)
}

func (f *skillTxFixture) eligibility(t *testing.T, svc *SkillProposalService, id string) RevertEligibility {
	t.Helper()
	review, err := svc.Store.GetReview(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	eligibility, err := svc.RevertEligibility(t.Context(), review, f.root)
	if err != nil {
		t.Fatalf("RevertEligibility(%s) = %v", id, err)
	}
	return eligibility
}

// withInstance returns a fixture on the same target root for another instance of the workspace.
func (f *skillTxFixture) withInstance(id string) *skillTxFixture {
	other := *f
	other.instance.ID = id
	return &other
}

func requireRevertConflict(t *testing.T, err error, reason string) {
	t.Helper()
	if !errors.Is(err, ErrSkillReviewRevertConflict) || SkillRevertReason(err) != reason {
		t.Fatalf("Revert() = %v (reason %q), want skill_review_revert_conflict %s", err, SkillRevertReason(err), reason)
	}
}

func (f *skillTxFixture) currentWriter(t *testing.T, name string) *SkillInstallMarker {
	t.Helper()
	entries, err := readSkillInstallLog(f.root, name)
	if err != nil {
		t.Fatal(err)
	}
	return currentSkillWriter(entries)
}

func (f *skillTxFixture) requireBaseRestored(t *testing.T, base string) {
	t.Helper()
	if got := f.digest(t, skillEditName); got != base {
		t.Fatalf("restored digest = %s, want the byte- and mode-exact base %s", got, base)
	}
	for _, file := range skillEditBase {
		data, mode := readSkillFileMode(t, filepath.Join(f.root, skillEditName, filepath.FromSlash(file.path)))
		if !bytes.Equal(data, []byte(file.content)) || mode != file.mode {
			t.Fatalf("%s = %q %o, want %q %o", file.path, data, mode, file.content, file.mode)
		}
	}
}

// approvedEditOverBase installs the operator-authored base, then proposes and approves one edit.
func (f *skillTxFixture) approvedEditOverBase(t *testing.T, svc *SkillProposalService) (string, SkillReview) {
	t.Helper()
	base := f.install(t, skillEditName, skillEditBase...)
	edit := f.mustProposeEdit(t, svc, "1. Run make gate.\n2. Confirm the freeze calendar.\n")
	return base, f.mustApprove(t, svc, edit)
}

// Invariant: an operator reverts only the install log's current writer whose live tree is still its
// installed tree; `new` removes the skill, `edit` restores the verified base byte- and mode-exact and
// makes the previous writer current again, across instances and homes.
// Owning layer: agentinstance install transaction; canonical suite for revert.
func TestSkillRevert(t *testing.T) {
	t.Parallel()

	t.Run("Should remove a reverted new skill [UT-032]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		reverted := f.mustRevert(t, svc, review)
		if f.digest(t, skillEditName) != "" {
			t.Fatal("the reverted new skill is still installed")
		}
		if reverted.RevertedBy != "operator:prof-a" || reverted.RevertedAt == nil {
			t.Fatalf("reverted review = %+v", reverted)
		}
		f.requireClean(t)
		entries, err := readSkillInstallLog(f.root, skillEditName)
		if err != nil || len(entries) != 2 {
			t.Fatalf("install log = %+v, %v", entries, err)
		}
		last := entries[1]
		if last.Action != SkillInstallActionRevert || last.ReviewID != review.ID || last.Current != nil ||
			last.Previous == nil || last.Previous.ReviewID != review.ID {
			t.Fatalf("revert entry = %+v", last)
		}
	})

	t.Run("Should restore the edited base byte- and mode-exact [UT-033]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		base, edit := f.approvedEditOverBase(t, svc)
		if f.digest(t, skillEditName) == base {
			t.Fatal("the edit did not change the skill")
		}
		f.mustRevert(t, svc, edit)
		f.requireBaseRestored(t, base)
		f.requireClean(t)
	})

	t.Run("Should refuse agent credentials [UT-034]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		agent := workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "sess-a"}
		_, err := svc.Revert(t.Context(), agent, f.instance, review.ID, review.Digest, "agent", f.root)
		if !errors.Is(err, ErrSkillReviewOperatorOnly) {
			t.Fatalf("agent Revert() = %v, want skill_review_operator_only", err)
		}
		requireReviewState(t, svc, review.ID, skillReviewApproved)
	})

	t.Run("Should refuse a skill changed after approval [UT-035]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		skillFile := filepath.Join(f.root, skillEditName, "SKILL.md")
		if err := os.WriteFile(skillFile, []byte("edited by hand"), 0o600); err != nil {
			t.Fatal(err)
		}
		changed := f.digest(t, skillEditName)
		_, err := f.revert(t, svc, review)
		requireRevertConflict(t, err, SkillRevertChangedOnDisk)
		if f.digest(t, skillEditName) != changed {
			t.Fatal("the refused revert wrote files")
		}
		requireReviewState(t, svc, review.ID, skillReviewApproved)
		f.requireClean(t)
	})

	t.Run("Should refuse an older approval under a newer one [UT-036]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		older := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		f.mustApprove(t, svc, f.mustProposeEdit(t, svc, "Check the changelog and the calendar.\n"))
		installed := f.digest(t, skillEditName)
		_, err := f.revert(t, svc, older)
		requireRevertConflict(t, err, SkillRevertSuperseded)
		if f.digest(t, skillEditName) != installed {
			t.Fatal("the refused revert wrote files")
		}
	})

	t.Run("Should repeat a revert without writing and refuse undecided reviews [UT-037]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		f.mustRevert(t, svc, review)
		writers := installLogWriters(t, f.root, skillEditName)
		if result, err := f.revert(t, svc, review); err != nil || result.Committed {
			t.Fatalf("repeat Revert() = %+v, %v; want an idempotent success", result, err)
		}
		if again := installLogWriters(t, f.root, skillEditName); len(again) != len(writers) {
			t.Fatalf("repeat revert appended to the install log: %v", again)
		}
		pending := f.propose(t, svc, "other-skill", "Pending.")
		if _, err := f.revert(t, svc, pending); !errors.Is(err, ErrSkillReviewStale) {
			t.Fatalf("Revert(pending) = %v, want skill_review_stale", err)
		}
		rejected := f.propose(t, svc, "rejected-skill", "Rejected.")
		if err := svc.Reject(t.Context(), skillOperator, f.instance, rejected.ID, rejected.Digest, "operator",
			f.root); err != nil {
			t.Fatal(err)
		}
		if _, err := f.revert(t, svc, rejected); !errors.Is(err, ErrSkillReviewStale) {
			t.Fatalf("Revert(rejected) = %v, want skill_review_stale", err)
		}
		_, err := svc.Revert(t.Context(), skillOperator, f.instance, review.ID, "sha256:other", "operator", f.root)
		if !errors.Is(err, ErrSkillReviewStale) {
			t.Fatalf("Revert(wrong digest) = %v, want skill_review_stale", err)
		}
	})

	t.Run("Should accept the same contents again after a revert [UT-051]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		f.mustRevert(t, svc, review)
		again := f.propose(t, svc, skillEditName, "Check the changelog.")
		if again.ID == review.ID || again.State != SkillReviewPending || again.Digest != review.Digest {
			t.Fatalf("re-proposal = %+v, want a new pending review of the same contents", again)
		}
	})

	t.Run("Should refuse an approval another instance changed and changed back [UT-061]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		other := f.withInstance("inst-b")
		first := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		other.mustApprove(t, svc, other.mustProposeEdit(t, svc, "Check the calendar.\n"))
		other.mustApprove(t, svc, other.mustProposeEdit(t, svc, "Check the changelog."))
		installed := f.digest(t, skillEditName)
		_, err := f.revert(t, svc, first)
		requireRevertConflict(t, err, SkillRevertSuperseded)
		if f.digest(t, skillEditName) != installed {
			t.Fatal("the refused revert wrote files")
		}
	})

	t.Run("Should report every revert eligibility like Revert does [UT-062]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		older := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		latest := f.mustApprove(t, svc, f.mustProposeEdit(t, svc, "Check the calendar.\n"))
		if got := f.eligibility(t, svc, latest.ID); !got.Allowed || got.Reason != "" {
			t.Fatalf("latest eligibility = %+v, want allowed", got)
		}
		if got := f.eligibility(t, svc, older.ID); got.Reason != SkillRevertSuperseded {
			t.Fatalf("older eligibility = %+v, want superseded", got)
		}
		pending := f.propose(t, svc, "other-skill", "Pending.")
		if got := f.eligibility(t, svc, pending.ID); got.Reason != SkillRevertNotApproved {
			t.Fatalf("pending eligibility = %+v, want not_approved", got)
		}
		legacy := latest
		legacy.ID, legacy.InstalledDigest, legacy.SkillKey = "skr-legacy", "", ""
		skillStore(svc).reviews[legacy.ID] = legacy
		if got := f.eligibility(t, svc, legacy.ID); got.Reason != SkillRevertLegacy {
			t.Fatalf("legacy eligibility = %+v, want legacy", got)
		}
		_, err := f.revert(t, svc, legacy)
		requireRevertConflict(t, err, SkillRevertLegacy)

		tampered := filepath.Join(latest.SnapshotPath, "SKILL.md")
		original, mode := readSkillFileMode(t, tampered)
		if err := os.WriteFile(tampered, []byte("tampered"), mode); err != nil {
			t.Fatal(err)
		}
		if got := f.eligibility(t, svc, latest.ID); got.Reason != SkillRevertTargetUnavailable {
			t.Fatalf("tampered snapshot eligibility = %+v, want target_unavailable", got)
		}
		_, err = f.revert(t, svc, latest)
		requireRevertConflict(t, err, SkillRevertTargetUnavailable)
		if err := os.WriteFile(tampered, original, mode); err != nil {
			t.Fatal(err)
		}

		installed := filepath.Join(f.root, skillEditName, "SKILL.md")
		if err := os.WriteFile(installed, []byte("edited by hand"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := f.eligibility(t, svc, latest.ID); got.Reason != SkillRevertChangedOnDisk {
			t.Fatalf("changed eligibility = %+v, want changed_on_disk", got)
		}
	})

	t.Run("Should keep the base snapshot of rejected and reverted edits [UT-071]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		f.install(t, skillEditName, skillEditBase...)
		rejected := f.mustProposeEdit(t, svc, "Rejected change.\n")
		if err := svc.Reject(t.Context(), skillOperator, f.instance, rejected.ID, rejected.Digest, "operator",
			f.root); err != nil {
			t.Fatal(err)
		}
		approved := f.mustApprove(t, svc, f.mustProposeEdit(t, svc, "Approved change.\n"))
		reverted := f.mustRevert(t, svc, approved)
		for _, review := range []SkillReview{requireReviewState(t, svc, rejected.ID, "rejected"), reverted} {
			contents, err := ReadSkillReviewContents(review)
			if err != nil || contents.SnapshotProblem != nil || len(contents.Base) != len(skillEditBase) ||
				len(contents.Changes) == 0 {
				t.Fatalf("contents of %s %s = %+v, %v", review.State, review.ID, contents, err)
			}
		}
	})

	t.Run("Should leave an external edit in place before and after the pre-rename check [UT-072]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		_, edit := f.approvedEditOverBase(t, svc)
		skillFile := filepath.Join(f.root, skillEditName, "SKILL.md")
		external := func(step, content string) func(string) bool {
			return func(current string) bool {
				if current == step {
					if err := os.WriteFile(skillFile, []byte(content), 0o600); err != nil {
						t.Error(err)
					}
				}
				return false
			}
		}
		_, err := f.revert(t, withHook(svc, external("incoming_ready", "external revert")), edit)
		requireRevertConflict(t, err, SkillRevertChangedOnDisk)
		if data, _ := readSkillFileMode(t, skillFile); string(data) != "external revert" {
			t.Fatalf("external content = %q", data)
		}
		f.requireClean(t)
		requireReviewState(t, svc, edit.ID, skillReviewApproved)

		h := newSkillTxFixture(t)
		approver := h.service(t, "home-a", nil)
		h.install(t, skillEditName, skillEditBase...)
		pending := h.mustProposeEdit(t, approver, "Another change.\n")
		approveFile := filepath.Join(h.root, skillEditName, "SKILL.md")
		racing := withHook(approver, func(step string) bool {
			if step == "incoming_ready" {
				if err := os.WriteFile(approveFile, []byte("external approve"), 0o600); err != nil {
					t.Error(err)
				}
			}
			return false
		})
		if _, err := h.approve(t, racing, pending); !errors.Is(err, ErrSkillReviewBaseStale) {
			t.Fatalf("Approve() with an external edit = %v, want skill_review_base_stale", err)
		}
		if data, _ := readSkillFileMode(t, approveFile); string(data) != "external approve" {
			t.Fatalf("external content = %q", data)
		}
		h.requireClean(t)
		requireReviewState(t, approver, pending.ID, SkillReviewPending)

		g := newSkillTxFixture(t)
		other := g.service(t, "home-a", nil)
		_, raced := g.approvedEditOverBase(t, other)
		racedFile := filepath.Join(g.root, skillEditName, "SKILL.md")
		hook := withHook(other, func(step string) bool {
			if step == "rechecked" {
				if err := os.WriteFile(racedFile, []byte("external rechecked"), 0o600); err != nil {
					t.Error(err)
				}
			}
			return false
		})
		if _, err := g.revert(t, hook, raced); !errors.Is(err, ErrSkillInstallConflict) {
			t.Fatalf("Revert() racing the move = %v, want skill_install_conflict", err)
		}
		if payload, ok := g.journal(t, skillEditName); !ok || payload.State != skillInstallStateConflict ||
			payload.Action != SkillInstallActionRevert {
			t.Fatalf("journal = %+v, %v; want a retained revert conflict", payload, ok)
		}
		if data, _ := readSkillFileMode(t, racedFile); string(data) != "external rechecked" {
			t.Fatalf("external content = %q", data)
		}
	})

	t.Run("Should read another home's approval as the current writer [UT-074]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		homeA := f.service(t, "home-a", nil)
		homeB := f.service(t, "home-b", nil)
		first := f.mustApprove(t, homeA, f.propose(t, homeA, skillEditName, "Check the changelog."))
		f.mustApprove(t, homeB, f.mustProposeEdit(t, homeB, "Check the calendar.\n"))
		if got := f.eligibility(t, homeA, first.ID); got.Reason != SkillRevertSuperseded {
			t.Fatalf("home A eligibility = %+v, want superseded", got)
		}
	})

	t.Run("Should order approvals of two homes by the install log [UT-077]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		homeA := f.service(t, "home-a", nil)
		homeB := f.service(t, "home-b", nil)
		first := f.mustApprove(t, homeA, f.propose(t, homeA, skillEditName, "Check the changelog."))
		f.mustApprove(t, homeB, f.mustProposeEdit(t, homeB, "Check the calendar.\n"))
		last := f.mustApprove(t, homeB, f.mustProposeEdit(t, homeB, "Check the changelog."))
		if writer := f.currentWriter(t, skillEditName); writer == nil || writer.ReviewID != last.ID ||
			writer.HomeID != "home-b" {
			t.Fatalf("current writer = %+v, want home B's last review", writer)
		}
		_, err := f.revert(t, homeA, first)
		requireRevertConflict(t, err, SkillRevertSuperseded)
	})

	t.Run("Should resolve a conflicted revert from another home either way [UT-078]", func(t *testing.T) {
		t.Parallel()
		for _, keep := range []string{SkillConflictKeepCurrent, SkillConflictKeepPrevious} {
			f := newSkillTxFixture(t)
			svc := f.service(t, "home-a", nil)
			_, edit := f.approvedEditOverBase(t, svc)
			installed := f.digest(t, skillEditName)
			skillFile := filepath.Join(f.root, skillEditName, "SKILL.md")
			hook := withHook(svc, func(step string) bool {
				if step == "rechecked" {
					if err := os.WriteFile(skillFile, []byte("external"), 0o600); err != nil {
						t.Error(err)
					}
				}
				return false
			})
			if _, err := f.revert(t, hook, edit); !errors.Is(err, ErrSkillInstallConflict) {
				t.Fatalf("Revert() = %v, want skill_install_conflict", err)
			}
			payload, ok := f.journal(t, skillEditName)
			if !ok || payload.OriginalSource != skillOriginalInCopy {
				t.Fatalf("journal = %+v, %v; want the original in the verified copy", payload, ok)
			}
			external := f.digest(t, skillEditName)
			result, err := f.resolve(t, f.service(t, "home-b", nil), payload.OpID, keep)
			if err != nil || result.AlreadyResolved {
				t.Fatalf("ResolveInstallConflict(%s) = %+v, %v", keep, result, err)
			}
			writer := f.currentWriter(t, skillEditName)
			switch keep {
			case SkillConflictKeepCurrent:
				if f.digest(t, skillEditName) != external || writer == nil || writer.ReviewID != "" {
					t.Fatalf("keep=current left %s with writer %+v", f.digest(t, skillEditName), writer)
				}
			default:
				if f.digest(t, skillEditName) != installed || writer == nil || writer.ReviewID != edit.ID {
					t.Fatalf("keep=previous left %s with writer %+v", f.digest(t, skillEditName), writer)
				}
				if f.digest(t, skillConflictCopyPrefix+payload.OpID) != external {
					t.Fatal("the external edit is not in the keep-copy directory")
				}
			}
		}
	})

	t.Run("Should make the predecessor revertible again after reverting its successor [UT-079]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		other := f.withInstance("inst-b")
		first := f.mustApprove(t, svc, f.propose(t, svc, skillEditName, "Check the changelog."))
		v1 := f.digest(t, skillEditName)
		second := other.mustApprove(t, svc, other.mustProposeEdit(t, svc, "Check the calendar.\n"))
		var previous SkillInstallMarker
		if err := json.Unmarshal(second.PrevInstallMarker, &previous); err != nil || previous.ReviewID != first.ID {
			t.Fatalf("prev_install_marker = %s, %v; want the first review", second.PrevInstallMarker, err)
		}
		other.mustRevert(t, svc, second)
		if f.digest(t, skillEditName) != v1 {
			t.Fatal("v1 was not restored")
		}
		if writer := f.currentWriter(t, skillEditName); writer == nil || *writer != previous {
			t.Fatalf("current writer = %+v, want %+v", writer, previous)
		}
		if got := f.eligibility(t, svc, first.ID); !got.Allowed {
			t.Fatalf("first eligibility = %+v, want allowed", got)
		}

		g := newSkillTxFixture(t)
		homeA := g.service(t, "home-a", nil)
		homeB := g.service(t, "home-b", nil)
		firstA := g.mustApprove(t, homeA, g.propose(t, homeA, skillEditName, "Check the changelog."))
		secondB := g.mustApprove(t, homeB, g.mustProposeEdit(t, homeB, "Check the calendar.\n"))
		g.mustRevert(t, homeB, secondB)
		if got := g.eligibility(t, homeA, firstA.ID); !got.Allowed {
			t.Fatalf("home A eligibility after home B's revert = %+v, want allowed", got)
		}
		g.mustRevert(t, homeA, firstA)
		if g.digest(t, skillEditName) != "" {
			t.Fatal("reverting the restored first approval left the skill installed")
		}
	})

	t.Run("Should return an edit of an operator-authored skill to no writer [UT-080]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		base, edit := f.approvedEditOverBase(t, svc)
		if edit.PrevInstallMarker != nil {
			t.Fatalf("prev_install_marker = %s, want none", edit.PrevInstallMarker)
		}
		f.mustRevert(t, svc, edit)
		f.requireBaseRestored(t, base)
		if writer := f.currentWriter(t, skillEditName); writer != nil {
			t.Fatalf("current writer = %+v, want none (operator-authored)", writer)
		}
	})
}
