package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/contentconv"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type uiModel struct {
	jobs               []uiJob
	total              int
	completed          int
	failed             int
	frame              int
	events             <-chan uiMsg
	onQuit             func(uiQuitRequest)
	transcriptViewport viewport.Model
	sidebarViewport    viewport.Model
	progressBar        progress.Model
	selectedJob        int
	width              int
	height             int
	sidebarWidth       int
	timelineWidth      int
	contentHeight      int
	layoutMode         uiLayoutMode
	currentView        uiViewState
	focusedPane        uiPane
	shutdown           shutdownState
	failures           []failInfo
	aggregateUsage     *model.Usage
	cfg                *config
}

type uiController struct {
	ch              chan uiMsg
	model           *uiModel
	prog            *tea.Program
	done            chan error
	quitHandler     func(uiQuitRequest)
	quitHandlerMu   sync.RWMutex
	stopEvents      func()
	adapterDone     <-chan struct{}
	closeEventsOnce sync.Once
	shutdownOnce    sync.Once
}

func newUIModel(total int) *uiModel {
	transcriptVp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(24))
	transcriptVp.Style = lipgloss.NewStyle().
		Foreground(colorFgBright)
	sidebarVp := viewport.New(viewport.WithWidth(30), viewport.WithHeight(24))
	sidebarVp.Style = lipgloss.NewStyle().
		Foreground(colorFgBright)
	pb := progress.New(
		progress.WithColors(
			lipgloss.Color(progressGradientStart),
			lipgloss.Color(progressGradientEnd),
		),
		progress.WithoutPercentage(),
	)
	pb.Empty = progress.DefaultFullCharFullBlock
	pb.EmptyColor = colorBorder
	defaultWidth := 120
	defaultHeight := 40
	initialSidebarWidth := int(float64(defaultWidth) * sidebarWidthRatio)
	if initialSidebarWidth < sidebarMinWidth {
		initialSidebarWidth = sidebarMinWidth
	}
	if initialSidebarWidth > sidebarMaxWidth {
		initialSidebarWidth = sidebarMaxWidth
	}
	initialMainWidth := defaultWidth - initialSidebarWidth
	if initialMainWidth < mainMinWidth {
		initialMainWidth = mainMinWidth
	}
	initialContentHeight := defaultHeight - chromeHeight
	if initialContentHeight < minContentHeight {
		initialContentHeight = minContentHeight
	}
	mdl := &uiModel{
		total:              total,
		transcriptViewport: transcriptVp,
		sidebarViewport:    sidebarVp,
		progressBar:        pb,
		selectedJob:        0,
		width:              defaultWidth,
		height:             defaultHeight,
		sidebarWidth:       initialSidebarWidth,
		timelineWidth:      initialMainWidth,
		contentHeight:      initialContentHeight,
		layoutMode:         uiLayoutSplit,
		currentView:        uiViewJobs,
		focusedPane:        uiPaneJobs,
		failures:           []failInfo{},
		aggregateUsage:     &model.Usage{},
	}
	layout := mdl.computeLayout(defaultWidth, defaultHeight)
	mdl.layoutMode = layout.mode
	mdl.sidebarWidth = layout.sidebarWidth
	mdl.timelineWidth = layout.timelineWidth
	mdl.contentHeight = layout.contentHeight
	mdl.configureViewports(layout)
	return mdl
}

func (m *uiModel) setEventSource(ch <-chan uiMsg) {
	m.events = ch
}

func (m *uiModel) Init() tea.Cmd {
	return tea.Batch(m.waitEvent(), m.tick())
}

func (m *uiModel) waitEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	return func() tea.Msg {
		if ev, ok := <-m.events; ok {
			return ev
		}
		return drainMsg{}
	}
}

