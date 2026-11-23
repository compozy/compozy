package agentinstance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// memSkillReviewStore is the home-local review store fake: projections are idempotent and only a
// pending review changes, as in SQLite.
type memSkillReviewStore struct {
	mu         sync.Mutex
	reviews    map[string]SkillReview
	projectErr error
}

var _ SkillReviewStore = (*memSkillReviewStore)(nil)

func (s *memSkillReviewStore) CreateReview(
	_ context.Context,
	review SkillReview,
	_ ReportInput,
) (SkillReview, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	review.State = SkillReviewPending
	review.CreatedAt = time.Now()
	s.reviews[review.ID] = review
	return review, false, nil
}

func (s *memSkillReviewStore) GetReview(_ context.Context, id string) (SkillReview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	review, ok := s.reviews[id]
	if !ok {
		return SkillReview{}, ErrSkillReviewNotFound
	}
	return review, nil
}

func (s *memSkillReviewStore) ListReviews(context.Context, SkillReviewQuery) ([]SkillReview, error) {
	return nil, nil
}

func (s *memSkillReviewStore) ProjectApproval(
	_ context.Context, marker SkillInstallMarker, installedDigest string, prev []byte, decidedAt time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projectErr != nil {
		return s.projectErr
	}
	review, ok := s.reviews[marker.ReviewID]
	if !ok || review.State != SkillReviewPending {
		return nil
	}
	review.State, review.DecidedBy, review.DecidedVia = "approved", marker.DecidedBy, marker.DecidedVia
	review.InstalledDigest, review.PrevInstallMarker, review.DecidedAt = installedDigest, prev, &decidedAt
	s.reviews[review.ID] = review
	return nil
}

func (s *memSkillReviewStore) ProjectRevert(_ context.Context, id, operator string, revertedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projectErr != nil {
		return s.projectErr
	}
	review, ok := s.reviews[id]
	if !ok || review.State != "approved" {
		return nil
	}
	review.State, review.RevertedBy, review.RevertedAt = SkillReviewReverted, operator, &revertedAt
	s.reviews[id] = review
	return nil
}

func (s *memSkillReviewStore) CountAutoApprovals(
	_ context.Context,
	instanceID string,
	dayStart time.Time,
) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, review := range s.reviews {
		if review.InstanceID == instanceID && review.DecidedVia == skillDecidedViaAutoApprove &&
			(review.State == skillReviewApproved || review.State == SkillReviewReverted) &&
			review.DecidedAt != nil && !review.DecidedAt.Before(dayStart) &&
			review.DecidedAt.Before(dayStart.Add(24*time.Hour)) {
			count++
		}
	}
	return count, nil
}

func (s *memSkillReviewStore) DecideReject(_ context.Context, id, digest, operator string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	review, ok := s.reviews[id]
	if !ok || review.State != SkillReviewPending || review.Digest != digest {
		return ErrSkillReviewStale
	}
	review.State, review.DecidedBy, review.DecidedVia = "rejected", operator, skillDecidedViaOperator
	s.reviews[id] = review
	return nil
}

type skillTxFixture struct {
	root     string
	instance Instance
}

var skillOperator = workspaceaccess.ActorRef{Operator: true}

var errSkillTestChanged = errors.New("live skill changed")

func newSkillTxFixture(t *testing.T) *skillTxFixture {
	t.Helper()
	return &skillTxFixture{
		root:     filepath.Join(t.TempDir(), "skills"),
		instance: Instance{ID: "inst-a", ProfileID: "prof-a", WorkspaceID: "ws_a", AgentName: "release-watch"},
	}
}

// service returns one home's service; a nil store starts a new home store.
func (f *skillTxFixture) service(t *testing.T, homeID string, store *memSkillReviewStore) *SkillProposalService {
	t.Helper()
	if store == nil {
		store = &memSkillReviewStore{reviews: map[string]SkillReview{}}
	}
	return &SkillProposalService{
		Store:     store,
		Evidence:  scopedEvidenceSessions{"sess-a": {"prof-a", "ws_a"}},
		DraftRoot: t.TempDir(),
		ScanRoots: []string{f.root},
		HomeID:    homeID,
		Locks:     NewSkillInstallLocks(),
	}
}

