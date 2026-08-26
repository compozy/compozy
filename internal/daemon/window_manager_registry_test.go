package daemon

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
)

// barrierClientState lets one store write be held open on demand.
//
// The deletion race is about a commit that is *already inside* the store call when
// the purge starts, which no amount of goroutine scheduling can reproduce on
// purpose — so the test drives that state directly instead of hoping for it.
type barrierClientState struct {
	clientstate.Service

	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
}

// workspaceDeletingClientState models the client-state boundary after workspace
// removal has acquired its deletion gate: regular writes are rejected while the
// workspace purge owns physical cleanup.
type workspaceDeletingClientState struct {
	clientstate.Service
}

func (s workspaceDeletingClientState) Apply(
	context.Context,
	clientstate.WorkspaceID,
	string,
	[]clientstate.Op,
	clientstate.ApplyOptions,
) ([]clientstate.Entry, error) {
	return nil, clientstate.ErrWorkspaceNotFound
}

func newBarrierClientState(service clientstate.Service) *barrierClientState {
	return &barrierClientState{Service: service}
}

// arm holds the next write to the given key open until the returned release is
// called, and reports when that write has entered the store.
func (b *barrierClientState) arm() (entered <-chan struct{}, release func()) {
	enteredCh := make(chan struct{})
	releaseCh := make(chan struct{})
	b.mu.Lock()
	b.armed = true
	b.entered = enteredCh
	b.release = releaseCh
	b.mu.Unlock()
	return enteredCh, sync.OnceFunc(func() { close(releaseCh) })
}

func (b *barrierClientState) takeBarrier(ops []clientstate.Op) (chan struct{}, chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.armed {
		return nil, nil
	}
	held := false
	for _, op := range ops {
		if op.Kind == clientstate.OpPut && strings.HasPrefix(op.Key, windowManagerSnapshotKeyStem+":") {
			held = true
		}
	}
	if !held {
		return nil, nil
	}
	b.armed = false
	return b.entered, b.release
}

func (b *barrierClientState) Apply(
	ctx context.Context,
	ws clientstate.WorkspaceID,
	domain string,
	ops []clientstate.Op,
	opts clientstate.ApplyOptions,
) ([]clientstate.Entry, error) {
	entered, release := b.takeBarrier(ops)
	if entered != nil {
		close(entered)
		<-release
	}
	return b.Service.Apply(ctx, ws, domain, ops, opts)
}

// blockingWorkspaceAuthorizer holds one workspace resolution open on demand.
//
// Every client registration resolves its workspace first, which makes it the one
// point where a claim can be parked *inside* the operation — the only way to force
// two claims for the same client id to genuinely overlap rather than hope they do.
type blockingWorkspaceAuthorizer struct {
	inner windowmanager.WorkspaceResolver

	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
}

func newBlockingWorkspaceAuthorizer(
	inner windowmanager.WorkspaceResolver,
) *blockingWorkspaceAuthorizer {
	return &blockingWorkspaceAuthorizer{inner: inner}
}

func (b *blockingWorkspaceAuthorizer) arm() (entered <-chan struct{}, release func()) {
	enteredCh := make(chan struct{})
	releaseCh := make(chan struct{})
	b.mu.Lock()
	b.armed = true
	b.entered = enteredCh
	b.release = releaseCh
	b.mu.Unlock()
	return enteredCh, sync.OnceFunc(func() { close(releaseCh) })
}

func (b *blockingWorkspaceAuthorizer) takeBarrier() (chan struct{}, chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.armed {
		return nil, nil
	}
	b.armed = false
	return b.entered, b.release
}

func (b *blockingWorkspaceAuthorizer) ResolveWorkspace(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) error {
	entered, release := b.takeBarrier()
	if entered != nil {
		close(entered)
		<-release
	}
	return b.inner.ResolveWorkspace(ctx, workspaceID)
}

func TestWindowManagerRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should treat a workspace purge race as already cleaned", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const profileID = "01JQPROFILERACE00000000000"
		manager, err := fixture.registry.For(profileID)
		if err != nil {
			t.Fatalf("For(profile) error = %v", err)
		}
		executeDaemonDesktopCreate(t, manager, workspaceID, "desktop-race", "Race")

		deleting := workspaceDeletingClientState{Service: fixture.engine}
		registry := newTestWindowManagerRegistry(t, &fixture, deleting)
		if err := registry.PurgeDesktopPartitions(ctx, profileID); err != nil {
			t.Fatalf("PurgeDesktopPartitions() error = %v, want workspace purge race to be ignored", err)
		}
	})

	t.Run("Should wait for an in-flight commit before removing a profile's desktops", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const doomed = "01JQPROFILEBARRIER00000000"
		barrier := newBarrierClientState(fixture.engine)
		registry := newTestWindowManagerRegistry(t, &fixture, barrier)

		manager, err := registry.For(doomed)
		if err != nil {
			t.Fatalf("For(doomed) error = %v", err)
		}
		executeDaemonDesktopCreate(t, manager, workspaceID, "desktop-second", "Second")
		snapshot, err := manager.Snapshot(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}

		entered, release := barrier.arm()
		commitDone := make(chan error, 1)
		go func() {
			_, execErr := manager.Execute(ctx, windowmanager.CommandRequest{
				WorkspaceID:      workspaceID,
				ExpectedRevision: snapshot.Revision,
				Payload: windowmanager.CreateDesktopCommand{
					DesktopID: "desktop-third", Name: "Third",
				},
			})
			commitDone <- execErr
		}()
		<-entered

		purgeDone := make(chan error, 1)
		go func() { purgeDone <- registry.PurgeDesktopPartitions(ctx, doomed) }()

		// The purge must be blocked behind the write that is already in the store.
		select {
		case err := <-purgeDone:
			t.Fatalf("PurgeDesktopPartitions() returned %v while a commit was mid-write", err)
		case <-time.After(200 * time.Millisecond):
		}

		release()
		if err := <-commitDone; err != nil {
			t.Fatalf("Execute(in-flight) error = %v, want the commit to land", err)
		}
		if err := <-purgeDone; err != nil {
			t.Fatalf("PurgeDesktopPartitions() error = %v", err)
		}

		// The write that landed first was still enumerated and removed.
		remaining, err := registry.CountDesktopPartitions(ctx, doomed)
		if err != nil {
			t.Fatalf("CountDesktopPartitions() error = %v", err)
		}
		if remaining != 0 {
			t.Fatalf("CountDesktopPartitions() = %d, want 0", remaining)
		}

		// And nothing can write through the manager the caller still holds.
		_, err = manager.Execute(ctx, windowmanager.CommandRequest{
			WorkspaceID:      workspaceID,
			ExpectedRevision: 0,
			Payload:          windowmanager.CreateDesktopCommand{DesktopID: "desktop-late", Name: "Late"},
		})
		if !errors.Is(err, errWindowManagerProfileDeleted) &&
			!errors.Is(err, windowmanager.ErrClosed) {
			t.Fatalf("Execute(after purge) error = %v, want a deleted-profile refusal", err)
		}
		after, err := registry.CountDesktopPartitions(ctx, doomed)
		if err != nil {
			t.Fatalf("CountDesktopPartitions(after late write) error = %v", err)
		}
		if after != 0 {
			t.Fatalf("CountDesktopPartitions(after late write) = %d, want 0", after)
		}
	})

	t.Run("Should refuse to rebuild a deleted profile and stay idempotent", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const (
			doomed   = "01JQPROFILEGONE00000000000"
			survivor = "01JQPROFILEKEPT00000000000"
		)
		for _, profileID := range []string{doomed, survivor} {
			manager, err := fixture.registry.For(profileID)
			if err != nil {
				t.Fatalf("For(%s) error = %v", profileID, err)
			}
			executeDaemonDesktopCreate(t, manager, workspaceID, "desktop-second", "Second")
			executeDaemonDesktopCreate(
				t,
				manager,
				windowmanager.GlobalDesktopWorkspaceID,
				"desktop-global",
				"Global",
			)
		}

		if err := fixture.registry.PurgeDesktopPartitions(ctx, doomed); err != nil {
			t.Fatalf("PurgeDesktopPartitions() error = %v", err)
		}
		if err := fixture.registry.PurgeDesktopPartitions(ctx, doomed); err != nil {
			t.Fatalf("PurgeDesktopPartitions(repeat) error = %v", err)
		}
		if _, err := fixture.registry.For(doomed); !errors.Is(err, errWindowManagerProfileDeleted) {
			t.Fatalf("For(deleted) error = %v, want errWindowManagerProfileDeleted", err)
		}
		survivorCount, err := fixture.registry.CountDesktopPartitions(ctx, survivor)
		if err != nil {
			t.Fatalf("CountDesktopPartitions(survivor) error = %v", err)
		}
		if survivorCount != 2 {
			t.Fatalf("CountDesktopPartitions(survivor) = %d, want 2", survivorCount)
		}
	})

	t.Run("Should finish an interrupted delete's desktop purge at boot", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const doomed = "01JQPROFILECRASHED00000000"
		manager, err := fixture.registry.For(doomed)
		if err != nil {
			t.Fatalf("For(doomed) error = %v", err)
		}
		executeDaemonDesktopCreate(t, manager, workspaceID, "desktop-second", "Second")

		state := &bootState{
			windowManagerBootState: windowManagerBootState{windowManagers: fixture.registry},
			profiles:               newDaemonTestProfileManager(t, &fixture),
		}
		seedInterruptedProfileDelete(ctx, t, &fixture, doomed)

		// The composition the boot sequence calls: window managers are up, so the
		// step this delete still owes can finally run.
		if err := recoverProfiles(ctx, state); err != nil {
			t.Fatalf("recoverProfiles() error = %v", err)
		}
		remaining, err := fixture.registry.CountDesktopPartitions(ctx, doomed)
		if err != nil {
			t.Fatalf("CountDesktopPartitions() error = %v", err)
		}
		if remaining != 0 {
			t.Fatalf("CountDesktopPartitions(after recovery) = %d, want 0", remaining)
		}
	})

	t.Run("Should settle overlapping claims for one client id on a single profile", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const (
			profileA = "01JQPROFILECLAIMA000000000"
			profileB = "01JQPROFILECLAIMB000000000"
			clientID = windowmanager.ClientID("client:web")
		)
		gate := newBlockingWorkspaceAuthorizer(
			windowManagerWorkspaceAuthorizer{resolver: fixture.storeResolver},
		)
		registry := newTestWindowManagerRegistryWithAuthorizer(t, &fixture, fixture.engine, gate)
		registration := windowmanager.ClientRegistration{WorkspaceID: workspaceID, ClientID: clientID}

		// A enters the claim and is held inside its registration; B then has to wait
		// for the whole claim rather than slipping between A's retire and register.
		entered, release := gate.arm()
		claimA := make(chan error, 1)
		go func() {
			_, err := registry.ClaimClient(ctx, profileA, registration)
			claimA <- err
		}()
		<-entered

		claimB := make(chan error, 1)
		go func() {
			_, err := registry.ClaimClient(ctx, profileB, registration)
			claimB <- err
		}()
		select {
		case err := <-claimB:
			t.Fatalf("ClaimClient(B) returned %v while A held the claim", err)
		case <-time.After(200 * time.Millisecond):
		}

		release()
		if err := <-claimA; err != nil {
			t.Fatalf("ClaimClient(A) error = %v", err)
		}
		if err := <-claimB; err != nil {
			t.Fatalf("ClaimClient(B) error = %v", err)
		}

		// Whoever entered last owns the attachment, and there is only ever one.
		attached, err := registry.ClientsInWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ClientsInWorkspace() error = %v", err)
		}
		if len(attached) != 1 || attached[0].ClientID != clientID {
			t.Fatalf("ClientsInWorkspace() = %#v, want exactly one %q", attached, clientID)
		}
		winner, err := registry.For(profileB)
		if err != nil {
			t.Fatalf("For(B) error = %v", err)
		}
		owner, err := registry.ManagerForClient(ctx, workspaceID, clientID)
		if err != nil {
			t.Fatalf("ManagerForClient() error = %v", err)
		}
		if owner != winner {
			t.Fatalf("ManagerForClient() resolved a profile that lost the claim")
		}
	})

	t.Run("Should keep one client id attached to a single profile across a switch", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const (
			from     = "01JQPROFILEFROM00000000000"
			to       = "01JQPROFILETO000000000000"
			clientID = windowmanager.ClientID("client:web")
		)
		registration := windowmanager.ClientRegistration{WorkspaceID: workspaceID, ClientID: clientID}
		if _, err := fixture.registry.ClaimClient(ctx, from, registration); err != nil {
			t.Fatalf("ClaimClient(from) error = %v", err)
		}

		// Switching profiles re-registers the same browser tab; the claim moves it.
		if _, err := fixture.registry.ClaimClient(ctx, to, registration); err != nil {
			t.Fatalf("ClaimClient(to) error = %v", err)
		}
		after, err := fixture.registry.For(to)
		if err != nil {
			t.Fatalf("For(to) error = %v", err)
		}

		attached, err := fixture.registry.ClientsInWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ClientsInWorkspace() error = %v", err)
		}
		if len(attached) != 1 || attached[0].ClientID != clientID {
			t.Fatalf("ClientsInWorkspace() = %#v, want exactly one %q", attached, clientID)
		}
		owner, err := fixture.registry.ManagerForClient(ctx, workspaceID, clientID)
		if err != nil {
			t.Fatalf("ManagerForClient() error = %v", err)
		}
		if owner != after {
			t.Fatalf("ManagerForClient() resolved the profile the client left")
		}
	})
}
