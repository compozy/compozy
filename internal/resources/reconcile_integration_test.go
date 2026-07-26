//go:build integration

package resources

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestReconcileDriverRunBootTopologyIntegration(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)

	var mu sync.Mutex
	var order []ResourceKind
	recordOrder := func(kind ResourceKind) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, kind)
	}

	driver, err := NewReconcileDriver(
		kernel,
		testDaemonActor(),
		[]ProjectorRegistration{
			newTestProjectorRegistration(ResourceKind("tool"), nil,
				func(context.Context, projectionInput) (ProjectionPlan, error) {
					recordOrder(ResourceKind("tool"))
					return testPlan{kind: ResourceKind("tool"), revision: 1, operations: 1}, nil
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
			newTestProjectorRegistration(ResourceKind("agent"), []ResourceKind{ResourceKind("tool")},
				func(context.Context, projectionInput) (ProjectionPlan, error) {
					recordOrder(ResourceKind("agent"))
					return testPlan{kind: ResourceKind("agent"), revision: 1, operations: 1}, nil
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
			newTestProjectorRegistration(ResourceKind("bundle.activation"), []ResourceKind{ResourceKind("agent")},
				func(context.Context, projectionInput) (ProjectionPlan, error) {
					recordOrder(ResourceKind("bundle.activation"))
					return testPlan{kind: ResourceKind("bundle.activation"), revision: 1, operations: 1}, nil
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
		},
	)
	if err != nil {
		t.Fatalf("NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if closeErr := driver.Close(closeCtx); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	if err := driver.RunBoot(testutil.Context(t)); err != nil {
		t.Fatalf("RunBoot() error = %v", err)
	}

	mu.Lock()
	gotOrder := append([]ResourceKind(nil), order...)
	mu.Unlock()
	wantOrder := []ResourceKind{ResourceKind("tool"), ResourceKind("agent"), ResourceKind("bundle.activation")}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("boot order = %#v, want %#v", gotOrder, wantOrder)
	}
	for idx := range wantOrder {
		if gotOrder[idx] != wantOrder[idx] {
			t.Fatalf("boot order[%d] = %q, want %q (full=%#v)", idx, gotOrder[idx], wantOrder[idx], gotOrder)
		}
	}
}

func TestReconcileDriverRunBootRejectsInvalidGraphIntegration(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	driver, err := NewReconcileDriver(
		kernel,
		testDaemonActor(),
		[]ProjectorRegistration{
			newTestProjectorRegistration(ResourceKind("tool"), []ResourceKind{ResourceKind("agent")},
				func(context.Context, projectionInput) (ProjectionPlan, error) {
					return testPlan{kind: ResourceKind("tool"), revision: 1, operations: 1}, nil
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
			newTestProjectorRegistration(ResourceKind("agent"), []ResourceKind{ResourceKind("tool")},
				func(context.Context, projectionInput) (ProjectionPlan, error) {
					return testPlan{kind: ResourceKind("agent"), revision: 1, operations: 1}, nil
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
		},
	)
	if err != nil {
		t.Fatalf("NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if closeErr := driver.Close(closeCtx); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	if err := driver.RunBoot(testutil.Context(t)); !errors.Is(err, ErrValidation) {
		t.Fatalf("RunBoot() error = %v, want ErrValidation", err)
	}
}

func TestReconcileDriverWriteStormCoalescesIntegration(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)

	var mu sync.Mutex
	buildCalls := 0
	firstBuildStarted := make(chan struct{})
	releaseFirstBuild := make(chan struct{})

	driver, err := NewReconcileDriver(
		kernel,
		testDaemonActor(),
		[]ProjectorRegistration{
			newTestProjectorRegistration(testResourceKind, nil,
				func(context.Context, projectionInput) (ProjectionPlan, error) {
					mu.Lock()
					buildCalls++
					call := buildCalls
					mu.Unlock()

					if call == 1 {
						close(firstBuildStarted)
						<-releaseFirstBuild
					}
					return testPlan{kind: testResourceKind, revision: int64(call), operations: 1}, nil
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
		},
		WithReconcileCoalesceWindow(25*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if closeErr := driver.Close(closeCtx); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
		t.Fatalf("Trigger(first) error = %v", err)
	}
	<-firstBuildStarted

	for i := 0; i < 25; i++ {
		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(storm %d) error = %v", i, err)
		}
	}

	close(releaseFirstBuild)

	waitForCondition(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return buildCalls == 2
	})
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	gotBuildCalls := buildCalls
	mu.Unlock()
	if gotBuildCalls != 2 {
		t.Fatalf("buildCalls = %d, want 2", gotBuildCalls)
	}
}

func TestReconcileDriverCloseCancelsInFlightWorkIntegration(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)

	buildStarted := make(chan struct{})
	driver, err := NewReconcileDriver(
		kernel,
		testDaemonActor(),
		[]ProjectorRegistration{
			newTestProjectorRegistration(testResourceKind, nil,
				func(ctx context.Context, _ projectionInput) (ProjectionPlan, error) {
					close(buildStarted)
					<-ctx.Done()
					return nil, ctx.Err()
				},
				func(context.Context, ProjectionPlan) error { return nil },
			),
		},
	)
	if err != nil {
		t.Fatalf("NewReconcileDriver() error = %v", err)
	}

	if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}
	<-buildStarted

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := driver.Close(closeCtx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err == nil {
		t.Fatal("Trigger(after Close) error = nil, want non-nil")
	}
}
