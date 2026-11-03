package agentinstance

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (s *SkillProposalService) Approve(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	id, digest, operator, targetRoot string,
) (err error) {
	if !actor.Operator {
		return ErrSkillReviewOperatorOnly
	}
	if operator == "" {
		return errors.New("skill review operator identity is required")
	}
	if targetRoot == "" {
		return ErrSkillReviewNoTarget
	}
	review, draft, err := s.Review(ctx, actor, instance, id)
	if err != nil {
		return err
	}
	if review.Digest != digest || (review.State != SkillReviewPending && review.State != "approved") {
		return ErrSkillReviewStale
	}
	if !proposedSkillName.MatchString(draft.Name) || draft.Name != review.SkillName {
		return ErrSkillReviewStale
	}
	target, err := fileutil.ResolvePathWithinRoot(targetRoot, filepath.Join(targetRoot, draft.Name))
	if err != nil {
		return err
	}
	if _, statErr := os.Lstat(target); statErr == nil {
		if err := matchInstalledDraft(target, draft); err != nil {
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	stage, err := os.MkdirTemp(filepath.Dir(review.DraftPath), "skill-install-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	if err := writeSkillInstallStage(stage, draft); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if review.State == SkillReviewPending {
		if err := s.Store.DecideReview(ctx, id, digest, "approved", operator); err != nil {
			return err
		}
	}
	return publishSkillInstall(stage, target, draft)
}

func publishSkillInstall(stage, target string, draft SkillDraft) error {
	if _, statErr := os.Lstat(target); statErr == nil {
		return matchInstalledDraft(target, draft)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.Rename(stage, target)
}

func writeSkillInstallStage(stage string, draft SkillDraft) error {
	for _, file := range draft.Files {
		if !fs.ValidPath(file.Path) || file.Path == "." || strings.ContainsAny(file.Path, "\\\x00:") {
			return ErrSkillReviewStale
		}
		path := filepath.Join(stage, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func matchInstalledDraft(root string, draft SkillDraft) (err error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	want := make(map[string]string, len(draft.Files))
	for _, file := range draft.Files {
		want[file.Path] = file.Content
	}
	seen := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("skill installation target contains a non-regular file")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		expected, ok := want[filepath.ToSlash(rel)]
		if !ok {
			return errors.New("skill installation target contains different files")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() != int64(len(expected)) {
			return errors.New("skill installation target contains different contents")
		}
		content, err := directory.ReadFile(rel)
		if err != nil {
			return err
		}
		if string(content) != expected {
			return errors.New("skill installation target contains different contents")
		}
		seen++
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(want) {
		return errors.New("skill installation target is incomplete")
	}
	return nil
}
