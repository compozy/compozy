package workpackages

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

var completionHeadingPattern = regexp.MustCompile(`(?m)^## \[([ x])\] (WP-[0-9]{3}) — [^\r\n]+`)

// CompletionBlockReason identifies why completion cannot be recorded.
type CompletionBlockReason string

const (
	// CompletionBlockNone means all completion prerequisites hold.
	CompletionBlockNone CompletionBlockReason = ""
	// CompletionBlockVerificationFailed means final verification did not pass.
	CompletionBlockVerificationFailed CompletionBlockReason = "verification_failed"
	// CompletionBlockReviewInterrupted means final review did not reach a clean result.
	CompletionBlockReviewInterrupted CompletionBlockReason = "review_interrupted"
	// CompletionBlockNewIssues means the final review created issues.
	CompletionBlockNewIssues CompletionBlockReason = "new_issues"
	// CompletionBlockPriorIssuesUnresolved means an earlier issue is not resolved.
	CompletionBlockPriorIssuesUnresolved CompletionBlockReason = "prior_issues_unresolved"
	// CompletionBlockHeadingMissing means the selected stable package heading is unavailable.
	CompletionBlockHeadingMissing CompletionBlockReason = "heading_missing"
)

// CompletionPreconditions is the review state required before plan mutation.
type CompletionPreconditions struct {
	VerificationPassed bool
	ReviewInterrupted  bool
	NewIssues          bool
	PriorIssueStatuses []string
	HeadingExists      bool
}

// CompletionEligibility describes whether completion may be recorded.
type CompletionEligibility struct {
	Eligible bool
	Reason   CompletionBlockReason
}

// CanRecordCompletion evaluates review and selected-heading preconditions.
func CanRecordCompletion(preconditions CompletionPreconditions) CompletionEligibility {
	if !preconditions.VerificationPassed {
		return CompletionEligibility{Reason: CompletionBlockVerificationFailed}
	}
	if preconditions.ReviewInterrupted {
		return CompletionEligibility{Reason: CompletionBlockReviewInterrupted}
	}
	if preconditions.NewIssues {
		return CompletionEligibility{Reason: CompletionBlockNewIssues}
	}
	for _, status := range preconditions.PriorIssueStatuses {
		if strings.TrimSpace(status) != "resolved" {
			return CompletionEligibility{Reason: CompletionBlockPriorIssuesUnresolved}
		}
	}
	if !preconditions.HeadingExists {
		return CompletionEligibility{Reason: CompletionBlockHeadingMissing}
	}
	return CompletionEligibility{Eligible: true}
}

// LifecycleState is the Markdown-derived Work Package lifecycle projection.
type LifecycleState struct {
	LifecycleComplete bool
}

// ProjectLifecycleState projects only the canonical Markdown checkbox state.
func ProjectLifecycleState(plan Plan, packageID string) (LifecycleState, error) {
	pkg, exists := plan.Package(packageID)
	if !exists {
		return LifecycleState{}, packageNotFound(Ref{Initiative: plan.Initiative, PackageID: packageID}, plan)
	}
	return LifecycleState{LifecycleComplete: pkg.Completed}, nil
}

// RewriteResult is the result of an in-memory stable checkbox rewrite.
type RewriteResult struct {
	Content          []byte
	AlreadyCompleted bool
	WriteRequired    bool
}

// RewriteCompletion changes only the selected stable heading checkbox in content.
func RewriteCompletion(content []byte, packageID string) (RewriteResult, error) {
	matches := completionHeadingPattern.FindAllSubmatchIndex(content, -1)
	selected := make([][]int, 0, 1)
	for _, match := range matches {
		id := string(content[match[4]:match[5]])
		if id == packageID {
			selected = append(selected, match)
		}
	}
	if len(selected) != 1 {
		return RewriteResult{}, newError(
			ErrCompletionConflict,
			"",
			packageID,
			"",
			[]Issue{{Field: "body." + packageID, Message: "must contain exactly one compatible package heading"}},
		)
	}
	if _, err := ParsePlan(string(content)); err != nil {
		return RewriteResult{}, err
	}
	match := selected[0]
	if content[match[2]] == 'x' {
		return RewriteResult{Content: slices.Clone(content), AlreadyCompleted: true}, nil
	}
	rewritten := slices.Clone(content)
	rewritten[match[2]] = 'x'
	return RewriteResult{Content: rewritten, WriteRequired: true}, nil
}

// CompletionResult reports a durable completion mutation without any Git state.
type CompletionResult struct {
	Plan               Plan
	CompletionRecorded bool
	AlreadyCompleted   bool
}

