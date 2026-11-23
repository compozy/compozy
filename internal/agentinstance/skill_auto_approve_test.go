package agentinstance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const autoSkillName = "release-checklist"

// autoApproveToggle is the instance's skill_auto_approve as the store reads it.
type autoApproveToggle struct{ on atomic.Bool }

func (f *skillTxFixture) autoService(t *testing.T, toggle *autoApproveToggle) *SkillProposalService {
	t.Helper()
	svc := f.service(t, "home-a", nil)
	svc.AutoApprove = func(context.Context, string) (bool, error) { return toggle.on.Load(), nil }
	return svc
}

// autoInput is a high-confidence proposal of name; files are draft files (new) or the overlay (edit).
func autoInput(name, body string, mode SkillProposalMode, files ...SkillDraftFile) SkillProposalInput {
	return SkillProposalInput{
		Name: name, Description: "Use before tagging a release.", Body: body, Mode: mode, Files: files,
		Lesson: SkillLesson{
			Cue: "Tagging a release", Lesson: "Check the changelog first.", Evidence: "sess-a missed it.",
			Confidence: skillConfidenceHigh,
		},
		EvidenceSessionIDs: []string{"sess-a"},
	}
}

func (f *skillTxFixture) proposeAuto(
	t *testing.T,
	svc *SkillProposalService,
	input SkillProposalInput,
) SkillProposalResult {
	t.Helper()
	result, err := svc.Propose(t.Context(), skillOperator, f.instance, f.root, input)
	if err != nil {
		t.Fatalf("Propose(%s) = %v", input.Name, err)
	}
	return result
}

func requireAutoApproved(t *testing.T, svc *SkillProposalService, result SkillProposalResult) {
	t.Helper()
	review := result.Review
	if result.AutoApproveSkipped != "" || review.State != skillReviewApproved ||
		review.DecidedVia != skillDecidedViaAutoApprove || review.DecidedBy != skillDecidedByAutoApprove {
		t.Fatalf("Propose() = %+v (skipped %q), want approved automatically", review, result.AutoApproveSkipped)
	}
	stored := requireReviewState(t, svc, review.ID, skillReviewApproved)
	if stored.DecidedVia != skillDecidedViaAutoApprove || stored.InstalledDigest == "" {
		t.Fatalf("stored review = %+v, want an auto_approve projection", stored)
	}
}

func requireAutoSkipped(t *testing.T, svc *SkillProposalService, result SkillProposalResult, reason string) {
	t.Helper()
	if result.AutoApproveSkipped != reason || result.Review.State != SkillReviewPending {
		t.Fatalf("Propose() = %+v (skipped %q), want pending with %s",
			result.Review, result.AutoApproveSkipped, reason)
	}
	requireReviewState(t, svc, result.Review.ID, SkillReviewPending)
}

