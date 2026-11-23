package agentinstance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
)

var (
	// ErrSkillInstallBusy means another process holds the skill's publication lock (retryable).
	ErrSkillInstallBusy = errors.New("skill_install_busy")
	// ErrSkillInstallConflict means the skill has a retained install conflict awaiting the operator.
	ErrSkillInstallConflict = errors.New("skill_install_conflict")
	// ErrSkillReviewTargetExists means a `new` review's skill name is now installed.
	ErrSkillReviewTargetExists = errors.New("skill_review_target_exists")
)

// SkillInstallLocks is the in-process keyed mutex per physical skill. It queues callers inside one
// daemon; the dirpublish lock next to the skill serializes daemons of other homes.
type SkillInstallLocks struct {
	mu   sync.Mutex
	held map[string]*skillKeyLock
}

type skillKeyLock struct {
	ch   chan struct{}
	refs int
}

// NewSkillInstallLocks returns the keyed mutex one daemon shares between its skill services.
func NewSkillInstallLocks() *SkillInstallLocks {
	return &SkillInstallLocks{held: make(map[string]*skillKeyLock)}
}

// defaultSkillInstallLocks serves services built without wiring, such as tests of one home.
var defaultSkillInstallLocks = NewSkillInstallLocks()

func (l *SkillInstallLocks) lock(ctx context.Context, key string) (func(), error) {
	l.mu.Lock()
	entry := l.held[key]
	if entry == nil {
		entry = &skillKeyLock{ch: make(chan struct{}, 1)}
		l.held[key] = entry
	}
	entry.refs++
	l.mu.Unlock()
	drop := func() {
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.held, key)
		}
		l.mu.Unlock()
	}
	select {
	case entry.ch <- struct{}{}:
		return func() { <-entry.ch; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

// skillTarget is one physical skill held under both locks. parent is nil when the target root does
// not exist and the caller did not ask to create it: nothing can be installed or in flight there.
type skillTarget struct {
	root    string // canonical target root
	name    string
	dir     string // root/name
	parent  *fileutil.Directory
	lock    *dirpublish.Lock
	release func()
}

// acquireSkill takes the keyed mutex, then the dirpublish lock of root/name. A lock held by another
// process returns ErrSkillInstallBusy.
func (s *SkillProposalService) acquireSkill(
	ctx context.Context,
	targetRoot, name string,
	create bool,
) (_ *skillTarget, err error) {
	if !proposedSkillName.MatchString(name) {
		return nil, ErrSkillReviewInvalid
	}
	if targetRoot == "" {
		return nil, ErrSkillReviewNoTarget
	}
	if create {
		if err := os.MkdirAll(targetRoot, 0o700); err != nil {
			return nil, err
		}
	}
	root, err := fileutil.CanonicalPathWithExistingPrefix(targetRoot)
	if err != nil {
		return nil, err
	}
	// The leaf is never canonicalized: root is canonical and name is a validated single path element,
	// so the lexical path cannot leave the root. A symlinked skill directory is then reported by each
	// operation (Lstat-based digests and reads refuse it; moves rename the link through the parent
	// handle) instead of being followed or failing here with a path error.
	dir := filepath.Join(root, name)
	locks := s.Locks
	if locks == nil {
		locks = defaultSkillInstallLocks
	}
	release, err := locks.lock(ctx, dir)
	if err != nil {
		return nil, err
	}
	target := &skillTarget{root: root, name: name, dir: dir, release: release}
	defer func() {
		if err != nil {
			err = errors.Join(err, target.close())
		}
	}()
	target.parent, err = fileutil.OpenDirectoryForMutation(root)
	if errors.Is(err, os.ErrNotExist) && !create {
		return target, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open skill target root: %w", err)
	}
	target.lock, err = dirpublish.AcquireLock(target.parent, name)
	if errors.Is(err, dirpublish.ErrBusy) {
		return nil, fmt.Errorf("%w: %s is being changed by another process", ErrSkillInstallBusy, name)
	}
	if err != nil {
		return nil, err
	}
	return target, nil
}

func (t *skillTarget) close() error {
	err := t.lock.Close()
	if t.parent != nil {
		err = errors.Join(err, t.parent.Close())
	}
	t.release()
	return err
}

func (t *skillTarget) path(name string) string { return filepath.Join(t.root, name) }
