package reconciler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	pkgcfg "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/romdo/go-debounce"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type depKey struct {
	typ resources.ResourceType
	id  string
}

type Reconciler struct {
	store          resources.ResourceStore
	state          *appstate.State
	proj           *project.Config
	sched          schedule.Manager
	mode           string
	registry       *autoload.ConfigRegistry
	idx            map[resources.ResourceType]map[string]map[string]struct{}
	wfdep          map[string][]depKey
	idxMu          sync.RWMutex
	in             chan resources.Event
	deb            func()
	cancelDebounce func()
	pendMu         sync.Mutex
	pending        []resources.Event
	eventsTotal    metric.Int64Counter
	eventsDropped  metric.Int64Counter
	recompileTotal metric.Int64Counter
	batchDur       metric.Float64Histogram
	wg             sync.WaitGroup
	runCtx         context.Context
	runCancel      context.CancelFunc
}

func newReconcilerMetrics(
	ctx context.Context,
) (metric.Int64Counter, metric.Int64Counter, metric.Int64Counter, metric.Float64Histogram) {
	meter := otel.GetMeterProvider().Meter("compozy")
	evTot, err1 := meter.Int64Counter(
		"compozy_reconciler_events_total",
		metric.WithDescription("Total store events received"),
	)
	if err1 != nil {
		logger.FromContext(ctx).Error("meter creation failed", "error", err1)
	}
	evDrop, err2 := meter.Int64Counter(
		"compozy_reconciler_events_dropped_total",
		metric.WithDescription("Total store events dropped due to backpressure"),
	)
	if err2 != nil {
		logger.FromContext(ctx).Error("meter creation failed", "error", err2)
	}
	recTot, err3 := meter.Int64Counter(
		"compozy_reconciler_recompile_total",
		metric.WithDescription("Total recompilations attempted"),
	)
	if err3 != nil {
		logger.FromContext(ctx).Error("meter creation failed", "error", err3)
	}
	bDur, err4 := meter.Float64Histogram(
		"compozy_reconciler_batch_duration_seconds",
		metric.WithDescription("Batch processing duration seconds"),
	)
	if err4 != nil {
		logger.FromContext(ctx).Error("meter creation failed", "error", err4)
	}
	return evTot, evDrop, recTot, bDur
}

const (
	defaultQueueCap        = 1024
	defaultDebounceWait    = 300 * time.Millisecond
	defaultDebounceMaxWait = 500 * time.Millisecond
)

const (
	modeRepo    = "repo"
	modeBuilder = "builder"
)

func resolveStore(state *appstate.State) (resources.ResourceStore, error) {
	v, ok := state.ResourceStore()
	if !ok {
		return nil, fmt.Errorf("resource store not found in state")
	}
	store, ok := v.(resources.ResourceStore)
	if !ok {
		return nil, fmt.Errorf("invalid resource store type")
	}
	return store, nil
}

func resolveSched(state *appstate.State) (schedule.Manager, error) {
	sm, ok := state.ScheduleManager()
	if !ok {
		return nil, fmt.Errorf("schedule manager not set")
	}
	sched, ok := sm.(schedule.Manager)
	if !ok {
		return nil, fmt.Errorf("invalid schedule manager type")
	}
	return sched, nil
}

func deriveSettings(cfg *pkgcfg.Config) (string, int, time.Duration, time.Duration) {
	mode := modeRepo
	queueSize := defaultQueueCap
	debWait := defaultDebounceWait
	debMaxWait := defaultDebounceMaxWait
	if cfg != nil {
		if cfg.Server.SourceOfTruth == modeBuilder {
			mode = modeBuilder
		}
		if v := cfg.Server.Reconciler.QueueCapacity; v > 0 {
			queueSize = v
		}
		if v := cfg.Server.Reconciler.DebounceWait; v > 0 {
			debWait = v
		}
		if v := cfg.Server.Reconciler.DebounceMaxWait; v > 0 {
			debMaxWait = v
		}
	}
	return mode, queueSize, debWait, debMaxWait
}

