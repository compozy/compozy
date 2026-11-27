package agentinstance

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
func (f *skillTxFixture) requireLinkedOutside(t *testing.T, name, outside string, originalMode os.FileMode) {
	t.Helper()
	if target, err := os.Readlink(filepath.Join(f.root, name)); err != nil || target != outside {
		t.Fatalf("skill link = %q, %v; want %q", target, err, outside)
	}
	data, mode := readSkillFileMode(t, filepath.Join(outside, "scripts", "check.sh"))
	if string(data) != skillEditBase[1].content || mode != originalMode {
		t.Fatalf("outside directory changed: %q %o", data, mode)
	}
}

// failingCreateStore fails CreateReview, optionally after committing the row, and optionally fails the
// read that settles whether the row exists.
type failingCreateStore struct {
	*memSkillReviewStore
	err     error
	commit  bool
	readErr error
}

func (s *failingCreateStore) CreateReview(
	ctx context.Context,
	review SkillReview,
	report ReportInput,
) (SkillReview, bool, error) {
	if s.commit {
		if _, _, err := s.memSkillReviewStore.CreateReview(ctx, review, report); err != nil {
			return SkillReview{}, false, err
		}
	}
	return SkillReview{}, false, s.err
}

func (s *failingCreateStore) GetReview(ctx context.Context, id string) (SkillReview, error) {
	if s.readErr != nil {
		return SkillReview{}, s.readErr
	}
	return s.memSkillReviewStore.GetReview(ctx, id)
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

	t.Run("Should approve a case-only rename when the original path is removed", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase[0],
			skillTestFile{path: "references/Notes.md", content: "Old notes.\n", mode: 0o644})
		svc := f.service(t, "home-a", nil)
		result, err := f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.RemovePaths = []string{"references/Notes.md"}
			input.Files = []SkillDraftFile{{Path: "references/notes.md", Content: "Updated notes.\n"}}
		})
		if err != nil {
			t.Fatalf("Propose() = %v", err)
		}
		f.mustApprove(t, svc, result.Review)
		dir := filepath.Join(f.root, skillEditName, "references")
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 || entries[0].Name() != "notes.md" {
			t.Fatalf("installed references = %v, %v; want only notes.md", entries, err)
		}
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

	t.Run("Should refuse a symlink in an installed skill [UT-015]", func(t *testing.T) {
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
	})

	t.Run("Should refuse a symlinked installed skill directory [UT-015]", func(t *testing.T) {
		t.Parallel()
		linked := newSkillTxFixture(t)
		linked.install(t, skillEditName, skillEditBase...)
		_, originalMode := readSkillFileMode(t, filepath.Join(linked.root, skillEditName, "scripts", "check.sh"))
		outside := linked.linkSkillOutside(t, skillEditName)
		linkedSvc := linked.service(t, "home-a", nil)
		if _, err := linked.proposeEdit(t, linkedSvc, "Changed.\n", nil); !errors.Is(err,
			ErrSkillReviewBaseUnsupported) {
			t.Fatalf("symlinked skill directory: Propose() = %v, want skill_review_base_unsupported", err)
		}
		requireNoReviews(t, linkedSvc)
		linked.requireLinkedOutside(t, skillEditName, outside, originalMode)
	})

	t.Run("Should refuse an installed skill with too many files [UT-015]", func(t *testing.T) {
		t.Parallel()
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

	t.Run("Should refuse a base file with special permission bits", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not store Unix set-group-ID bits")
		}
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		path := filepath.Join(f.root, skillEditName, "scripts", "check.sh")
		if err := os.Chmod(path, skillEditBase[1].mode|os.ModeSetgid); err != nil {
			t.Fatal(err)
		}
		svc := f.service(t, "home-a", nil)
		if _, err := f.proposeEdit(t, svc, "Changed.\n", nil); !errors.Is(err, ErrSkillReviewBaseUnsupported) {
			t.Fatalf("Propose() = %v, want skill_review_base_unsupported", err)
		}
		requireNoReviews(t, svc)
	})

	t.Run("Should enforce the file limit on the completed overlay", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		files := []skillTestFile{skillEditBase[0]}
		for i := range maxSkillBaseFiles - 1 {
			files = append(files, skillTestFile{
				path: "refs/" + strings.Repeat("a", i+1) + ".md", content: "x", mode: 0o600,
			})
		}
		f.install(t, skillEditName, files...)
		svc := f.service(t, "home-a", nil)
		added := SkillDraftFile{Path: "refs/new.md", Content: "New."}
		_, err := f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.Files = []SkillDraftFile{added}
		})
		if !errors.Is(err, ErrSkillReviewInvalid) {
			t.Fatalf("overflowing edit = %v, want skill_review_invalid", err)
		}
		requireNoReviews(t, svc)
		if _, err := f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.Files = []SkillDraftFile{added}
			input.RemovePaths = []string{"refs/a.md"}
		}); err != nil {
			t.Fatalf("edit within the file limit = %v", err)
		}
	})

	t.Run("Should refuse a completed overlay exceeding the byte limit", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase[0], skillTestFile{
			path: "padding.md", content: strings.Repeat("a", maxSkillBaseBytes-200), mode: 0o600,
		})
		svc := f.service(t, "home-a", nil)
		_, err := f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.Files = []SkillDraftFile{{Path: "refs/new.md", Content: strings.Repeat("a", 300)}}
		})
		if !errors.Is(err, ErrSkillReviewInvalid) {
			t.Fatalf("oversized edit = %v, want skill_review_invalid", err)
		}
		requireNoReviews(t, svc)
	})

	t.Run("Should refuse agent-reachable overlay inputs with skill_review_invalid [UT-015]", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name   string
			mutate func(*SkillProposalInput)
		}{
			{"Should refuse a case-only collision", func(input *SkillProposalInput) {
				input.Files = []SkillDraftFile{{Path: "Scripts/check.sh", Content: "echo other\n"}}
			}},
			{"Should refuse a case-only directory collision after removing the original file", func(input *SkillProposalInput) {
				input.RemovePaths = []string{"scripts/check.sh"}
				input.Files = []SkillDraftFile{
					{Path: "Scripts/check.sh", Content: "echo other\n"},
					{Path: "scripts/alternate.sh", Content: "echo alternate\n"},
				}
			}},
			{"Should refuse a file over a base directory", func(input *SkillProposalInput) {
				input.Files = []SkillDraftFile{{Path: "scripts", Content: "not a directory\n"}}
			}},
			{"Should refuse a file over a base directory with different case", func(input *SkillProposalInput) {
				input.Files = []SkillDraftFile{{Path: "Scripts", Content: "not a directory\n"}}
			}},
			{"Should refuse an unknown mode", func(input *SkillProposalInput) { input.Mode = "replace" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				f.install(t, skillEditName, skillEditBase...)
				svc := f.service(t, "home-a", nil)
				if _, err := f.proposeEdit(t, svc, "Changed.\n", tc.mutate); !errors.Is(err, ErrSkillReviewInvalid) {
					t.Fatalf("Propose() = %v, want skill_review_invalid", err)
				}
				requireNoReviews(t, svc)
			})
		}
	})

	t.Run("Should drop the draft and snapshot only when the failed review row is absent", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name      string
			committed bool
			readErr   error
			kept      bool
		}{
			{name: "Should discard artifacts when no row was created", kept: false},
			{name: "Should keep artifacts when the row committed", committed: true, kept: true},
			{name: "Should keep artifacts when the row is unreadable", readErr: errors.New("database closed"), kept: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				f.install(t, skillEditName, skillEditBase...)
				svc := f.service(t, "home-a", nil)
				createErr := errors.New("database is locked")
				svc.Store = &failingCreateStore{
					memSkillReviewStore: skillStore(svc), err: createErr, commit: tc.committed, readErr: tc.readErr,
				}
				if _, err := f.proposeEdit(t, svc, "Changed.\n", nil); !errors.Is(err, createErr) {
					t.Fatalf("%s: Propose() = %v, want the store error", tc.name, err)
				}
				entries, err := os.ReadDir(svc.DraftRoot)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				// A kept proposal leaves its draft and its base snapshot.
				want := 0
				if tc.kept {
					want = 2
				}
				if len(entries) != want {
					t.Fatalf("%s: draft root holds %d entries, kept = %v", tc.name, len(entries), tc.kept)
				}
			})
		}
	})

	t.Run("Should install the reviewed tree when the base still matches [UT-016]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		path := filepath.Join(f.root, skillEditName, "SKILL.md")
		_, originalMode := readSkillFileMode(t, path)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "1. Run make gate.\n2. Confirm the freeze calendar.\n")
		result, err := f.approve(t, svc, review)
		if err != nil || !result.Committed || result.Warning != nil {
			t.Fatalf("Approve() = %+v %v", result, err)
		}
		got := requireReviewState(t, svc, review.ID, skillReviewApproved)
		live := f.digest(t, skillEditName)
		if got.InstalledDigest != live || live != review.ProposedDigest {
			t.Fatalf("review = %+v, live %s", got, live)
		}
		definition, mode := readSkillFileMode(t, path)
		if !strings.Contains(string(definition), "2. Confirm the freeze calendar.") || mode != originalMode {
			t.Fatalf("SKILL.md = %q mode %o", definition, mode)
		}
		f.requireClean(t)
	})

	t.Run("Should refuse edit approval at a different agent-local root with the same bytes", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.instance.WorkspaceID = ""
		original := f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "Changed.\n")
		moved := skillTxFixture{root: filepath.Join(t.TempDir(), "skills"), instance: f.instance}
		if base := moved.install(t, skillEditName, skillEditBase...); base != original {
			t.Fatalf("alternate base digest = %q, want %q", base, original)
		}
		if _, err := moved.approve(t, svc, review); !errors.Is(err, ErrSkillReviewBaseStale) {
			t.Fatalf("Approve() = %v, want skill_review_base_stale", err)
		}
		requireReviewState(t, svc, review.ID, SkillReviewPending)
		if f.digest(t, skillEditName) != original || moved.digest(t, skillEditName) != original {
			t.Fatal("the refused approval changed an installed skill")
		}
	})

	t.Run("Should refuse approval after the installed skill changed or was deleted [UT-017]", func(t *testing.T) {
		t.Parallel()
		for _, change := range []struct {
			name    string
			deleted bool
		}{
			{name: "Should refuse approval after the base changed"},
			{name: "Should refuse approval after the base was deleted", deleted: true},
		} {
			t.Run(change.name, func(t *testing.T) {
				t.Parallel()
				f := newSkillTxFixture(t)
				f.install(t, skillEditName, skillEditBase...)
				svc := f.service(t, "home-a", nil)
				review := f.mustProposeEdit(t, svc, "Changed.\n")
				dir := filepath.Join(f.root, skillEditName)
				if !change.deleted {
					if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("operator"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
				before, err := skillTreeState(dir)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.approve(t, svc, review); !errors.Is(err, ErrSkillReviewBaseStale) {
					t.Fatalf("Approve() = %v, want base stale", err)
				}
				requireReviewState(t, svc, review.ID, SkillReviewPending)
				after, err := skillTreeState(dir)
				if err != nil || after != before {
					t.Fatalf("live %s -> %s, err %v", before, after, err)
				}
				f.requireClean(t)
			})
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
		modes := make(map[string]os.FileMode, len(skillEditBase)-1)
		for _, file := range skillEditBase[1:] {
			_, modes[file.path] = readSkillFileMode(
				t,
				filepath.Join(f.root, skillEditName, filepath.FromSlash(file.path)),
			)
		}
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "1. Run make gate.\n2. Confirm the freeze calendar.\n")
		contents, err := ReadSkillReviewContents(t.Context(), review)
		if err != nil || contents.SnapshotProblem != nil {
			t.Fatalf("ReadSkillReviewContents() = %v %v", err, contents.SnapshotProblem)
		}
		for _, file := range skillEditBase[1:] {
			kept, ok := contents.Result.find(file.path)
			if !ok || !bytes.Equal(kept.Data, []byte(file.content)) || kept.Perm != modes[file.path] {
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
			if !bytes.Equal(data, []byte(file.content)) || mode != modes[file.path] {
				t.Fatalf("installed %s = %q mode %o", file.path, data, mode)
			}
		}
	})

	t.Run("Should refuse an absent removal path [UT-053]", func(t *testing.T) {
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
		requireNoReviews(t, svc)
	})

	t.Run("Should refuse removal paths on a new skill [UT-053]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		_, err := f.proposeEdit(t, svc, "Changed.\n", func(input *SkillProposalInput) {
			input.Name, input.Mode, input.RemovePaths = "fresh-skill", SkillProposalNew, []string{"notes.md"}
		})
		if !errors.Is(err, ErrSkillProposeInvalidRemove) {
			t.Fatalf("mode new: Propose() = %v", err)
		}
		requireNoReviews(t, svc)
	})

	t.Run("Should show the overlay with the named file removed [UT-053]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		removed, err := f.proposeEdit(t, svc, "1. Run make gate.\n", func(input *SkillProposalInput) {
			input.Description, input.RemovePaths = "Base", []string{"scripts/check.sh"}
		})
		if err != nil {
			t.Fatalf("valid removal: %v", err)
		}
		contents, err := ReadSkillReviewContents(t.Context(), removed.Review)
		if err != nil || contents.SnapshotProblem != nil {
			t.Fatalf("ReadSkillReviewContents() = %v, snapshot problem %v", err, contents.SnapshotProblem)
		}
		if _, kept := contents.Result.find("assets/flow.png"); !kept {
			t.Fatal("retained file missing from the resulting tree")
		}
		if _, kept := contents.Result.find("scripts/check.sh"); kept {
			t.Fatal("removed file kept in the resulting tree")
		}
	})

	t.Run("Should report a tampered snapshot and refuse to approve it [UT-016]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		base := f.install(t, skillEditName, skillEditBase...)
		svc := f.service(t, "home-a", nil)
		review := f.mustProposeEdit(t, svc, "Changed.\n")
		if err := os.WriteFile(filepath.Join(review.SnapshotPath, "SKILL.md"), []byte("tampered"), 0o600); err != nil {
			t.Fatal(err)
		}
		contents, err := ReadSkillReviewContents(t.Context(), review)
		if err != nil || !errors.Is(contents.SnapshotProblem, ErrSkillReviewSnapshotInvalid) ||
			contents.Base != nil || contents.Changes != nil {
			t.Fatalf("contents = %+v %v", contents, err)
		}
		// The live base still matches, but revert could never restore it: approval is refused.
		if _, err := f.approve(t, svc, review); !errors.Is(err, ErrSkillReviewInvalid) ||
			!errors.Is(err, ErrSkillReviewSnapshotInvalid) {
			t.Fatalf("Approve() with a tampered snapshot = %v, want skill_review_invalid", err)
		}
		if f.digest(t, skillEditName) != base {
			t.Fatal("the refused approval changed the skill")
		}
		requireReviewState(t, svc, review.ID, SkillReviewPending)
		f.requireClean(t)
	})
}