func (m *uiModel) tick() tea.Cmd {
	return tea.Tick(uiTickInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func newUIController(ctx context.Context, total int, cfg *config, bus *events.Bus[events.Event]) *uiController {
	uiCh := make(chan uiMsg, max(total*4, 4))
	mdl := newUIModel(total)
	mdl.cfg = cfg
	mdl.setEventSource(uiCh)
	stopEvents, adapterDone := startUIEventAdapter(ctx, bus, uiCh)

	ctrl := &uiController{
		ch:          uiCh,
		model:       mdl,
		done:        make(chan error, 1),
		stopEvents:  stopEvents,
		adapterDone: adapterDone,
	}
	mdl.onQuit = ctrl.requestQuit
	ctrl.prog = tea.NewProgram(mdl, tea.WithoutSignalHandler())
	go func() {
		_, runErr := ctrl.prog.Run()
		if runErr != nil {
			ctrl.done <- runErr
		}
		close(ctrl.done)
	}()
	return ctrl
}

func (c *uiController) Enqueue(msg any) {
	if c == nil {
		return
	}
	typed, ok := msg.(uiMsg)
	if !ok {
		return
	}
	c.ch <- typed
}

func (c *uiController) enqueue(msg uiMsg) {
	c.Enqueue(msg)
}

func (c *uiController) SetQuitHandler(fn func(uiQuitRequest)) {
	c.quitHandlerMu.Lock()
	defer c.quitHandlerMu.Unlock()
	c.quitHandler = fn
}

func (c *uiController) setQuitHandler(fn func(uiQuitRequest)) {
	c.SetQuitHandler(fn)
}

func (c *uiController) requestQuit(req uiQuitRequest) {
	c.quitHandlerMu.RLock()
	fn := c.quitHandler
	c.quitHandlerMu.RUnlock()
	if fn != nil {
		fn(req)
	}
}

func (c *uiController) CloseEvents() {
	c.closeEventsOnce.Do(func() {
		if c.stopEvents != nil {
			c.stopEvents()
		}
	})
}

func (c *uiController) closeEvents() {
	c.CloseEvents()
}

func (c *uiController) Shutdown() {
	c.shutdownOnce.Do(func() {
		c.CloseEvents()
		if c.prog != nil {
			c.prog.Quit()
		}
	})
}

func (c *uiController) shutdown() {
	c.Shutdown()
}

func (c *uiController) Wait() error {
	err, ok := <-c.done
	if !ok {
		if c.adapterDone != nil {
			c.CloseEvents()
			<-c.adapterDone
		}
		return nil
	}
	if c.adapterDone != nil {
		c.CloseEvents()
		<-c.adapterDone
	}
	return err
}

func (c *uiController) wait() error {
	return c.Wait()
}

func Setup(ctx context.Context, jobs []job, cfg *config, bus *events.Bus[events.Event], enabled bool) Session {
	if !enabled {
		return nil
	}
	ctrl := newUIController(ctx, len(jobs), cfg, bus)
	for idx := range jobs {
		jb := &jobs[idx]
		totalIssues := 0
		for _, items := range jb.Groups {
			totalIssues += len(items)
		}
		codeFileLabel := jb.CodeFileLabel()
		if len(jb.CodeFiles) > 3 {
			codeFileLabel = fmt.Sprintf("%s and %d more", strings.Join(jb.CodeFiles[:3], ", "), len(jb.CodeFiles)-3)
		}
		ctrl.Enqueue(jobQueuedMsg{
			Index:     idx,
			CodeFile:  codeFileLabel,
			CodeFiles: jb.CodeFiles,
			Issues:    totalIssues,
			TaskTitle: jb.TaskTitle,
			TaskType:  jb.TaskType,
			SafeName:  jb.SafeName,
			OutLog:    jb.OutLog,
			ErrLog:    jb.ErrLog,
			OutBuffer: jb.OutBuffer,
			ErrBuffer: jb.ErrBuffer,
		})
	}
	return ctrl
}

func setupUI(ctx context.Context, jobs []job, cfg *config, bus *events.Bus[events.Event], enabled bool) uiSession {
	return Setup(ctx, jobs, cfg, bus, enabled)
}

type uiEventTranslator struct {
	sessionViews map[int]*sessionViewModel
}

func newUIEventTranslator() *uiEventTranslator {
	return &uiEventTranslator{
		sessionViews: make(map[int]*sessionViewModel),
	}
}

func startUIEventAdapter(
	parent context.Context,
	bus *events.Bus[events.Event],
	sink chan uiMsg,
) (func(), <-chan struct{}) {
	done := make(chan struct{})
	var closeSinkOnce sync.Once
	closeSink := func() {
		closeSinkOnce.Do(func() {
			close(sink)
			close(done)
		})
	}
	if bus == nil {
		return closeSink, done
	}

	_, updates, unsubscribe := bus.Subscribe()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		defer closeSink()
		defer unsubscribe()

		translator := newUIEventTranslator()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-updates:
				if !ok {
					return
				}
				for _, msg := range translator.translateMessages(ev) {
					select {
					case <-ctx.Done():
						return
					case sink <- msg:
					}
				}
			}
		}
	}()
	return cancel, done
}