func resolveProject(state *appstate.State) (*project.Config, error) {
	proj := state.ProjectConfig
	if proj == nil || proj.Name == "" {
		return nil, fmt.Errorf("project config is required")
	}
	return proj, nil
}

func resolveRegistry(state *appstate.State) *autoload.ConfigRegistry {
	var reg *autoload.ConfigRegistry
	if v, ok := state.ConfigRegistry(); ok {
		if rr, ok2 := v.(*autoload.ConfigRegistry); ok2 {
			reg = rr
		}
	}
	return reg
}

func makeDebouncer(ctx context.Context, r *Reconciler, debWait, debMaxWait time.Duration) (func(), func()) {
	debFn, cancel := debounce.NewWithMaxWait(debWait, debMaxWait, func() {
		c := r.runCtx
		if c == nil {
			c = ctx
		}
		r.onDebounceFire(c)
	})
	return debFn, cancel
}

func NewReconciler(ctx context.Context, state *appstate.State) (*Reconciler, error) {
	log := logger.FromContext(ctx)
	if state == nil {
		return nil, fmt.Errorf("nil state")
	}
	store, err := resolveStore(state)
	if err != nil {
		return nil, err
	}
	sched, err := resolveSched(state)
	if err != nil {
		return nil, err
	}
	evTot, evDrop, recTot, bDur := newReconcilerMetrics(ctx)
	mode, queueSize, debWait, debMaxWait := deriveSettings(pkgcfg.FromContext(ctx))
	proj, err := resolveProject(state)
	if err != nil {
		return nil, err
	}
	reg := resolveRegistry(state)
	r := &Reconciler{
		store:          store,
		state:          state,
		proj:           proj,
		sched:          sched,
		mode:           mode,
		registry:       reg,
		idx:            make(map[resources.ResourceType]map[string]map[string]struct{}),
		wfdep:          make(map[string][]depKey),
		in:             make(chan resources.Event, queueSize),
		eventsTotal:    evTot,
		eventsDropped:  evDrop,
		recompileTotal: recTot,
		batchDur:       bDur,
	}
	debFn, cancel := makeDebouncer(ctx, r, debWait, debMaxWait)
	r.deb = debFn
	r.cancelDebounce = cancel
	log.Info("Reconciler initialized", "project", r.proj.Name)
	return r, nil
}

func (r *Reconciler) Start(ctx context.Context) error {
	if err := r.buildInitialIndex(ctx); err != nil {
		return err
	}
	r.runCtx, r.runCancel = context.WithCancel(ctx)
	types := []resources.ResourceType{
		resources.ResourceWorkflow,
		resources.ResourceAgent,
		resources.ResourceTool,
		resources.ResourceSchema,
		resources.ResourceModel,
		resources.ResourceMCP,
		resources.ResourceKnowledgeBase,
	}
	for i := range types {
		typ := types[i]
		ch, err := r.store.Watch(r.runCtx, r.proj.Name, typ)
		if err != nil {
			return fmt.Errorf("watch %s failed: %w", string(typ), err)
		}
		r.wg.Add(1)
		go r.forward(r.runCtx, typ, ch)
	}
	r.wg.Add(1)
	go r.aggregate(r.runCtx)
	return nil
}

func (r *Reconciler) Stop() {
	if r.cancelDebounce != nil {
		r.cancelDebounce()
	}
	if r.runCancel != nil {
		r.runCancel()
	}
	r.wg.Wait()
}

func (r *Reconciler) forward(ctx context.Context, typ resources.ResourceType, ch <-chan resources.Event) {
	defer r.wg.Done()
	log := logger.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if r.eventsTotal != nil {
				r.eventsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("type", string(typ))))
			}
			select {
			case r.in <- evt:
			default:
				select {
				case <-r.in:
				default:
				}
				if r.eventsDropped != nil {
					r.eventsDropped.Add(ctx, 1, metric.WithAttributes(
						attribute.String("type", string(typ)),
						attribute.String("reason", "drop_oldest"),
					))
				}
				log.Warn("reconciler queue full; dropping oldest")
				sent := false
				select {
				case r.in <- evt:
					sent = true
				default:
				}
				if !sent {
					if r.eventsDropped != nil {
						r.eventsDropped.Add(ctx, 1, metric.WithAttributes(
							attribute.String("type", string(typ)),
							attribute.String("reason", "drop_incoming"),
						))
					}
					log.Warn("reconciler queue still full; dropping incoming event")
				}
			}
		}
	}
}

