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
// daemon; the dirpublish lock next to the skill serializes daemons of other homes. It also remembers
// the daemon's committed auto-approvals whose review row is still unprojected, and carries the
// daemon's pending catalog refresh, shared by every copy of the skill service.
type SkillInstallLocks struct {
	mu   sync.Mutex
	held map[string]*skillKeyLock
	// unprojected maps an instance ID to its committed auto-approvals whose projection failed.
	unprojected map[string]map[string]struct{}
	// publishMu serializes a catalog refresh with its publishPending update; publishPending is set
	// while a committed change still needs a catalog refresh.
	publishMu      sync.Mutex
	publishPending bool
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

func (s *SkillProposalService) installLocks() *SkillInstallLocks {
	if s.Locks == nil {
		return defaultSkillInstallLocks
	}
	return s.Locks
}

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

// markUnprojected remembers a committed auto-approval whose review row is still pending, so the
// instance's daily cap fails closed until Reconcile projects it (Business Rule 14).
// ponytail: in memory, so a daemon restart forgets it; persist it if restarts with a failed
// projection matter.
func (l *SkillInstallLocks) markUnprojected(instanceID, reviewID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.unprojected == nil {
		l.unprojected = make(map[string]map[string]struct{})
	}
	if l.unprojected[instanceID] == nil {
		l.unprojected[instanceID] = make(map[string]struct{})
	}
	l.unprojected[instanceID][reviewID] = struct{}{}
}

// unprojectedReviews lists the instance's remembered unprojected auto-approvals.
func (l *SkillInstallLocks) unprojectedReviews(instanceID string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	ids := make([]string, 0, len(l.unprojected[instanceID]))
	for id := range l.unprojected[instanceID] {
		ids = append(ids, id)
	}
	return ids
}

// clearUnprojected forgets an auto-approval whose review row has been projected.
func (l *SkillInstallLocks) clearUnprojected(instanceID, reviewID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.unprojected[instanceID], reviewID)
	if len(l.unprojected[instanceID]) == 0 {
		delete(l.unprojected, instanceID)
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
	release, err := s.installLocks().lock(ctx, dir)
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

// ensureParent creates a missing target root and takes its publication lock, as acquireSkill does
// with create, for a proposal that installs automatically.
func (t *skillTarget) ensureParent() error {
	if t.parent != nil {
		return nil
	}
	if err := os.MkdirAll(t.root, 0o700); err != nil {
		return err
	}
	parent, err := fileutil.OpenDirectoryForMutation(t.root)
	if err != nil {
		return fmt.Errorf("open skill target root: %w", err)
	}
	lock, err := dirpublish.AcquireLock(parent, t.name)
	if errors.Is(err, dirpublish.ErrBusy) {
		err = fmt.Errorf("%w: %s is being changed by another process", ErrSkillInstallBusy, t.name)
	}
	if err != nil {
		return errors.Join(err, parent.Close())
	}
	t.parent, t.lock = parent, lock
	return nil
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
