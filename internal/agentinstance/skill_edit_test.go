package agentinstance

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/fileutil"
)

const skillEditName = "release-checklist"

var skillEditBase = []skillTestFile{
	{
		path:    "SKILL.md",
		content: "---\nname: release-checklist\ndescription: Base\n---\n\n1. Run make gate.\n",
		mode:    0o644,
	},
	{path: "scripts/check.sh", content: "#!/bin/sh\necho check\n", mode: 0o755},
	{path: "assets/flow.png", content: "\x89PNG\r\n\x1a\n\x00\xff\xfe", mode: 0o600},
}

func (f *skillTxFixture) proposeEdit(
	t *testing.T,
	svc *SkillProposalService,
	body string,
	mutate func(*SkillProposalInput),
) (SkillProposalResult, error) {
	t.Helper()
	input := SkillProposalInput{
		Name: skillEditName, Description: "Use before tagging a release.", Body: body, Mode: SkillProposalEdit,
		Lesson: SkillLesson{
			Cue: "Tagging a release", Lesson: "Also confirm the freeze calendar.", Evidence: "sess-a was blocked.",
		},
		EvidenceSessionIDs: []string{"sess-a"},
	}
	if mutate != nil {
		mutate(&input)
	}
	return svc.Propose(t.Context(), skillOperator, f.instance, f.root, input)
}

func (f *skillTxFixture) mustProposeEdit(t *testing.T, svc *SkillProposalService, body string) SkillReview {
	t.Helper()
	result, err := f.proposeEdit(t, svc, body, nil)
	if err != nil {
		t.Fatalf("Propose(edit) = %v", err)
	}
	return result.Review
}

// linkSkillOutside moves the installed skill name to a directory outside the target root and leaves
// a symlink to it at the skill path. It returns the outside directory.
func (f *skillTxFixture) linkSkillOutside(t *testing.T, name string) string {
	t.Helper()
	outside := filepath.Join(t.TempDir(), name)
	if err := os.Rename(filepath.Join(f.root, name), outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.root, name)); err != nil {
		t.Fatal(err)
	}
	return outside
}

// requireLinkedOutside checks that the skill path is still the symlink and its target is untouched.
func (f *skillTxFixture) requireLinkedOutside(t *testing.T, name, outside string) {
	t.Helper()
	if target, err := os.Readlink(filepath.Join(f.root, name)); err != nil || target != outside {
		t.Fatalf("skill link = %q, %v; want %q", target, err, outside)
	}
	data, mode := readSkillFileMode(t, filepath.Join(outside, "scripts", "check.sh"))
	if string(data) != skillEditBase[1].content || mode != skillEditBase[1].mode {
		t.Fatalf("outside directory changed: %q %o", data, mode)
	}
}

func requireNoReviews(t *testing.T, svc *SkillProposalService) {
	t.Helper()
	if reviews := skillStore(svc).reviews; len(reviews) != 0 {
		t.Fatalf("refused proposal created reviews: %+v", reviews)
	}
	if entries, err := os.ReadDir(svc.DraftRoot); err != nil || len(entries) != 0 {
		t.Fatalf("refused proposal left drafts or snapshots: %d %v", len(entries), err)
	}
}

func readSkillFileMode(t *testing.T, path string) ([]byte, os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return data, info.Mode().Perm()
}