func (r *Reconciler) aggregate(ctx context.Context) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-r.in:
			r.pendMu.Lock()
			r.pending = append(r.pending, evt)
			r.pendMu.Unlock()
			r.deb()
		}
	}
}

func (r *Reconciler) onDebounceFire(ctx context.Context) {
	start := time.Now()
	r.pendMu.Lock()
	batch := make([]resources.Event, len(r.pending))
	copy(batch, r.pending)
	r.pending = r.pending[:0]
	r.pendMu.Unlock()
	if len(batch) == 0 {
		return
	}
	impacted, deletes := r.computeImpacted(ctx, batch)
	if len(impacted) == 0 && len(deletes) == 0 {
		if r.batchDur != nil {
			r.batchDur.Record(ctx, time.Since(start).Seconds())
		}
		return
	}
	if err := r.recompileAndSwap(ctx, impacted, deletes); err != nil {
		logger.FromContext(ctx).Error("recompile failed; state unchanged", "error", err)
	}
	if r.batchDur != nil {
		r.batchDur.Record(ctx, time.Since(start).Seconds())
	}
}

func (r *Reconciler) computeImpacted(
	ctx context.Context,
	batch []resources.Event,
) (map[string]struct{}, map[string]struct{}) {
	log := logger.FromContext(ctx)
	impacted := make(map[string]struct{})
	deletes := make(map[string]struct{})
	r.idxMu.RLock()
	for i := range batch {
		e := batch[i]
		if e.Key.Type == resources.ResourceWorkflow {
			if e.Type == resources.EventDelete {
				deletes[e.Key.ID] = struct{}{}
			}
			impacted[e.Key.ID] = struct{}{}
			continue
		}
		byID := r.idx[e.Key.Type]
		if byID != nil {
			if set := byID[e.Key.ID]; set != nil {
				for wf := range set {
					impacted[wf] = struct{}{}
				}
			}
		}
	}
	r.idxMu.RUnlock()
	if len(impacted) == 0 && len(deletes) == 0 {
		log.Debug("no impacted workflows from batch")
	}
	return impacted, deletes
}

func (r *Reconciler) recompileAndSwap(
	ctx context.Context,
	impacted map[string]struct{},
	deletes map[string]struct{},
) error {
	log := logger.FromContext(ctx)
	if len(impacted) == 0 && len(deletes) == 0 {
		return nil
	}
	if r.recompileTotal != nil {
		r.recompileTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "attempt")))
	}
	compiled, err := r.compileImpacted(ctx, impacted, deletes)
	if err != nil {
		if r.recompileTotal != nil {
			r.recompileTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		}
		return err
	}
	next := r.buildNextSet(compiled, deletes)
	slugs := workflow.SlugsFromList(next)
	if err := project.NewWebhookSlugsValidator(slugs).Validate(); err != nil {
		if r.recompileTotal != nil {
			r.recompileTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		}
		return fmt.Errorf("webhook slugs invalid after update: %w", err)
	}
	r.state.ReplaceWorkflows(next)
	if err := r.sched.OnConfigurationReload(ctx, next); err != nil {
		log.Warn("schedule reconciliation on reload failed", "error", err)
	}
	r.updateReverseIndex(compiled, deletes)
	if r.recompileTotal != nil {
		r.recompileTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
	}
	return nil
}