func (f *skillTxFixture) installLogLen(t *testing.T, name string) int {
	t.Helper()
	entries, err := readSkillInstallLog(f.root, name)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func (f *skillTxFixture) requireSkillAbsent(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(f.root, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("skill %s exists (%v), want no files written", name, err)
	}
	if n := f.installLogLen(t, name); n != 0 {
		t.Fatalf("install log of %s has %d entries, want none", name, n)
	}
}

// ownSkill installs name through an operator approval of this instance's review, so the install log's
// current writer is an approved review of this instance.
func (f *skillTxFixture) ownSkill(
	t *testing.T,
	svc *SkillProposalService,
	toggle *autoApproveToggle,
	name string,
	files ...SkillDraftFile,
) SkillReview {
	t.Helper()
	was := toggle.on.Swap(false)
	defer toggle.on.Store(was)
	result := f.proposeAuto(t, svc, autoInput(name, "1. Run make gate.\n", SkillProposalNew, files...))
	return f.mustApprove(t, svc, result.Review)
}

// seedAutoApprovals stores decided reviews of the fixture instance, as earlier automatic approvals.
func (f *skillTxFixture) seedAutoApprovals(svc *SkillProposalService, rows ...SkillReview) {
	store := skillStore(svc)
	store.mu.Lock()
	defer store.mu.Unlock()
	for i, row := range rows {
		row.ID = "skr-seed-" + string(rune('a'+i))
		if row.InstanceID == "" {
			row.InstanceID = f.instance.ID
		}
		store.reviews[row.ID] = row
	}
}

func autoDecided(state, via string, at time.Time) SkillReview {
	return SkillReview{State: state, DecidedVia: via, DecidedAt: &at}
}

func fixedClock(svc *SkillProposalService, at string) {
	now, err := time.Parse(time.RFC3339, at)
	if err != nil {
		panic(err)
	}
	svc.Now = func() time.Time { return now }
}

// Invariant: a proposal is approved automatically only when the instance's toggle is on, the lesson
// is high confidence, the resulting tree runs nothing automatically, an edit changes a skill whose
// install-log current writer is an approved review of this instance and is not stale, and the
// instance has fewer than 3 automatic approvals in the current UTC day; the first failing check is
// reported and the review stays pending. Every propose serializes on the per-instance mutex.
// Owning layer: agentinstance proposal service; canonical suite for auto-approve.
func TestSkillAutoApprove(t *testing.T) {
	t.Parallel()

	t.Run("Should install an eligible new skill automatically [UT-022]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		result := f.proposeAuto(t, svc, autoInput(autoSkillName, "1. Run make gate.\n", SkillProposalNew))
		requireAutoApproved(t, svc, result)
		writer := f.currentWriter(t, autoSkillName)
		if writer == nil || writer.ReviewID != result.Review.ID || writer.DecidedVia != skillDecidedViaAutoApprove ||
			writer.DecidedBy != skillDecidedByAutoApprove {
			t.Fatalf("current writer = %+v, want the automatic approval", writer)
		}
		if _, err := os.Stat(filepath.Join(f.root, autoSkillName, "SKILL.md")); err != nil {
			t.Fatalf("installed SKILL.md: %v", err)
		}
		f.requireClean(t)
	})

	t.Run("Should install an eligible edit of a skill this instance installed [UT-023]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		f.ownSkill(t, svc, toggle, autoSkillName)
		result := f.proposeAuto(t, svc, autoInput(autoSkillName, "1. Run make gate.\n2. Check the freeze.\n",
			SkillProposalEdit))
		requireAutoApproved(t, svc, result)
		if got := f.digest(t, autoSkillName); got != result.Review.ProposedDigest {
			t.Fatalf("installed digest = %s, want the proposed %s", got, result.Review.ProposedDigest)
		}
	})

	t.Run("Should skip anything that runs automatically in the resulting tree [UT-024]", func(t *testing.T) {
		t.Parallel()
		script := SkillDraftFile{Path: "scripts/check.sh", Content: "#!/bin/sh\necho check\n"}
		mcp := SkillDraftFile{Path: "mcp.json", Content: `{"mcpServers":{"linear":{"command":"echo"}}}`}
		hook := SkillDraftFile{
			Path:    "extra/SKILL.md",
			Content: "---\nname: extra\ndescription: Extra\nmetadata:\n  compozy:\n    hooks:\n      - event: x\n---\n\nbody",
		}
		for _, tc := range []struct {
			name  string
			base  []SkillDraftFile // nil: propose a new skill
			input SkillProposalInput
			want  string
			setup func(t *testing.T, f *skillTxFixture)
		}{
			{name: "Should skip a new skill with a script (a)",
				input: autoInput(autoSkillName, "body\n", SkillProposalNew, script), want: SkillAutoApproveRunsAutomatically},
			{name: "Should skip a body-only edit of a skill with a script (b)", base: []SkillDraftFile{script},
				input: autoInput(autoSkillName, "new body\n", SkillProposalEdit), want: SkillAutoApproveRunsAutomatically},
			{name: "Should skip a body-only edit of a skill with an MCP file (c)", base: []SkillDraftFile{mcp},
				input: autoInput(autoSkillName, "new body\n", SkillProposalEdit), want: SkillAutoApproveRunsAutomatically},
			{name: "Should skip an edit whose overlay adds a hook (d)", base: []SkillDraftFile{},
				input: autoInput(autoSkillName, "new body\n", SkillProposalEdit, hook),
				want:  SkillAutoApproveRunsAutomatically},
			{name: "Should approve an edit that removes the only script (e)", base: []SkillDraftFile{script},
				input: func() SkillProposalInput {
					input := autoInput(autoSkillName, "new body\n", SkillProposalEdit)
					input.RemovePaths = []string{"scripts/check.sh"}
					return input
				}()},
			{name: "Should fail closed on a malformed MCP file kept from the base (f)", base: []SkillDraftFile{},
				input: autoInput(autoSkillName, "new body\n", SkillProposalEdit), want: SkillAutoApproveRunsAutomatically,
				setup: func(t *testing.T, f *skillTxFixture) {
					path := filepath.Join(f.root, autoSkillName, "mcp.json")
					if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
						t.Fatal(err)
					}
				}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
				toggle.on.Store(true)
				svc := f.autoService(t, toggle)
				if tc.base != nil {
					f.ownSkill(t, svc, toggle, autoSkillName, tc.base...)
				}
				if tc.setup != nil {
					tc.setup(t, f)
				}
				result := f.proposeAuto(t, svc, tc.input)
				if tc.want == "" {
					requireAutoApproved(t, svc, result)
					return
				}
				requireAutoSkipped(t, svc, result, tc.want)
			})
		}
	})

	t.Run("Should report toggled_off first and then the reasons in order [UT-088]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		svc := f.autoService(t, toggle)
		input := autoInput(autoSkillName, "body\n", SkillProposalNew,
			SkillDraftFile{Path: "scripts/run.sh", Content: "#!/bin/sh\n"})
		input.Lesson.Confidence = "low"
		requireAutoSkipped(t, svc, f.proposeAuto(t, svc, input), SkillAutoApproveToggledOff)
		toggle.on.Store(true)
		input.Name = "release-checklist-two"
		requireAutoSkipped(t, svc, f.proposeAuto(t, svc, input), SkillAutoApproveLowConfidence)
	})

	t.Run("Should skip confidence below high [UT-047]", func(t *testing.T) {
		t.Parallel()
		for _, confidence := range []string{"", "low", "medium"} {
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			input := autoInput(autoSkillName, "body\n", SkillProposalNew)
			input.Lesson.Confidence = confidence
			requireAutoSkipped(t, svc, f.proposeAuto(t, svc, input), SkillAutoApproveLowConfidence)
			f.requireSkillAbsent(t, autoSkillName)
		}
	})

	t.Run("Should skip an edit of a skill this instance did not install [UT-025]", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name    string
			install func(t *testing.T, f *skillTxFixture, svc *SkillProposalService, toggle *autoApproveToggle)
		}{
			{name: "Should skip an operator-authored skill",
				install: func(t *testing.T, f *skillTxFixture, _ *SkillProposalService, _ *autoApproveToggle) {
					f.install(t, autoSkillName, skillTestFile{path: "SKILL.md", content: "---\nname: a\n---\n\nx", mode: 0o644})
				}},
			{name: "Should skip a skill another instance installed",
				install: func(t *testing.T, f *skillTxFixture, svc *SkillProposalService, toggle *autoApproveToggle) {
					f.withInstance("inst-b").ownSkill(t, svc, toggle, autoSkillName)
				}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
				toggle.on.Store(true)
				svc := f.autoService(t, toggle)
				tc.install(t, f, svc, toggle)
				result := f.proposeAuto(t, svc, autoInput(autoSkillName, "new body\n", SkillProposalEdit))
				requireAutoSkipped(t, svc, result, SkillAutoApproveOperatorAuthored)
			})
		}
	})

	t.Run("Should treat a conflict kept as current as operator-authored [UT-025a]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		r1 := f.ownSkill(t, svc, toggle, autoSkillName)
		op := f.replaceOp(t, autoSkillName)
		op.prev = f.currentWriter(t, autoSkillName)
		if err := f.runOp(t, crashAt(svc, "target_moved"), autoSkillName, op); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		f.writeTree(t, autoSkillName, "---\nname: release-checklist\ndescription: Edited\n---\n\nedited")
		f.reconcile(t, svc)
		payload, ok := f.journal(t, autoSkillName)
		if !ok || payload.State != skillInstallStateConflict {
			t.Fatalf("journal = %+v, %v; want a retained conflict", payload, ok)
		}
		if _, err := f.resolve(t, svc, payload.OpID, SkillConflictKeepCurrent); err != nil {
			t.Fatalf("ResolveInstallConflict(current) = %v", err)
		}
		result := f.proposeAuto(t, svc, autoInput(autoSkillName, "new body\n", SkillProposalEdit))
		if result.Review.BaseDigest != f.digest(t, autoSkillName) {
			t.Fatal("the proposal's base does not match the live tree")
		}
		requireAutoSkipped(t, svc, result, SkillAutoApproveOperatorAuthored)
		_, err := f.revert(t, svc, r1)
		requireRevertConflict(t, err, SkillRevertSuperseded)
	})

	t.Run("Should treat an externally edited own skill as operator-authored [UT-025b D-01]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		f.ownSkill(t, svc, toggle, autoSkillName)
		edited := f.writeTree(t, autoSkillName, "---\nname: release-checklist\ndescription: Operator\n---\n\nmine")
		result := f.proposeAuto(t, svc, autoInput(autoSkillName, "new body\n", SkillProposalEdit))
		if result.Review.BaseDigest != edited {
			t.Fatalf("base digest = %s, want the live tree %s", result.Review.BaseDigest, edited)
		}
		requireAutoSkipped(t, svc, result, SkillAutoApproveOperatorAuthored)
		if f.digest(t, autoSkillName) != edited {
			t.Fatal("a skipped proposal changed the operator's contents")
		}
	})

	t.Run("Should stop at the toggle read under the instance mutex [UT-026]", func(t *testing.T) {
		t.Parallel()
		t.Run("Should write nothing when the toggle is turned off before evaluation", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			release, err := svc.Locks.lock(t.Context(), "instance:"+f.instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan SkillProposalResult, 1)
			go func() {
				result, _ := svc.Propose(context.Background(), skillOperator, f.instance, f.root,
					autoInput(autoSkillName, "body\n", SkillProposalNew))
				done <- result
			}()
			waitInstanceWaiters(t, svc, f.instance.ID, 2)
			toggle.on.Store(false)
			release()
			result := <-done
			requireAutoSkipped(t, svc, result, SkillAutoApproveToggledOff)
			f.requireSkillAbsent(t, autoSkillName)
		})
		t.Run("Should finish an install whose toggle is turned off after the check", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			svc.onStep = func(step string, _ *skillTarget) {
				if step == "auto_approve_eligible" {
					toggle.on.Store(false)
				}
			}
			requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput(autoSkillName, "body\n", SkillProposalNew)))
		})
		t.Run("Should roll a crashed new install back without a backup rename (N-005)", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			result := f.proposeAuto(
				t,
				crashAt(svc, "target_moved"),
				autoInput(autoSkillName, "body\n", SkillProposalNew),
			)
			requireAutoSkipped(t, svc, result, SkillAutoApproveInstallFailed)
			payload, ok := f.journal(t, autoSkillName)
			if !ok || payload.PreExists {
				t.Fatalf("journal = %+v, %v; want an install of a new skill left for Reconcile", payload, ok)
			}
			f.reconcile(t, f.service(t, "home-a", skillStore(svc)))
			f.requireSkillAbsent(t, autoSkillName)
			f.requireClean(t)
			requireReviewState(t, svc, result.Review.ID, SkillReviewPending)
		})
	})

	t.Run("Should keep the review pending when the install fails or the base moved [UT-027]", func(t *testing.T) {
		t.Parallel()
		t.Run("Should report install_failed for an unwritable target root", func(t *testing.T) {
			t.Parallel()
			if os.Geteuid() == 0 {
				t.Skip("root ignores directory permissions")
			}
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			if err := os.MkdirAll(f.root, 0o700); err != nil {
				t.Fatal(err)
			}
			svc.onStep = func(step string, _ *skillTarget) {
				if step == "auto_approve_eligible" {
					if err := os.Chmod(f.root, 0o500); err != nil {
						t.Error(err)
					}
				}
			}
			t.Cleanup(func() { _ = os.Chmod(f.root, 0o700) })
			result := f.proposeAuto(t, svc, autoInput(autoSkillName, "body\n", SkillProposalNew))
			requireAutoSkipped(t, svc, result, SkillAutoApproveInstallFailed)
			if err := os.Chmod(f.root, 0o700); err != nil {
				t.Fatal(err)
			}
			f.requireSkillAbsent(t, autoSkillName)
		})
		t.Run("Should report base_stale when the installed skill changed after capture", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			f.ownSkill(t, svc, toggle, autoSkillName)
			// The base is captured over an external edit; before evaluation the installed contents of
			// this instance's review come back, so provenance holds and only the base is stale.
			path := filepath.Join(f.root, autoSkillName, "SKILL.md")
			installed, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f.writeTree(t, autoSkillName, "---\nname: release-checklist\ndescription: x\n---\n\nexternal")
			svc.onStep = func(step string, _ *skillTarget) {
				if step == "auto_approve_eval" {
					if err := os.WriteFile(path, installed, 0o600); err != nil {
						t.Error(err)
					}
				}
			}
			result := f.proposeAuto(t, svc, autoInput(autoSkillName, "new body\n", SkillProposalEdit))
			requireAutoSkipped(t, svc, result, SkillAutoApproveBaseStale)
		})
	})

	t.Run("Should cap automatic approvals at 3 per UTC day by commit time [UT-085]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		fixedClock(svc, "2026-11-22T23:59:00Z")
		today := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
		f.seedAutoApprovals(svc,
			autoDecided(skillReviewApproved, skillDecidedViaAutoApprove, today),
			autoDecided(SkillReviewReverted, skillDecidedViaAutoApprove, today), // a revert never refunds
			autoDecided(skillReviewApproved, skillDecidedViaOperator, today),
			autoDecided(skillReviewApproved, skillDecidedViaAutoApprove,
				time.Date(2026, 11, 21, 23, 59, 59, 0, time.UTC)),
		)
		requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-three", "body\n", SkillProposalNew)))
		capped := f.proposeAuto(t, svc, autoInput("skill-four", "body\n", SkillProposalNew))
		requireAutoSkipped(t, svc, capped, SkillAutoApproveDailyCapReached)
		f.requireSkillAbsent(t, "skill-four")
		low := autoInput("skill-low", "body\n", SkillProposalNew)
		low.Lesson.Confidence = "low"
		requireAutoSkipped(t, svc, f.proposeAuto(t, svc, low), SkillAutoApproveLowConfidence)
		fixedClock(svc, "2026-11-23T00:00:00Z")
		requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-five", "body\n", SkillProposalNew)))
	})

	t.Run("Should count an uncertain automatic commit against the daily cap [UT-085] [N-03]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-one", "body\n", SkillProposalNew)))
		requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-two", "body\n", SkillProposalNew)))
		uncertain := withHook(svc, func(step string) bool {
			return step == "published_unsynced" || step == "published_sync_failed"
		})
		third := f.proposeAuto(t, uncertain, autoInput("skill-three", "body\n", SkillProposalNew))
		requireAutoSkipped(t, svc, third, SkillAutoApproveInstallFailed)
		capped := f.proposeAuto(t, svc, autoInput("skill-four", "body\n", SkillProposalNew))
		requireAutoSkipped(t, svc, capped, SkillAutoApproveDailyCapReached)
		f.requireSkillAbsent(t, "skill-four")
	})

	t.Run("Should fail the cap closed while an automatic approval is unprojected [N-01]", func(t *testing.T) {
		t.Parallel()
		setFailures := func(svc *SkillProposalService, n int) {
			store := skillStore(svc)
			store.mu.Lock()
			store.approvalFailures = n
			store.mu.Unlock()
		}
		t.Run("Should retry a failed projection once synchronously", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			setFailures(svc, 1)
			requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-one", "body\n", SkillProposalNew)))
			requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-two", "body\n", SkillProposalNew)))
		})
		t.Run("Should report daily_cap until Reconcile projects the approval", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			setFailures(svc, 2)
			first := f.proposeAuto(t, svc, autoInput("skill-one", "body\n", SkillProposalNew))
			if first.AutoApproveSkipped != "" || first.Review.State != skillReviewApproved {
				t.Fatalf("Propose() = %+v (skipped %q), want a committed approval", first.Review,
					first.AutoApproveSkipped)
			}
			requireReviewState(t, svc, first.Review.ID, SkillReviewPending)
			capped := f.proposeAuto(t, svc, autoInput("skill-two", "body\n", SkillProposalNew))
			requireAutoSkipped(t, svc, capped, SkillAutoApproveDailyCapReached)
			f.requireSkillAbsent(t, "skill-two")
			f.reconcile(t, svc)
			requireReviewState(t, svc, first.Review.ID, skillReviewApproved)
			requireAutoApproved(t, svc, f.proposeAuto(t, svc, autoInput("skill-three", "body\n", SkillProposalNew)))
		})
	})

	t.Run("Should report install_failed for a new skill whose install log is unreadable [N-02]", func(t *testing.T) {
		t.Parallel()
		f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
		toggle.on.Store(true)
		svc := f.autoService(t, toggle)
		// The proposal's own Reconcile reads the log first, so it breaks between Reconcile and evaluation.
		path := skillInstallLogPath(f.root, autoSkillName)
		svc.onStep = func(step string, _ *skillTarget) {
			if step != "auto_approve_eval" {
				return
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Error(err)
			}
			if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
				t.Error(err)
			}
		}
		result := f.proposeAuto(t, svc, autoInput(autoSkillName, "body\n", SkillProposalNew))
		requireAutoSkipped(t, svc, result, SkillAutoApproveInstallFailed)
		// The corrupt log cannot be decoded, so check it directly: no files and nothing appended.
		if _, err := os.Lstat(filepath.Join(f.root, autoSkillName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("skill %s exists (%v), want no files written", autoSkillName, err)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != "not json\n" {
			t.Fatalf("install log = %q, %v; want the corrupt log unchanged", data, err)
		}
	})

	t.Run("Should serialize the cap of one instance across skills [UT-086]", func(t *testing.T) {
		t.Parallel()
		race := func(t *testing.T, svc *SkillProposalService, f *skillTxFixture, start func()) []string {
			t.Helper()
			var wg sync.WaitGroup
			reasons := make([]string, 2)
			barrier := make(chan struct{})
			for i, name := range []string{"skill-left", "skill-right"} {
				wg.Go(func() {
					<-barrier
					result, err := svc.Propose(context.Background(), skillOperator, f.instance, f.root,
						autoInput(name, "body\n", SkillProposalNew))
					if err != nil {
						t.Error(err)
					}
					reasons[i] = result.AutoApproveSkipped
				})
			}
			close(barrier)
			start()
			wg.Wait()
			slices.Sort(reasons)
			return reasons
		}
		seed := func(f *skillTxFixture, svc *SkillProposalService) {
			now := time.Now().UTC()
			f.seedAutoApprovals(svc,
				autoDecided(skillReviewApproved, skillDecidedViaAutoApprove, now),
				autoDecided(skillReviewApproved, skillDecidedViaAutoApprove, now))
		}
		t.Run("Should approve exactly one of two eligible proposals at count 2 (a)", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			toggle.on.Store(true)
			svc := f.autoService(t, toggle)
			seed(f, svc)
			if got := race(t, svc, f, func() {}); !slices.Equal(got, []string{"", SkillAutoApproveDailyCapReached}) {
				t.Fatalf("skip reasons = %q, want one approval and one daily_cap", got)
			}
		})
		t.Run("Should read the toggle under the mutex after it is turned on (b)", func(t *testing.T) {
			t.Parallel()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			svc := f.autoService(t, toggle)
			seed(f, svc)
			release, err := svc.Locks.lock(t.Context(), "instance:"+f.instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			got := race(t, svc, f, func() {
				waitInstanceWaiters(t, svc, f.instance.ID, 3)
				toggle.on.Store(true)
				release()
			})
			if !slices.Equal(got, []string{"", SkillAutoApproveDailyCapReached}) {
				t.Fatalf("skip reasons = %q, want one approval and one daily_cap, never toggled_off", got)
			}
			count, err := svc.Store.CountAutoApprovals(
				t.Context(),
				f.instance.ID,
				time.Now().UTC().Truncate(24*time.Hour),
			)
			if err != nil || count != SkillAutoApproveDailyCap {
				t.Fatalf("automatic approvals today = %d, %v; want 3", count, err)
			}
		})
		t.Run("Should take the instance mutex before the skill locks on every propose (c)", func(t *testing.T) {
			t.Parallel()
			for _, on := range []bool{false, true} {
				f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
				toggle.on.Store(on)
				svc := f.autoService(t, toggle)
				var mu sync.Mutex
				var steps []string
				record := func(step string) {
					mu.Lock()
					defer mu.Unlock()
					steps = append(steps, step)
				}
				svc.onStep = func(step string, _ *skillTarget) { record(step) }
				svc.AutoApprove = func(context.Context, string) (bool, error) {
					record("toggle_read")
					return toggle.on.Load(), nil
				}
				f.proposeAuto(t, svc, autoInput(autoSkillName, "body\n", SkillProposalNew))
				want := []string{"instance_locked", "skill_locked", "auto_approve_eval", "toggle_read"}
				if !slices.Equal(steps[:len(want)], want) {
					t.Fatalf("toggle %v: steps = %q, want prefix %q", on, steps, want)
				}
			}
		})
	})

	t.Run("Should serialize an automatic and a manual approval of one skill [UT-060]", func(t *testing.T) {
		t.Parallel()
		setup := func(t *testing.T) (*skillTxFixture, *autoApproveToggle, *SkillProposalService, SkillReview) {
			t.Helper()
			f, toggle := newSkillTxFixture(t), &autoApproveToggle{}
			svc := f.autoService(t, toggle)
			f.ownSkill(t, svc, toggle, autoSkillName)
			manual := f.proposeAuto(t, svc, autoInput(autoSkillName, "manual body\n", SkillProposalEdit))
			requireAutoSkipped(t, svc, manual, SkillAutoApproveToggledOff)
			toggle.on.Store(true)
			return f, toggle, svc, manual.Review
		}
		t.Run("Should make the manual approval stale when the automatic one wins", func(t *testing.T) {
			t.Parallel()
			f, _, svc, manual := setup(t)
			auto := f.proposeAuto(t, svc, autoInput(autoSkillName, "auto body\n", SkillProposalEdit))
			requireAutoApproved(t, svc, auto)
			if _, err := f.approve(t, svc, manual); !errors.Is(err, ErrSkillReviewBaseStale) {
				t.Fatalf("Approve(manual) = %v, want skill_review_base_stale", err)
			}
		})
		t.Run("Should leave the automatic proposal base_stale when the manual one wins", func(t *testing.T) {
			t.Parallel()
			f, _, svc, manual := setup(t)
			svc.onStep = func(step string, target *skillTarget) {
				if step != "auto_approve_eval" {
					return
				}
				_, draft, err := svc.Review(context.Background(), skillOperator, f.instance, manual.ID)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := svc.approveLocked(context.Background(), target, f.instance, manual.ID, manual.Digest,
					"operator:prof-a", draft); err != nil {
					t.Error(err)
				}
			}
			auto := f.proposeAuto(t, svc, autoInput(autoSkillName, "auto body\n", SkillProposalEdit))
			requireAutoSkipped(t, svc, auto, SkillAutoApproveBaseStale)
			requireReviewState(t, svc, manual.ID, skillReviewApproved)
		})
	})
}

// waitInstanceWaiters waits until n callers hold or wait for the instance mutex.
func waitInstanceWaiters(t *testing.T, svc *SkillProposalService, instanceID string, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		svc.Locks.mu.Lock()
		entry := svc.Locks.held["instance:"+instanceID]
		refs := 0
		if entry != nil {
			refs = entry.refs
		}
		svc.Locks.mu.Unlock()
		if refs >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("instance mutex waiters never reached %d", n)
}
