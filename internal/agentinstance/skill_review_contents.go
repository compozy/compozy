package agentinstance

import (
	"bytes"
	"errors"
)

// Skill tree change kinds of an edit review.
const (
	SkillChangeAdded     = "added"
	SkillChangeRemoved   = "removed"
	SkillChangeModified  = "modified"
	SkillChangeUnchanged = "unchanged"
)

// SkillTreeChange is one path of an edit's base or resulting tree.
type SkillTreeChange struct {
	Path   string
	Change string
}

// SkillReviewContents is what a review shows: the stored draft and, for an edit, the verified base
// snapshot, the resulting tree and its per-path changes.
type SkillReviewContents struct {
	Draft SkillDraft
	// Base is the snapshot of an edit; nil for new reviews or when SnapshotProblem is set.
	Base SkillTree
	// Result is the tree approval installs: the draft for new reviews, the overlay for edits.
	Result  SkillTree
	Changes []SkillTreeChange
	// SnapshotProblem is ErrSkillReviewSnapshotInvalid when the edit's base snapshot fails verification;
	// Base, Result and Changes are then empty.
	SnapshotProblem error
}

// ReadSkillReviewContents reads a review's draft and, for an edit, its re-verified base snapshot. A
// draft that fails verification returns ErrSkillReviewInvalid.
func ReadSkillReviewContents(review SkillReview) (SkillReviewContents, error) {
	draft, err := ReadSkillDraft(review.DraftPath, review.Digest)
	if err != nil {
		return SkillReviewContents{}, err
	}
	contents := SkillReviewContents{Draft: draft}
	if review.Kind != SkillProposalEdit {
		contents.Result = draftSkillTree(draft)
		return contents, nil
	}
	base, err := readSkillSnapshot(review)
	if err == nil {
		contents.Result, err = applySkillOverlay(base, draft)
	}
	if err != nil {
		contents.SnapshotProblem = errors.Join(ErrSkillReviewSnapshotInvalid, err)
		contents.Result = nil
		return contents, nil
	}
	contents.Base = base
	contents.Changes = skillTreeChanges(base, contents.Result)
	return contents, nil
}

// skillTreeChanges compares base and result by path and bytes, sorted by path.
func skillTreeChanges(base, result SkillTree) []SkillTreeChange {
	changes := make([]SkillTreeChange, 0, len(base)+len(result))
	i, j := 0, 0
	for i < len(base) || j < len(result) {
		switch {
		case j == len(result) || (i < len(base) && base[i].Path < result[j].Path):
			changes = append(changes, SkillTreeChange{Path: base[i].Path, Change: SkillChangeRemoved})
			i++
		case i == len(base) || result[j].Path < base[i].Path:
			changes = append(changes, SkillTreeChange{Path: result[j].Path, Change: SkillChangeAdded})
			j++
		default:
			change := SkillChangeUnchanged
			if !bytes.Equal(base[i].Data, result[j].Data) || base[i].Perm != result[j].Perm {
				change = SkillChangeModified
			}
			changes = append(changes, SkillTreeChange{Path: result[j].Path, Change: change})
			i, j = i+1, j+1
		}
	}
	return changes
}