func (r *Reconciler) compileImpacted(
	ctx context.Context,
	impacted map[string]struct{},
	deletes map[string]struct{},
) (map[string]*workflow.Config, error) {
	compiled := make(map[string]*workflow.Config)
	if r.mode == modeRepo {
		return r.compileFromRepo(ctx, impacted, deletes)
	}
	for id := range impacted {
		if _, isDel := deletes[id]; isDel {
			continue
		}
		key := resources.ResourceKey{Project: r.proj.Name, Type: resources.ResourceWorkflow, ID: id}
		v, _, err := r.store.Get(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get workflow '%s' failed: %w", id, err)
		}
		wf, err := toWorkflow(v)
		if err != nil {
			return nil, fmt.Errorf("decode workflow '%s' failed: %w", id, err)
		}
		c, err := wf.Compile(ctx, r.proj, r.store)
		if err != nil {
			return nil, fmt.Errorf("compile '%s' failed: %w", id, err)
		}
		compiled[id] = c
	}
	return compiled, nil
}

// compileFromRepo compiles only the impacted workflows from YAML (repo mode)
func (r *Reconciler) compileFromRepo(
	ctx context.Context,
	impacted map[string]struct{},
	deletes map[string]struct{},
) (map[string]*workflow.Config, error) {
	log := logger.FromContext(ctx)
	compiled := make(map[string]*workflow.Config)
	all, err := workflow.WorkflowsFromProject(ctx, r.proj)
	if err != nil {
		return nil, fmt.Errorf("load workflows from repo failed: %w", err)
	}
	byID := make(map[string]*workflow.Config, len(all))
	for _, wf := range all {
		if wf != nil {
			byID[wf.ID] = wf
		}
	}
	if err := r.proj.IndexToResourceStore(ctx, r.store); err != nil {
		return nil, fmt.Errorf("index project resources failed: %w", err)
	}
	if r.registry != nil {
		if err := r.registry.SyncToResourceStore(ctx, r.proj.Name, r.store); err != nil {
			log.Warn("autoload registry sync failed during repo compile", "error", err)
		}
	}
	for id := range impacted {
		if _, isDel := deletes[id]; isDel {
			continue
		}
		wf, ok := byID[id]
		if !ok {
			continue
		}
		if err := wf.IndexToResourceStore(ctx, r.proj.Name, r.store); err != nil {
			return nil, fmt.Errorf("index workflow '%s' failed: %w", id, err)
		}
		c, err := wf.Compile(ctx, r.proj, r.store)
		if err != nil {
			return nil, fmt.Errorf("compile '%s' failed: %w", id, err)
		}
		compiled[id] = c
	}
	return compiled, nil
}

func toWorkflow(v any) (*workflow.Config, error) {
	switch tv := v.(type) {
	case *workflow.Config:
		return tv, nil
	case workflow.Config:
		tmp := tv
		return &tmp, nil
	case map[string]any:
		var tmp workflow.Config
		if err := tmp.FromMap(tv); err != nil {
			return nil, err
		}
		return &tmp, nil
	default:
		return nil, fmt.Errorf("unsupported workflow value %T", v)
	}
}

func (r *Reconciler) buildNextSet(
	compiled map[string]*workflow.Config,
	deletes map[string]struct{},
) []*workflow.Config {
	old := r.state.GetWorkflows()
	next := make([]*workflow.Config, 0, len(old)+len(compiled))
	seen := make(map[string]struct{})
	for i := range old {
		id := old[i].ID
		if _, del := deletes[id]; del {
			continue
		}
		if nw, ok := compiled[id]; ok {
			next = append(next, nw)
			seen[id] = struct{}{}
		} else {
			next = append(next, old[i])
			seen[id] = struct{}{}
		}
	}
	for id, nw := range compiled {
		if _, ok := seen[id]; !ok {
			next = append(next, nw)
		}
	}
	return next
}

func (r *Reconciler) buildInitialIndex(ctx context.Context) error {
	keys, err := r.store.List(ctx, r.proj.Name, resources.ResourceWorkflow)
	if err != nil {
		return fmt.Errorf("list workflows for index failed: %w", err)
	}
	for i := range keys {
		v, _, err := r.store.Get(ctx, keys[i])
		if err != nil {
			return err
		}
		wf, err := toWorkflow(v)
		if err != nil {
			return fmt.Errorf("decode workflow '%s' failed: %w", keys[i].ID, err)
		}
		r.recordDeps(wf)
	}
	return nil
}

