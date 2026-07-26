package daemon

import (
	"context"
	"strings"
	"testing"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/resources"
)

type automationRuntimeOnlyStub struct {
	automationRuntime
}

type automationRuntimeTargetStub struct {
	automationRuntime
	automationResourceProjectorTarget
}

type rawStoreStub struct {
	resources.RawStore
}

type automationProjectionPlanStub struct{}

func (automationProjectionPlanStub) Kind() resources.ResourceKind { return "automation.test" }
func (automationProjectionPlanStub) Revision() int64              { return 1 }
func (automationProjectionPlanStub) OperationCount() int          { return 1 }

type automationProjectorTargetStub struct {
	jobBuildRecords     []resources.Record[automationpkg.Job]
	triggerBuildRecords []resources.Record[automationpkg.Trigger]
	jobApplyCount       int
	triggerApplyCount   int
}

func (s *automationProjectorTargetStub) BuildJobResourceState(
	_ context.Context,
	records []resources.Record[automationpkg.Job],
) (resources.ProjectionPlan, error) {
	s.jobBuildRecords = append([]resources.Record[automationpkg.Job](nil), records...)
	return automationProjectionPlanStub{}, nil
}

func (s *automationProjectorTargetStub) ApplyJobResourceState(
	_ context.Context,
	_ resources.ProjectionPlan,
) error {
	s.jobApplyCount++
	return nil
}

func (s *automationProjectorTargetStub) BuildTriggerResourceState(
	_ context.Context,
	records []resources.Record[automationpkg.Trigger],
) (resources.ProjectionPlan, error) {
	s.triggerBuildRecords = append([]resources.Record[automationpkg.Trigger](nil), records...)
	return automationProjectionPlanStub{}, nil
}

func (s *automationProjectorTargetStub) ApplyTriggerResourceState(
	_ context.Context,
	_ resources.ProjectionPlan,
) error {
	s.triggerApplyCount++
	return nil
}

var _ automationRuntime = automationRuntimeOnlyStub{}
var _ automationRuntime = automationRuntimeTargetStub{}
var _ automationResourceProjectorTarget = automationRuntimeTargetStub{}
var _ resources.RawStore = rawStoreStub{}

func TestAutomationResourceTargetReturnsProjectorTargetOnlyWhenSupported(t *testing.T) {
	t.Parallel()

	if got := automationResourceTarget(nil); got != nil {
		t.Fatalf("automationResourceTarget(nil) = %#v, want nil", got)
	}
	if got := automationResourceTarget(automationRuntimeOnlyStub{}); got != nil {
		t.Fatalf("automationResourceTarget(runtime without projector target) = %#v, want nil", got)
	}
	if got := automationResourceTarget(automationRuntimeTargetStub{}); got == nil {
		t.Fatal("automationResourceTarget(runtime with projector target) = nil, want target")
	}
}

func TestAutomationResourceStoresRejectsPartialWiring(t *testing.T) {
	t.Parallel()

	if jobs, triggers, err := automationResourceStores(nil, nil); err != nil || jobs != nil || triggers != nil {
		t.Fatalf(
			"automationResourceStores(nil, nil) = (%#v, %#v, %v), want (nil, nil, nil)",
			jobs,
			triggers,
			err,
		)
	}

	if _, _, err := automationResourceStores(nil, resources.NewCodecRegistry()); err == nil {
		t.Fatal("automationResourceStores(nil, codecs) error = nil, want missing raw store failure")
	} else if !strings.Contains(err.Error(), "raw store is required") {
		t.Fatalf("automationResourceStores(nil, codecs) error = %v, want raw store context", err)
	}

	if _, _, err := automationResourceStores(rawStoreStub{}, nil); err == nil {
		t.Fatal("automationResourceStores(raw, nil) error = nil, want missing codec registry failure")
	} else if !strings.Contains(err.Error(), "codec registry is required") {
		t.Fatalf("automationResourceStores(raw, nil) error = %v, want codec registry context", err)
	}
}