func (t *uiEventTranslator) translateMessages(ev events.Event) []uiMsg {
	msg, ok := t.translateEvent(ev)
	if !ok {
		return nil
	}

	msgs := []uiMsg{msg}
	if ev.Kind == events.EventKindJobFailed {
		payload, ok := decodeUIEventPayload[kinds.JobFailedPayload](ev)
		if !ok {
			return msgs
		}
		msgs = append(msgs, jobFinishedMsg{
			Index:    payload.Index,
			Success:  false,
			ExitCode: payload.ExitCode,
		})
	}
	return msgs
}

func (t *uiEventTranslator) translateEvent(ev events.Event) (uiMsg, bool) {
	if msg, ok := t.translateJobEvent(ev); ok {
		return msg, true
	}
	if msg, ok := t.translateSessionEvent(ev); ok {
		return msg, true
	}
	if msg, ok := t.translateUsageEvent(ev); ok {
		return msg, true
	}
	return translateShutdownEvent(ev)
}

func (t *uiEventTranslator) translateJobEvent(ev events.Event) (uiMsg, bool) {
	switch ev.Kind {
	case events.EventKindJobStarted:
		payload, ok := decodeUIEventPayload[kinds.JobStartedPayload](ev)
		if !ok {
			return nil, false
		}
		return jobStartedMsg{
			Index:       payload.Index,
			Attempt:     payload.Attempt,
			MaxAttempts: payload.MaxAttempts,
		}, true
	case events.EventKindJobCompleted:
		payload, ok := decodeUIEventPayload[kinds.JobCompletedPayload](ev)
		if !ok {
			return nil, false
		}
		return jobFinishedMsg{
			Index:    payload.Index,
			Success:  true,
			ExitCode: payload.ExitCode,
		}, true
	case events.EventKindJobRetryScheduled:
		payload, ok := decodeUIEventPayload[kinds.JobRetryScheduledPayload](ev)
		if !ok {
			return nil, false
		}
		return jobRetryMsg{
			Index:       payload.Index,
			Attempt:     payload.Attempt,
			MaxAttempts: payload.MaxAttempts,
			Reason:      payload.Reason,
		}, true
	case events.EventKindJobFailed:
		payload, ok := decodeUIEventPayload[kinds.JobFailedPayload](ev)
		if !ok {
			return nil, false
		}
		return jobFailureMsg{
			Failure: jobFailureFromPayload(payload),
		}, true
	case events.EventKindJobCancelled:
		payload, ok := decodeUIEventPayload[kinds.JobCancelledPayload](ev)
		if !ok {
			return nil, false
		}
		return jobFinishedMsg{
			Index:    payload.Index,
			Success:  false,
			ExitCode: exitCodeCanceled,
		}, true
	default:
		return nil, false
	}
}