func (r *Reconciler) updateReverseIndex(
	compiled map[string]*workflow.Config,
	deletes map[string]struct{},
) {
	for id := range deletes {
		r.dropWorkflowFromIndex(id)
	}
	for id := range compiled {
		r.dropWorkflowFromIndex(id)
	}
	for id, wf := range compiled {
		r.recordDepsRaw(id, wf)
	}
}

func (r *Reconciler) dropWorkflowFromIndex(id string) {
	r.idxMu.Lock()
	if olds, ok := r.wfdep[id]; ok {
		for _, d := range olds {
			m := r.idx[d.typ]
			if m != nil {
				if set := m[d.id]; set != nil {
					delete(set, id)
					if len(set) == 0 {
						delete(m, d.id)
					}
				}
			}
		}
		delete(r.wfdep, id)
	}
	r.idxMu.Unlock()
}

func (r *Reconciler) recordDeps(wf *workflow.Config) {
	if wf != nil {
		r.recordDepsRaw(wf.ID, wf)
	}
}

func (r *Reconciler) recordDepsRaw(id string, wf *workflow.Config) {
	deps := r.collectDeps(wf)
	r.idxMu.Lock()
	r.wfdep[id] = deps
	for _, d := range deps {
		byID := r.idx[d.typ]
		if byID == nil {
			byID = make(map[string]map[string]struct{})
			r.idx[d.typ] = byID
		}
		set := byID[d.id]
		if set == nil {
			set = make(map[string]struct{})
			byID[d.id] = set
		}
		set[id] = struct{}{}
	}
	r.idxMu.Unlock()
}

func (r *Reconciler) collectDeps(wf *workflow.Config) []depKey {
	var out []depKey
	if wf == nil {
		return out
	}
	appendKnowledgeDeps(&out, wf.Knowledge)
	for i := range wf.MCPs {
		appendMCPDep(&out, &wf.MCPs[i])
	}
	if wf.Opts.InputSchema != nil {
		if ok, id := wf.Opts.InputSchema.IsRef(); ok {
			out = append(out, depKey{typ: resources.ResourceSchema, id: id})
		}
	}
	for i := range wf.Triggers {
		t := &wf.Triggers[i]
		if t.Schema != nil {
			if ok, id := t.Schema.IsRef(); ok {
				out = append(out, depKey{typ: resources.ResourceSchema, id: id})
			}
		}
		if t.Webhook != nil {
			for ei := range t.Webhook.Events {
				e := &t.Webhook.Events[ei]
				if e.Schema != nil {
					if ok, id := e.Schema.IsRef(); ok {
						out = append(out, depKey{typ: resources.ResourceSchema, id: id})
					}
				}
			}
		}
	}
	for i := range wf.Tasks {
		r.walkTask(&wf.Tasks[i], &out)
	}
	for i := range wf.Tools {
		r.collectToolDeps(&wf.Tools[i], &out)
	}
	for i := range wf.Agents {
		r.collectAgentDeps(&wf.Agents[i], &out)
	}
	return out
}

func (r *Reconciler) walkTask(tcfg *task.Config, out *[]depKey) {
	if tcfg == nil {
		return
	}
	appendKnowledgeDeps(out, tcfg.Knowledge)
	if tcfg.InputSchema != nil {
		if ok, id := tcfg.InputSchema.IsRef(); ok {
			*out = append(*out, depKey{typ: resources.ResourceSchema, id: id})
		}
	}
	if tcfg.OutputSchema != nil {
		if ok, id := tcfg.OutputSchema.IsRef(); ok {
			*out = append(*out, depKey{typ: resources.ResourceSchema, id: id})
		}
	}
	if tcfg.Agent != nil {
		if r.isAgentSelector(tcfg.Agent) {
			*out = append(*out, depKey{typ: resources.ResourceAgent, id: tcfg.Agent.ID})
			if tcfg.Agent.Model.HasRef() {
				*out = append(*out, depKey{typ: resources.ResourceModel, id: tcfg.Agent.Model.Ref})
			}
		}
		appendAgentMCPDeps(tcfg.Agent, out)
		appendKnowledgeDeps(out, tcfg.Agent.Knowledge)
	}
	if tcfg.Tool != nil {
		if r.isToolSelector(tcfg.Tool) {
			*out = append(*out, depKey{typ: resources.ResourceTool, id: tcfg.Tool.ID})
		}
	}
	if tcfg.Type == task.TaskTypeParallel {
		for i := range tcfg.Tasks {
			r.walkTask(&tcfg.Tasks[i], out)
		}
	}
	if tcfg.Task != nil {
		r.walkTask(tcfg.Task, out)
	}
}