func TestAutomationProjectorsDelegateBuildAndApply(t *testing.T) {
	t.Parallel()

	target := &automationProjectorTargetStub{}

	if got := newAutomationJobProjector(nil); got != nil {
		t.Fatalf("newAutomationJobProjector(nil) = %#v, want nil", got)
	}
	jobProjector := newAutomationJobProjector(target)
	if jobProjector == nil {
		t.Fatal("newAutomationJobProjector() = nil, want projector")
	}
	if got, want := jobProjector.Kind(), automationpkg.JobResourceKind; got != want {
		t.Fatalf("jobProjector.Kind() = %q, want %q", got, want)
	}
	jobPlan, err := jobProjector.Build(context.Background(), []resources.Record[automationpkg.Job]{{ID: "job-1"}})
	if err != nil {
		t.Fatalf("jobProjector.Build() error = %v", err)
	}
	if err := jobProjector.Apply(context.Background(), jobPlan); err != nil {
		t.Fatalf("jobProjector.Apply() error = %v", err)
	}
	if got, want := len(target.jobBuildRecords), 1; got != want {
		t.Fatalf("len(target.jobBuildRecords) = %d, want %d", got, want)
	}
	if got, want := target.jobApplyCount, 1; got != want {
		t.Fatalf("target.jobApplyCount = %d, want %d", got, want)
	}

	if got := newAutomationTriggerProjector(nil); got != nil {
		t.Fatalf("newAutomationTriggerProjector(nil) = %#v, want nil", got)
	}
	triggerProjector := newAutomationTriggerProjector(target)
	if triggerProjector == nil {
		t.Fatal("newAutomationTriggerProjector() = nil, want projector")
	}
	if got, want := triggerProjector.Kind(), automationpkg.TriggerResourceKind; got != want {
		t.Fatalf("triggerProjector.Kind() = %q, want %q", got, want)
	}
	triggerPlan, err := triggerProjector.Build(
		context.Background(),
		[]resources.Record[automationpkg.Trigger]{{ID: "trigger-1"}},
	)
	if err != nil {
		t.Fatalf("triggerProjector.Build() error = %v", err)
	}
	if err := triggerProjector.Apply(context.Background(), triggerPlan); err != nil {
		t.Fatalf("triggerProjector.Apply() error = %v", err)
	}
	if got, want := len(target.triggerBuildRecords), 1; got != want {
		t.Fatalf("len(target.triggerBuildRecords) = %d, want %d", got, want)
	}
	if got, want := target.triggerApplyCount, 1; got != want {
		t.Fatalf("target.triggerApplyCount = %d, want %d", got, want)
	}

	var nilJobProjector *automationJobProjector
	if _, err := nilJobProjector.Build(context.Background(), nil); err == nil {
		t.Fatal("nil job projector Build() error = nil, want target failure")
	}
	if err := nilJobProjector.Apply(context.Background(), automationProjectionPlanStub{}); err == nil {
		t.Fatal("nil job projector Apply() error = nil, want target failure")
	}

	var nilTriggerProjector *automationTriggerProjector
	if _, err := nilTriggerProjector.Build(context.Background(), nil); err == nil {
		t.Fatal("nil trigger projector Build() error = nil, want target failure")
	}
	if err := nilTriggerProjector.Apply(context.Background(), automationProjectionPlanStub{}); err == nil {
		t.Fatal("nil trigger projector Apply() error = nil, want target failure")
	}
}

func TestAutomationResourceStoresBuildTypedStoresWhenCodecsRegistered(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}

	jobCodec, err := automationpkg.NewJobResourceCodec()
	if err != nil {
		t.Fatalf("automationpkg.NewJobResourceCodec() error = %v", err)
	}
	triggerCodec, err := automationpkg.NewTriggerResourceCodec()
	if err != nil {
		t.Fatalf("automationpkg.NewTriggerResourceCodec() error = %v", err)
	}

	codecs := resources.NewCodecRegistry()
	if err := resources.RegisterCodec(codecs, jobCodec); err != nil {
		t.Fatalf("RegisterCodec(job) error = %v", err)
	}
	if err := resources.RegisterCodec(codecs, triggerCodec); err != nil {
		t.Fatalf("RegisterCodec(trigger) error = %v", err)
	}

	jobStore, triggerStore, err := automationResourceStores(kernel, codecs)
	if err != nil {
		t.Fatalf("automationResourceStores() error = %v", err)
	}
	if jobStore == nil {
		t.Fatal("jobStore = nil, want typed job store")
	}
	if triggerStore == nil {
		t.Fatal("triggerStore = nil, want typed trigger store")
	}
}