// Store loads plans and records one stable completion through a locked atomic write.
type Store struct {
	newLock func(string) *flock.Flock
	ops     atomicFileOps
}

// NewStore creates a completion store with real filesystem operations.
func NewStore() *Store {
	return &Store{
		newLock: func(path string) *flock.Flock { return flock.New(path) },
		ops:     defaultAtomicFileOps(),
	}
}

// Load reads and validates the current plan from an initiative root.
func (s *Store) Load(ctx context.Context, initiativeDir string) (Plan, error) {
	if err := context.Cause(ctx); err != nil {
		return Plan{}, fmt.Errorf("load work package plan: %w", err)
	}
	planPath := filepath.Join(initiativeDir, ManifestFileName)
	content, err := os.ReadFile(planPath)
	if err != nil {
		return Plan{}, newError(
			ErrInvalidPlan,
			filepath.Base(initiativeDir),
			"",
			planPath,
			[]Issue{{Path: planPath, Field: "marker", Message: err.Error()}},
		)
	}
	plan, err := ParsePlanForInitiative(string(content), filepath.Base(initiativeDir))
	if err != nil {
		var domainErr *Error
		if errors.As(err, &domainErr) {
			domainErr.PlanPath = planPath
		}
		return Plan{}, err
	}
	plan.Path = planPath
	return plan, nil
}

// MarkComplete locks, rereads, validates, and atomically records one checkbox.
func (s *Store) MarkComplete(
	ctx context.Context,
	initiativeDir, packageID string,
) (CompletionResult, error) {
	if err := context.Cause(ctx); err != nil {
		return CompletionResult{}, fmt.Errorf("complete work package: %w", err)
	}
	s = usableStore(s)
	planPath := filepath.Join(initiativeDir, ManifestFileName)
	return s.withPlanLock(ctx, planPath, func() (CompletionResult, error) {
		return s.markCompleteLocked(ctx, initiativeDir, planPath, packageID)
	})
}

func usableStore(store *Store) *Store {
	if store == nil {
		return NewStore()
	}
	return store
}

func (s *Store) withPlanLock(
	ctx context.Context,
	planPath string,
	action func() (CompletionResult, error),
) (result CompletionResult, err error) {
	lock := s.newLock(planPath + ".lock")
	if lock == nil {
		return CompletionResult{}, errors.New("create work package completion lock")
	}
	locked, lockErr := lock.TryLockContext(ctx, 25*time.Millisecond)
	if lockErr != nil {
		return CompletionResult{}, fmt.Errorf("lock work package plan: %w", lockErr)
	}
	if !locked {
		return CompletionResult{}, fmt.Errorf("lock work package plan: %w", context.DeadlineExceeded)
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			err = errors.Join(err, fmt.Errorf("unlock work package plan: %w", unlockErr))
		}
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close work package completion lock: %w", closeErr))
		}
	}()
	return action()
}

func (s *Store) markCompleteLocked(
	ctx context.Context,
	initiativeDir, planPath, packageID string,
) (CompletionResult, error) {
	content, readErr := os.ReadFile(planPath)
	if readErr != nil {
		return CompletionResult{}, newError(
			ErrInvalidPlan,
			filepath.Base(initiativeDir),
			packageID,
			planPath,
			[]Issue{{Path: planPath, Field: "marker", Message: readErr.Error()}},
		)
	}
	plan, parseErr := ParsePlanForInitiative(string(content), filepath.Base(initiativeDir))
	if parseErr != nil {
		var domainErr *Error
		if errors.As(parseErr, &domainErr) {
			domainErr.PlanPath = planPath
		}
		return CompletionResult{}, parseErr
	}
	plan.Path = planPath
	if _, exists := plan.Package(packageID); !exists {
		return CompletionResult{}, newError(
			ErrCompletionConflict,
			plan.Initiative,
			packageID,
			planPath,
			[]Issue{{Field: "body." + packageID, Message: "selected package does not exist"}},
		)
	}
	rewrite, rewriteErr := RewriteCompletion(content, packageID)
	if rewriteErr != nil {
		return CompletionResult{}, rewriteErr
	}
	if rewrite.AlreadyCompleted {
		return CompletionResult{Plan: plan, AlreadyCompleted: true}, nil
	}
	info, statErr := os.Stat(planPath)
	if statErr != nil {
		return CompletionResult{}, fmt.Errorf("stat work package plan: %w", statErr)
	}
	if info.Mode().Perm()&0o222 == 0 {
		return CompletionResult{}, newError(
			ErrPlanReadOnly,
			plan.Initiative,
			packageID,
			planPath,
			[]Issue{{Path: planPath, Field: "write", Message: "work package plan has no write permission"}},
		)
	}
	if writeErr := s.ops.write(planPath, rewrite.Content, info.Mode().Perm()); writeErr != nil {
		if errors.Is(writeErr, fs.ErrPermission) {
			return CompletionResult{}, newError(
				ErrPlanReadOnly,
				plan.Initiative,
				packageID,
				planPath,
				[]Issue{{Path: planPath, Field: "write", Message: writeErr.Error()}},
			)
		}
		return CompletionResult{}, writeErr
	}
	committed, committedErr := s.Load(ctx, initiativeDir)
	if committedErr != nil {
		return CompletionResult{}, fmt.Errorf("verify completed work package plan: %w", committedErr)
	}
	if !committed.IsComplete(packageID) {
		return CompletionResult{}, newError(
			ErrCompletionConflict,
			committed.Initiative,
			packageID,
			planPath,
			[]Issue{{Field: "body." + packageID, Message: "completion checkbox was not recorded"}},
		)
	}
	return CompletionResult{Plan: committed, CompletionRecorded: true}, nil
}