func (t *uiEventTranslator) translateSessionEvent(ev events.Event) (uiMsg, bool) {
	switch ev.Kind {
	case events.EventKindSessionUpdate:
		return t.translateSessionUpdate(ev)
	default:
		return nil, false
	}
}

func (t *uiEventTranslator) translateUsageEvent(ev events.Event) (uiMsg, bool) {
	switch ev.Kind {
	case events.EventKindUsageUpdated:
		payload, ok := decodeUIEventPayload[kinds.UsageUpdatedPayload](ev)
		if !ok {
			return nil, false
		}
		return usageUpdateMsg{
			Index: payload.Index,
			Usage: contentconv.InternalUsage(payload.Usage),
		}, true
	default:
		return nil, false
	}
}

func translateShutdownEvent(ev events.Event) (uiMsg, bool) {
	switch ev.Kind {
	case events.EventKindShutdownRequested:
		payload, ok := decodeUIEventPayload[kinds.ShutdownRequestedPayload](ev)
		if !ok {
			return nil, false
		}
		return shutdownStatusMsg{
			State: shutdownStateFromPayload(
				shutdownPhaseDraining,
				payload.Source,
				payload.RequestedAt,
				payload.DeadlineAt,
			),
		}, true
	case events.EventKindShutdownDraining:
		payload, ok := decodeUIEventPayload[kinds.ShutdownDrainingPayload](ev)
		if !ok {
			return nil, false
		}
		return shutdownStatusMsg{
			State: shutdownStateFromPayload(
				shutdownPhaseDraining,
				payload.Source,
				payload.RequestedAt,
				payload.DeadlineAt,
			),
		}, true
	case events.EventKindShutdownTerminated:
		payload, ok := decodeUIEventPayload[kinds.ShutdownTerminatedPayload](ev)
		if !ok {
			return nil, false
		}
		phase := shutdownPhaseDraining
		if payload.Forced {
			phase = shutdownPhaseForcing
		}
		return shutdownStatusMsg{
			State: shutdownStateFromPayload(phase, payload.Source, payload.RequestedAt, payload.DeadlineAt),
		}, true
	default:
		return nil, false
	}
}

func (t *uiEventTranslator) translateSessionUpdate(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.SessionUpdatePayload](ev)
	if !ok {
		return nil, false
	}
	update, err := contentconv.InternalSessionUpdate(payload.Update)
	if err != nil {
		return nil, false
	}
	viewModel := t.sessionView(payload.Index)
	snapshot, changed := viewModel.Apply(update)
	if !changed {
		snapshot = viewModel.Snapshot()
	}
	return jobUpdateMsg{
		Index:    payload.Index,
		Snapshot: snapshot,
	}, true
}

func (t *uiEventTranslator) sessionView(index int) *sessionViewModel {
	if t.sessionViews == nil {
		t.sessionViews = make(map[int]*sessionViewModel)
	}
	viewModel := t.sessionViews[index]
	if viewModel == nil {
		viewModel = newSessionViewModel()
		t.sessionViews[index] = viewModel
	}
	return viewModel
}

func decodeUIEventPayload[T any](ev events.Event) (T, bool) {
	var payload T
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return payload, false
	}
	return payload, true
}

func jobFailureFromPayload(payload kinds.JobFailedPayload) failInfo {
	return failInfo{
		CodeFile: payload.CodeFile,
		ExitCode: payload.ExitCode,
		OutLog:   payload.OutLog,
		ErrLog:   payload.ErrLog,
		Err:      eventError(payload.Error),
	}
}

func shutdownStateFromPayload(
	phase shutdownPhase,
	source string,
	requestedAt time.Time,
	deadlineAt time.Time,
) shutdownState {
	return shutdownState{
		Phase:       phase,
		Source:      shutdownSource(strings.TrimSpace(source)),
		RequestedAt: requestedAt,
		DeadlineAt:  deadlineAt,
	}
}

func eventError(msg string) error {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil
	}
	return errors.New(msg)
}