// Invariant: an edit is an overlay on the installed skill of the target root, captured as a lossless
// snapshot; approval installs the reviewed resulting tree only while the installed tree equals the base.
// Owning layer: agentinstance proposal service and install transaction; canonical suite for edits.
func TestSkillEditProposals(t *testing.T) {
	t.Parallel()

	t.Run("Should create a pending edit with the base digest and a lossless snapshot [UT-011]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		base := f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "1. Run make gate.\n2. Confirm the freeze calendar.\n")
		if review.Kind != SkillProposalEdit || review.State != SkillReviewPending || review.BaseDigest != base {
			t.Fatalf("review = %+v, base %s", review, base)
		}
		if review.SnapshotPath != filepath.Join(filepath.Dir(review.DraftPath), review.ID+".base") {
			t.Fatalf("snapshot path = %s", review.SnapshotPath)
		}
		within, err := fileutil.PathWithinRoot(f.root, review.SnapshotPath)
		if err != nil || within {
			t.Fatalf("snapshot inside the scan root: %v %v", within, err)
		}
		if digest, err := SkillTreeDigest(review.SnapshotPath); err != nil || digest != base {
			t.Fatalf("snapshot digest = %s %v, want %s", digest, err, base)
		}
		if review.ProposedDigest == "" || review.ProposedDigest == base {
			t.Fatalf("proposed digest = %s", review.ProposedDigest)
		}
	})

	t.Run("Should refuse an edit of a skill installed only in another scan root [UT-013]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		other := filepath.Join(t.TempDir(), "user-skills")
		svc := f.service(t, "home-a", nil)
		svc.ScanRoots = append(svc.ScanRoots, other)
		otherFixture := &skillTxFixture{root: other, instance: f.instance}
		otherFixture.install(t, skillEditName, skillEditBase...)
		if _, err := f.proposeEdit(t, svc, "Changed.\n", nil); !errors.Is(err, ErrSkillReviewBaseMissing) {
			t.Fatalf("Propose() = %v, want base missing", err)
		}
		requireNoReviews(t, svc)
	})

	t.Run("Should refuse an edit equal to the installed skill [UT-014]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		created := f.propose(t, svc, skillEditName, "1. Run make gate.\n")
		if _, err := f.approve(t, svc, created); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadDir(svc.DraftRoot)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.proposeEdit(t, svc, "1. Run make gate.\n", nil); !errors.Is(err, ErrSkillProposeNoChange) {
			t.Fatalf("Propose() = %v, want no change", err)
		}
		if after, err := os.ReadDir(svc.DraftRoot); err != nil || len(after) != len(before) {
			t.Fatalf("refused edit left files: %d -> %d %v", len(before), len(after), err)
		}
	})

	t.Run("Should refuse an installed skill with a symlink or too many files [UT-015]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		if err := os.Symlink("SKILL.md", filepath.Join(f.root, skillEditName, "link.md")); err != nil {
			t.Fatal(err)
		}
		svc := f.service(t, "home-a", nil)
		if _, err := f.proposeEdit(t, svc, "Changed.\n", nil); !errors.Is(err, ErrSkillReviewBaseUnsupported) {
			t.Fatalf("symlink: Propose() = %v", err)
		}
		requireNoReviews(t, svc)

		// The whole skill directory is a symlink to a directory outside the target root.
		linked := newSkillTxFixture(t)
		linked.install(t, skillEditName, skillEditBase...)
		outside := linked.linkSkillOutside(t, skillEditName)
		linkedSvc := linked.service(t, "home-a", nil)
		if _, err := linked.proposeEdit(t, linkedSvc, "Changed.\n", nil); !errors.Is(err,
			ErrSkillReviewBaseUnsupported) {
			t.Fatalf("symlinked skill directory: Propose() = %v, want skill_review_base_unsupported", err)
		}
		requireNoReviews(t, linkedSvc)
		linked.requireLinkedOutside(t, skillEditName, outside)

		many := newSkillTxFixture(t)
		files := append([]skillTestFile{}, skillEditBase...)
		for i := range maxSkillBaseFiles {
			files = append(
				files,
				skillTestFile{path: "refs/" + strings.Repeat("a", i+1) + ".md", content: "x", mode: 0o600},
			)
		}
		many.install(t, skillEditName, files...)
		manySvc := many.service(t, "home-a", nil)
		if _, err := many.proposeEdit(t, manySvc, "Changed.\n", nil); !errors.Is(err, ErrSkillReviewBaseUnsupported) {
			t.Fatalf("too many files: Propose() = %v", err)
		}
	})

	t.Run("Should refuse agent-reachable overlay inputs with skill_review_invalid [UT-015]", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name   string
			mutate func(*SkillProposalInput)
		}{
			{"case-only collision", func(input *SkillProposalInput) {
				input.Files = []SkillDraftFile{{Path: "Scripts/check.sh", Content: "echo other\n"}}
			}},
			{"file over a base directory", func(input *SkillProposalInput) {
				input.Files = []SkillDraftFile{{Path: "scripts", Content: "not a directory\n"}}
			}},
			{"unknown mode", func(input *SkillProposalInput) { input.Mode = "replace" }},
		} {
			f := newSkillTxFixture(t)
			f.install(t, skillEditName, skillEditBase...)
			svc := f.service(t, "home-a", nil)
			if _, err := f.proposeEdit(t, svc, "Changed.\n", tc.mutate); !errors.Is(err, ErrSkillReviewInvalid) {
				t.Fatalf("%s: Propose() = %v, want skill_review_invalid", tc.name, err)
			}
			requireNoReviews(t, svc)
		}
	})

	t.Run("Should install the reviewed tree when the base still matches [UT-016]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "1. Run make gate.\n2. Confirm the freeze calendar.\n")
		result, err := f.approve(t, svc, review)
		if err != nil || !result.Committed || result.Warning != nil {
			t.Fatalf("Approve() = %+v %v", result, err)
		}
		got, _ := skillStore(svc).GetReview(t.Context(), review.ID)
		live := f.digest(t, skillEditName)
		if got.State != "approved" || got.InstalledDigest != live || live != review.ProposedDigest {
			t.Fatalf("review = %+v, live %s", got, live)
		}
		definition, mode := readSkillFileMode(t, filepath.Join(f.root, skillEditName, "SKILL.md"))
		if !strings.Contains(string(definition), "2. Confirm the freeze calendar.") || mode != 0o644 {
			t.Fatalf("SKILL.md = %q mode %o", definition, mode)
		}
		f.requireClean(t)
	})

	t.Run("Should refuse approval after the installed skill changed or was deleted [UT-017]", func(t *testing.T) {
		t.Parallel()
		for _, change := range []string{"modified", "deleted"} {
			f := newSkillTxFixture(t)
			f.install(t, skillEditName, skillEditBase...)
			svc := f.service(t, "home-a", nil)
			review := f.mustProposeEdit(t, svc, "Changed.\n")
			dir := filepath.Join(f.root, skillEditName)
			if change == "modified" {
				if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("operator"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			before, _ := skillTreeState(dir)
			if _, err := f.approve(t, svc, review); !errors.Is(err, ErrSkillReviewBaseStale) {
				t.Fatalf("%s: Approve() = %v, want base stale", change, err)
			}
			got, _ := skillStore(svc).GetReview(t.Context(), review.ID)
			if after, _ := skillTreeState(dir); after != before || got.State != SkillReviewPending {
				t.Fatalf("%s: live %s -> %s, state %s", change, before, after, got.State)
			}
			f.requireClean(t)
		}
	})

	t.Run("Should make the second of two pending edits stale after the first is approved [UT-018]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		first := f.mustProposeEdit(t, svc, "First change.\n")
		second := f.mustProposeEdit(t, svc, "Second change.\n")
		if first.BaseDigest != second.BaseDigest {
			t.Fatalf("bases differ: %s %s", first.BaseDigest, second.BaseDigest)
		}
		if _, err := f.approve(t, svc, first); err != nil {
			t.Fatal(err)
		}
		if _, err := f.approve(t, svc, second); !errors.Is(err, ErrSkillReviewBaseStale) {
			t.Fatalf("second Approve() = %v, want base stale", err)
		}
	})

	t.Run("Should keep binary and executable files of a body-only edit exactly [UT-052]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "1. Run make gate.\n2. Confirm the freeze calendar.\n")
		contents, err := ReadSkillReviewContents(review)
		if err != nil || contents.SnapshotProblem != nil {
			t.Fatalf("ReadSkillReviewContents() = %v %v", err, contents.SnapshotProblem)
		}
		for _, file := range skillEditBase[1:] {
			kept, ok := contents.Result.find(file.path)
			if !ok || !bytes.Equal(kept.Data, []byte(file.content)) || kept.Perm != file.mode {
				t.Fatalf("result %s = %+v", file.path, kept)
			}
		}
		proposedCounts, err := contents.Result.AutomaticContents()
		if err != nil {
			t.Fatal(err)
		}
		baseCounts, err := contents.Base.AutomaticContents()
		if err != nil || proposedCounts.Scripts != 1 || baseCounts.Scripts != 1 {
			t.Fatalf("scripts proposed=%d base=%d %v", proposedCounts.Scripts, baseCounts.Scripts, err)
		}
		want := []SkillTreeChange{
			{Path: "SKILL.md", Change: SkillChangeModified},
			{Path: "assets/flow.png", Change: SkillChangeUnchanged},
			{Path: "scripts/check.sh", Change: SkillChangeUnchanged},
		}
		if len(contents.Changes) != len(want) {
			t.Fatalf("changes = %+v", contents.Changes)
		}
		for i := range want {
			if contents.Changes[i] != want[i] {
				t.Fatalf("changes = %+v", contents.Changes)
			}
		}
		if _, err := f.approve(t, svc, review); err != nil {
			t.Fatal(err)
		}
		if live := f.digest(t, skillEditName); live != review.ProposedDigest {
			t.Fatalf("installed %s, proposed %s", live, review.ProposedDigest)
		}
		for _, file := range skillEditBase[1:] {
			data, mode := readSkillFileMode(t, filepath.Join(f.root, skillEditName, filepath.FromSlash(file.path)))
			if !bytes.Equal(data, []byte(file.content)) || mode != file.mode {
				t.Fatalf("installed %s = %q mode %o", file.path, data, mode)
			}
		}
	})

	t.Run("Should refuse remove_paths of an absent file or with mode new [UT-053]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		_, err := f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.RemovePaths = []string{"missing.md"}
		})
		if !errors.Is(err, ErrSkillProposeInvalidRemove) {
			t.Fatalf("absent path: Propose() = %v", err)
		}
		_, err = f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.Name, input.Mode, input.RemovePaths = "fresh-skill", SkillProposalNew, []string{"notes.md"}
		})
		if !errors.Is(err, ErrSkillProposeInvalidRemove) {
			t.Fatalf("mode new: Propose() = %v", err)
		}
		requireNoReviews(t, svc)

		removed, err := f.proposeEdit(t, svc, "1. Run make gate.\n", func(input *SkillProposalInput) {
			input.Description, input.RemovePaths = "Base", []string{"scripts/check.sh"}
		})
		if err != nil {
			t.Fatalf("valid removal: %v", err)
		}
		contents, err := ReadSkillReviewContents(removed.Review)
		if err != nil {
			t.Fatal(err)
		}
		if _, kept := contents.Result.find("scripts/check.sh"); kept {
			t.Fatal("removed file kept in the resulting tree")
		}
	})

	t.Run("Should report a tampered snapshot without failing the review read", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "Changed.\n")
		if err := os.WriteFile(filepath.Join(review.SnapshotPath, "SKILL.md"), []byte("tampered"), 0o600); err != nil {
			t.Fatal(err)
		}
		contents, err := ReadSkillReviewContents(review)
		if err != nil || !errors.Is(contents.SnapshotProblem, ErrSkillReviewSnapshotInvalid) ||
			contents.Base != nil || contents.Changes != nil {
			t.Fatalf("contents = %+v %v", contents, err)
		}
	})
}