// MarkComplete records a checkbox through a default completion store.
func MarkComplete(ctx context.Context, initiativeDir, packageID string) (CompletionResult, error) {
	return NewStore().MarkComplete(ctx, initiativeDir, packageID)
}

type atomicTempFile interface {
	Name() string
	Write([]byte) (int, error)
	Sync() error
	Close() error
	Chmod(fs.FileMode) error
}

type syncFile interface {
	Sync() error
	Close() error
}

type atomicFileOps struct {
	createTemp func(string, string) (atomicTempFile, error)
	stat       func(string) (fs.FileInfo, error)
	rename     func(string, string) error
	remove     func(string) error
	openDir    func(string) (syncFile, error)
	write      func(string, []byte, fs.FileMode) error
}

func defaultAtomicFileOps() atomicFileOps {
	ops := atomicFileOps{
		createTemp: func(directory, pattern string) (atomicTempFile, error) { return os.CreateTemp(directory, pattern) },
		stat:       os.Stat,
		rename:     os.Rename,
		remove:     os.Remove,
		openDir: func(directory string) (syncFile, error) {
			return os.Open(directory)
		},
	}
	ops.write = func(path string, content []byte, mode fs.FileMode) error {
		return writePlanAtomically(ops, path, content, mode)
	}
	return ops
}

func writePlanAtomically(ops atomicFileOps, path string, content []byte, mode fs.FileMode) (err error) {
	directory := filepath.Dir(path)
	temporary, err := ops.createTemp(directory, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create work package temp file: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	cleanup := func() error {
		var cleanupErr error
		if !closed {
			cleanupErr = errors.Join(cleanupErr, temporary.Close())
			closed = true
		}
		cleanupErr = errors.Join(cleanupErr, ops.remove(temporaryPath))
		return cleanupErr
	}
	fail := func(operation string, cause error) error {
		return errors.Join(fmt.Errorf("%s: %w", operation, cause), cleanup())
	}
	if _, writeErr := temporary.Write(content); writeErr != nil {
		return fail("write work package temp file", writeErr)
	}
	if chmodErr := temporary.Chmod(mode); chmodErr != nil {
		return fail("preserve work package plan permissions", chmodErr)
	}
	if syncErr := temporary.Sync(); syncErr != nil {
		return fail("sync work package temp file", syncErr)
	}
	if closeErr := temporary.Close(); closeErr != nil {
		closed = true
		return errors.Join(fmt.Errorf("close work package temp file: %w", closeErr), ops.remove(temporaryPath))
	}
	closed = true
	if renameErr := ops.rename(temporaryPath, path); renameErr != nil {
		return errors.Join(fmt.Errorf("replace work package plan: %w", renameErr), ops.remove(temporaryPath))
	}
	directoryFile, openErr := ops.openDir(directory)
	if openErr != nil {
		return fmt.Errorf("open work package plan directory: %w", openErr)
	}
	if syncErr := directoryFile.Sync(); syncErr != nil {
		closeErr := directoryFile.Close()
		return errors.Join(fmt.Errorf("sync work package plan directory: %w", syncErr), closeErr)
	}
	if closeErr := directoryFile.Close(); closeErr != nil {
		return fmt.Errorf("close work package plan directory: %w", closeErr)
	}
	return nil
}