func skillStore(svc *SkillProposalService) *memSkillReviewStore {
	return svc.Store.(*memSkillReviewStore)
}

// withHook returns a copy of svc sharing its store and locks whose transaction calls hook at each step.
func withHook(svc *SkillProposalService, hook func(step string) bool) *SkillProposalService {
	hooked := *svc
	hooked.crashAt = hook
	return &hooked
}

func crashAt(svc *SkillProposalService, step string) *SkillProposalService {
	return withHook(svc, func(current string) bool { return current == step })
}

func (f *skillTxFixture) propose(t *testing.T, svc *SkillProposalService, name, body string) SkillReview {
	t.Helper()
	result, err := svc.Propose(t.Context(), skillOperator, f.instance, f.root, SkillProposalInput{
		Name: name, Description: "Use before tagging a release.", Body: body,
		Lesson: SkillLesson{
			Cue: "Tagging a release", Lesson: "Check the changelog first.", Evidence: "sess-a missed it.",
		},
		EvidenceSessionIDs: []string{"sess-a"},
	})
	if err != nil {
		t.Fatalf("Propose(%s) = %v", name, err)
	}
	return result.Review
}

func (f *skillTxFixture) approve(
	t *testing.T,
	svc *SkillProposalService,
	review SkillReview,
) (SkillInstallResult, error) {
	t.Helper()
	return svc.Approve(t.Context(), skillOperator, f.instance, review.ID, review.Digest, "operator:prof-a", f.root)
}

func (f *skillTxFixture) reconcile(t *testing.T, svc *SkillProposalService) {
	t.Helper()
	if err := svc.ReconcileTargetRoot(t.Context(), f.root); err != nil {
		t.Fatalf("ReconcileTargetRoot() = %v", err)
	}
}

type skillTestFile struct {
	path    string
	content string
	mode    os.FileMode
}