func (r *Reconciler) collectToolDeps(tl *tool.Config, out *[]depKey) {
	if tl == nil {
		return
	}
	if tl.InputSchema != nil {
		if ok, id := tl.InputSchema.IsRef(); ok {
			*out = append(*out, depKey{typ: resources.ResourceSchema, id: id})
		}
	}
	if tl.OutputSchema != nil {
		if ok, id := tl.OutputSchema.IsRef(); ok {
			*out = append(*out, depKey{typ: resources.ResourceSchema, id: id})
		}
	}
}

func (r *Reconciler) collectAgentDeps(a *agent.Config, out *[]depKey) {
	if a == nil {
		return
	}
	appendKnowledgeDeps(out, a.Knowledge)
	if a.Model.HasRef() {
		*out = append(*out, depKey{typ: resources.ResourceModel, id: a.Model.Ref})
	}
	appendAgentMCPDeps(a, out)
	for i := range a.Actions {
		ac := a.Actions[i]
		if ac == nil {
			continue
		}
		if ac.InputSchema != nil {
			if ok, id := ac.InputSchema.IsRef(); ok {
				*out = append(*out, depKey{typ: resources.ResourceSchema, id: id})
			}
		}
		if ac.OutputSchema != nil {
			if ok, id := ac.OutputSchema.IsRef(); ok {
				*out = append(*out, depKey{typ: resources.ResourceSchema, id: id})
			}
		}
	}
}

func appendAgentMCPDeps(a *agent.Config, out *[]depKey) {
	if a == nil {
		return
	}
	for i := range a.MCPs {
		appendMCPDep(out, &a.MCPs[i])
	}
}

func appendMCPDep(out *[]depKey, mc *mcp.Config) {
	if mc == nil || mc.ID == "" {
		return
	}
	*out = append(*out, depKey{typ: resources.ResourceMCP, id: mc.ID})
}

func appendKnowledgeDeps(out *[]depKey, bindings []core.KnowledgeBinding) {
	for i := range bindings {
		id := strings.TrimSpace(bindings[i].ID)
		if id == "" {
			continue
		}
		*out = append(*out, depKey{typ: resources.ResourceKnowledgeBase, id: id})
	}
}

func (r *Reconciler) isAgentSelector(a *agent.Config) bool {
	if a == nil {
		return false
	}
	hasID := a.ID != ""
	noModel := a.Model.Config.Provider == "" && a.Model.Config.Model == "" && !a.Model.HasRef()
	noInstr := a.Instructions == ""
	return hasID && noModel && noInstr && len(a.Tools) == 0 && len(a.MCPs) == 0
}

func (r *Reconciler) isToolSelector(tl *tool.Config) bool {
	if tl == nil {
		return false
	}
	return tl.ID != "" && tl.Description == "" && tl.Timeout == "" && tl.InputSchema == nil && tl.OutputSchema == nil
}

func StartIfBuilderMode(ctx context.Context, state *appstate.State) (*Reconciler, error) {
	cfg := pkgcfg.FromContext(ctx)
	if cfg == nil || cfg.Server.SourceOfTruth != modeBuilder {
		return nil, nil
	}
	r, err := NewReconciler(ctx, state)
	if err != nil {
		return nil, err
	}
	if err := r.Start(ctx); err != nil {
		return nil, err
	}
	return r, nil
}