// install writes an operator-authored skill directly and returns its digest.
func (f *skillTxFixture) install(t *testing.T, name string, files ...skillTestFile) string {
	t.Helper()
	for _, file := range files {
		full := filepath.Join(f.root, name, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(file.content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(full, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	return f.digest(t, name)
}

func (f *skillTxFixture) digest(t *testing.T, name string) string {
	t.Helper()
	digest, err := skillTreeState(filepath.Join(f.root, name))
	if err != nil {
		t.Fatalf("digest %s: %v", name, err)
	}
	return digest
}

var skillBaseFiles = []skillTestFile{
	{path: "SKILL.md", content: "---\nname: base\ndescription: Base\n---\n\nBase body", mode: 0o644},
	{path: "scripts/run.sh", content: "#!/bin/sh\necho run\n", mode: 0o755},
	{path: "data.bin", content: "\x00\xff\x10binary", mode: 0o600},
}

// replaceOp replaces the installed skill name with one changed file, as edits and reverts will.
func (f *skillTxFixture) replaceOp(t *testing.T, name string) skillOp {
	t.Helper()
	return skillOp{
		reviewID: "skr-replace", instanceID: f.instance.ID, action: SkillInstallActionInstall,
		decidedVia: skillDecidedViaOperator, decidedBy: "operator:prof-a",
		preExists: true, preDigest: f.digest(t, name), changed: errSkillTestChanged,
		stage: func(dir string) error {
			return writeSkillFile(filepath.Join(dir, "SKILL.md"), []byte("replaced"), 0o600)
		},
		next: &SkillInstallMarker{ReviewID: "skr-replace", InstanceID: f.instance.ID, DecidedBy: "operator:prof-a"},
	}
}

func (f *skillTxFixture) runOp(t *testing.T, svc *SkillProposalService, name string, op skillOp) error {
	t.Helper()
	target, err := svc.acquireSkill(t.Context(), f.root, name, true)
	if err != nil {
		t.Fatal(err)
	}
	_, runErr := svc.runSkillOp(t.Context(), target, op)
	if err := target.close(); err != nil {
		t.Fatal(err)
	}
	return runErr
}

// leftovers lists transaction directories and journals left in the target root.
func (f *skillTxFixture) leftovers(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".compozy-incoming-") || strings.HasPrefix(name, ".compozy-backup-") ||
			strings.HasPrefix(name, skillOriginalCopyPrefix) ||
			(strings.HasPrefix(name, ".compozy-publication-") && strings.HasSuffix(name, ".json")) {
			names = append(names, name)
		}
	}
	return names
}

func (f *skillTxFixture) requireClean(t *testing.T) {
	t.Helper()
	if left := f.leftovers(t); len(left) != 0 {
		t.Fatalf("transaction leftovers: %v", left)
	}
}

func (f *skillTxFixture) journal(t *testing.T, name string) (SkillInstallPayload, bool) {
	t.Helper()
	parent, err := fileutil.OpenDirectory(f.root)
	if err != nil {
		t.Fatal(err)
	}
	journal, exists, err := dirpublish.ReadJournal(parent, name)
	if closeErr := parent.Close(); err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	if !exists {
		return SkillInstallPayload{}, false
	}
	payload, err := decodeSkillInstallPayload(journal.Payload)
	if err != nil {
		t.Fatal(err)
	}
	return payload, true
}

func requireReviewState(t *testing.T, svc *SkillProposalService, id, state string) SkillReview {
	t.Helper()
	review, err := svc.Store.GetReview(t.Context(), id)
	if err != nil || review.State != state {
		t.Fatalf("review %s = %+v, %v; want %s", id, review, err, state)
	}
	return review
}

// Invariant: every approval of a skill runs through one serialized install transaction whose single
// commit point is the journal reaching published after the install-log entry; anything earlier
// rolls back and an existing skill is never overwritten.
// Owning layer: agentinstance install transaction; canonical suite: TestSkillInstallTransaction.
func TestSkillInstallTransaction(t *testing.T) {
	t.Run("Should re-approve a committed review without writing [UT-019]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		if result, err := f.approve(t, svc, review); err != nil || !result.Committed || result.Warning != nil {
			t.Fatalf("Approve() = %+v, %v", result, err)
		}
		skillFile := filepath.Join(f.root, "release-checklist", "SKILL.md")
		before, err := os.Stat(skillFile)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(f.root)
		if err != nil {
			t.Fatal(err)
		}
		stages := atomic.Int32{}
		counting := withHook(svc, func(step string) bool {
			if step == "prepared" {
				stages.Add(1)
			}
			return false
		})
		if result, err := f.approve(t, counting, review); err != nil || result.Committed {
			t.Fatalf("repeat Approve() = %+v, %v; want an idempotent success", result, err)
		}
		after, err := os.Stat(skillFile)
		if err != nil || !after.ModTime().Equal(before.ModTime()) || stages.Load() != 0 {
			t.Fatalf("repeat approval wrote files: %v %d", err, stages.Load())
		}
		again, err := os.ReadDir(f.root)
		if err != nil || len(again) != len(entries) {
			t.Fatalf("repeat approval changed the target root: %v %v", again, err)
		}
		approved := requireReviewState(t, svc, review.ID, "approved")
		if approved.InstalledDigest != f.digest(t, "release-checklist") || approved.SkillKey !=
			"workspace:ws_a/release-checklist" {
			t.Fatalf("approved review = %+v", approved)
		}
	})
	t.Run("Should roll back a crash after the prepared journal [UT-054]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, crashAt(svc, "prepared"), review); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("Approve() = %v, want the simulated crash", err)
		}
		if _, ok := f.journal(t, "release-checklist"); !ok {
			t.Fatal("crash left no journal")
		}
		f.reconcile(t, f.service(t, "home-a", skillStore(svc)))
		if f.digest(t, "release-checklist") != "" {
			t.Fatal("rollback left a target")
		}
		f.requireClean(t)
		requireReviewState(t, svc, review.ID, SkillReviewPending)
	})
	t.Run("Should restore the original after a crash between the moves [UT-055]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		base := f.install(t, "release-checklist", skillBaseFiles...)
		op := f.replaceOp(t, "release-checklist")
		if err := f.runOp(
			t,
			crashAt(svc, "backup_moved"),
			"release-checklist",
			op,
		); !errors.Is(
			err,
			errSkillInstallCrashed,
		) {
			t.Fatalf("runSkillOp() = %v", err)
		}
		f.reconcile(t, f.service(t, "home-a", skillStore(svc)))
		if got := f.digest(t, "release-checklist"); got != base {
			t.Fatalf("restored digest = %s, want the byte- and mode-exact original %s", got, base)
		}
		f.requireClean(t)
	})
	t.Run("Should remove the staged tree after both moves and allow approving again [UT-056]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, crashAt(svc, "target_moved"), review); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("Approve() = %v", err)
		}
		if f.digest(t, "release-checklist") == "" {
			t.Fatal("crash after both moves left no staged target")
		}
		f.reconcile(t, svc)
		if f.digest(t, "release-checklist") != "" {
			t.Fatal("rollback kept the staged target")
		}
		f.requireClean(t)
		requireReviewState(t, svc, review.ID, SkillReviewPending)
		if result, err := f.approve(t, svc, review); err != nil || !result.Committed {
			t.Fatalf("approve after rollback = %+v, %v", result, err)
		}
		base := f.install(t, "base-skill", skillBaseFiles...)
		op := f.replaceOp(t, "base-skill")
		if err := f.runOp(t, crashAt(svc, "target_moved"), "base-skill", op); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("replace = %v", err)
		}
		f.reconcile(t, svc)
		if f.digest(t, "base-skill") != base {
			t.Fatal("rollback did not restore the replaced skill")
		}
	})
	t.Run("Should only finalize a crash after published, from any home [UT-057]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		homeA := f.service(t, "home-a", nil)
		review := f.propose(t, homeA, "release-checklist", "Check the changelog.")
		if _, err := f.approve(t, crashAt(homeA, "published"), review); !errors.Is(err, errSkillInstallCrashed) {
			t.Fatalf("Approve() = %v", err)
		}
		installed := f.digest(t, "release-checklist")
		f.reconcile(t, f.service(t, "home-b", nil))
		if f.digest(t, "release-checklist") != installed || installed == "" {
			t.Fatal("finalize changed the committed tree")
		}
		f.requireClean(t)
		requireReviewState(t, homeA, review.ID, SkillReviewPending)
		f.reconcile(t, homeA)
		if approved := requireReviewState(t, homeA, review.ID, "approved"); approved.InstalledDigest != installed {
			t.Fatalf("projected digest = %s, want %s", approved.InstalledDigest, installed)
		}
	})
	t.Run("Should run one transaction for two concurrent approvals [UT-058]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		stages := atomic.Int32{}
		counting := withHook(svc, func(step string) bool {
			if step == "incoming_created" {
				stages.Add(1)
			}
			return false
		})
		results := make([]SkillInstallResult, 2)
		errs := make([]error, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range 2 {
			wg.Go(func() {
				<-start
				results[i], errs[i] = f.approve(t, counting, review)
			})
		}
		close(start)
		wg.Wait()
		committed := 0
		for i := range 2 {
			if errs[i] != nil {
				t.Fatalf("approval %d = %v", i, errs[i])
			}
			if results[i].Committed {
				committed++
			}
		}
		if committed != 1 || stages.Load() != 1 {
			t.Fatalf("committed = %d, stage directories = %d; want 1 and 1", committed, stages.Load())
		}
	})
	t.Run("Should decide once when reject and approve race [UT-059]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		var approveErr, rejectErr error
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() { <-start; _, approveErr = f.approve(t, svc, review) })
		wg.Go(func() {
			<-start
			rejectErr = svc.Reject(t.Context(), skillOperator, f.instance, review.ID, review.Digest, "operator", f.root)
		})
		close(start)
		wg.Wait()
		switch {
		case approveErr == nil && errors.Is(rejectErr, ErrSkillReviewStale):
			requireReviewState(t, svc, review.ID, "approved")
		case rejectErr == nil && errors.Is(approveErr, ErrSkillReviewStale):
			requireReviewState(t, svc, review.ID, "rejected")
			if f.digest(t, "release-checklist") != "" {
				t.Fatal("rejected review left an installed skill")
			}
		default:
			t.Fatalf("approve = %v, reject = %v; want exactly one decision", approveErr, rejectErr)
		}
		f.requireClean(t)
	})
	t.Run("Should refuse a new review whose target exists [UT-081]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		review := f.propose(t, svc, "release-checklist", "Check the changelog.")
		existing := f.install(t, "release-checklist", skillBaseFiles...)
		if _, err := f.approve(t, svc, review); !errors.Is(err, ErrSkillReviewTargetExists) {
			t.Fatalf("Approve() = %v, want skill_review_target_exists", err)
		}
		if f.digest(t, "release-checklist") != existing {
			t.Fatal("the existing skill changed")
		}
		requireReviewState(t, svc, review.ID, SkillReviewPending)
		late := f.propose(t, svc, "late-skill", "Arrives late.")
		var created string
		racing := withHook(svc, func(step string) bool {
			if step == "incoming_ready" {
				created = f.install(t, "late-skill", skillBaseFiles...)
			}
			return false
		})
		if _, err := f.approve(t, racing, late); !errors.Is(err, ErrSkillReviewTargetExists) {
			t.Fatalf("Approve() with a racing create = %v, want skill_review_target_exists", err)
		}
		if f.digest(t, "late-skill") != created {
			t.Fatal("the racing skill changed")
		}
		requireReviewState(t, svc, late.ID, SkillReviewPending)
		f.requireClean(t)
	})
	t.Run("Should return busy on every operation while another process holds the lock [UT-083]", func(t *testing.T) {
		t.Parallel()
		f := newSkillTxFixture(t)
		svc := f.service(t, "home-a", nil)
		approveMe := f.propose(t, svc, "release-checklist", "Check the changelog.")
		rejectMe := f.propose(t, svc, "release-checklist", "Check the changelog twice.")
		if err := os.MkdirAll(f.root, 0o700); err != nil {
			t.Fatal(err)
		}
		parent, err := fileutil.OpenDirectoryForMutation(f.root)
		if err != nil {
			t.Fatal(err)
		}
		lock, err := dirpublish.AcquireLock(parent, "release-checklist")
		if err != nil {
			t.Fatal(err)
		}
		count := len(skillStore(svc).reviews)
		_, err = svc.Propose(t.Context(), skillOperator, f.instance, f.root, SkillProposalInput{
			Name: "release-checklist", Description: "Busy.", Body: "Busy body.",
			Lesson:             SkillLesson{Cue: "c", Lesson: "l", Evidence: "e"},
			EvidenceSessionIDs: []string{"sess-a"},
		})
		if !errors.Is(err, ErrSkillInstallBusy) || len(skillStore(svc).reviews) != count {
			t.Fatalf("Propose() = %v with %d reviews; want busy and no new review", err, len(skillStore(svc).reviews))
		}
		if _, err := f.approve(t, svc, approveMe); !errors.Is(err, ErrSkillInstallBusy) {
			t.Fatalf("Approve() = %v, want busy", err)
		}
		if err := svc.Reject(t.Context(), skillOperator, f.instance, rejectMe.ID, rejectMe.Digest, "operator",
			f.root); !errors.Is(err, ErrSkillInstallBusy) {
			t.Fatalf("Reject() = %v, want busy", err)
		}
		requireReviewState(t, svc, approveMe.ID, SkillReviewPending)
		requireReviewState(t, svc, rejectMe.ID, SkillReviewPending)
		if err := errors.Join(lock.Close(), parent.Close()); err != nil {
			t.Fatal(err)
		}
		if result, err := f.approve(t, svc, approveMe); err != nil || !result.Committed {
			t.Fatalf("Approve() after release = %+v, %v", result, err)
		}
	})
}

func installLogWriters(t *testing.T, root, name string) []string {
	t.Helper()
	entries, err := readSkillInstallLog(root, name)
	if err != nil {
		t.Fatal(err)
	}
	writers := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Current == nil {
			writers = append(writers, "")
			continue
		}
		writers = append(writers, entry.Current.ReviewID)
	}
	return writers
}
