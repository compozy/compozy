package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/contentconv"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type uiModel struct {
	ctx                          context.Context
	jobs                         []uiJob
	total                        int
	completed                    int
	failed                       int
	runStatus                    string
	frame                        int
	now                          time.Time
	onQuit                       func(uiQuitRequest)
	onJobControl                 func(context.Context, uiJobControlRequest) (model.JobControlResponse, error)
	transcriptViewport           viewport.Model
	sidebarViewport              viewport.Model
	composer                     textarea.Model
	composerBusy                 bool
	composerError                string
	progressBar                  progress.Model
	selectedJob                  int
	width                        int
	height                       int
	sidebarWidth                 int
	timelineWidth                int
	contentHeight                int
	layoutMode                   uiLayoutMode
	currentView                  uiViewState
	focusedPane                  uiPane
	headerHidden                 bool
	quitDialog                   quitDialogState
	shutdown                     shutdownState
	failures                     []failInfo
	aggregateUsage               *model.Usage
	cfg                          *config
	sidebarDirty                 bool
	sidebarContent               string
	spinnerRunning               bool
	timelineMounted              timelineMountState
	setTranscriptViewportContent func(vp *viewport.Model, content string)
	setSidebarViewportContent    func(vp *viewport.Model, content string)
}

type uiController struct {
	model               *uiModel
	prog                *tea.Program
	done                chan error
	quitHandler         func(uiQuitRequest)
	quitHandlerMu       sync.RWMutex
	jobControlHandler   func(context.Context, uiJobControlRequest) (model.JobControlResponse, error)
	jobControlHandlerMu sync.RWMutex
	stopEvents          func()
	adapterDone         <-chan struct{}
	closeEventsOnce     sync.Once
	shutdownOnce        sync.Once
	dispatchWake        chan struct{}
	dispatchDone        chan struct{}
	dispatchCtx         context.Context
	cancelDispatch      context.CancelFunc
	pendingMu           sync.Mutex
	pendingInputs       []any
	pendingUrgent       bool
	translator          *uiEventTranslator
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
	composer := textarea.New()
	composer.Prompt = composerPromptGlyph
	composer.Placeholder = composerPausedTaskPrompt
	composer.ShowLineNumbers = false
	composer.EndOfBufferCharacter = ' '
	composer.CharLimit = model.MaxJobControlMessageBytes
	composer.SetVirtualCursor(false)
	configureComposerStyles(&composer)
	composer.SetWidth(80)
	composer.SetHeight(1)
	composer.Blur()
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
	initialContentHeight := defaultHeight - chromeHeightStandalone
	if initialContentHeight < minContentHeight {
		initialContentHeight = minContentHeight
	}
	mdl := &uiModel{
		ctx:                          context.Background(),
		total:                        total,
		transcriptViewport:           transcriptVp,
		sidebarViewport:              sidebarVp,
		composer:                     composer,
		progressBar:                  pb,
		selectedJob:                  0,
		width:                        defaultWidth,
		height:                       defaultHeight,
		sidebarWidth:                 initialSidebarWidth,
		timelineWidth:                initialMainWidth,
		contentHeight:                initialContentHeight,
		layoutMode:                   uiLayoutSplit,
		currentView:                  uiViewJobs,
		focusedPane:                  uiPaneJobs,
		quitDialog:                   newQuitDialogState(),
		failures:                     []failInfo{},
		aggregateUsage:               &model.Usage{},
		sidebarDirty:                 true,
		now:                          time.Now(),
		timelineMounted:              invalidTimelineMountState(),
		setTranscriptViewportContent: setTranscriptViewportContent,
		setSidebarViewportContent:    setSidebarViewportContent,
	}
	layout := mdl.computeLayout(defaultWidth, defaultHeight)
	mdl.layoutMode = layout.mode
	mdl.sidebarWidth = layout.sidebarWidth
	mdl.timelineWidth = layout.timelineWidth
	mdl.contentHeight = layout.contentHeight
	mdl.configureViewports(layout)
	return mdl
}

func (m *uiModel) configureComposerAppearance() {
	if m == nil {
		return
	}
	m.composer.Prompt = composerPromptGlyph
}

func configureComposerStyles(composer *textarea.Model) {
	if composer == nil {
		return
	}
	focused := lipgloss.NewStyle().Foreground(colorFgBright)
	muted := lipgloss.NewStyle().Foreground(colorMuted)
	styles := composer.Styles()
	styles.Focused.Base = lipgloss.NewStyle()
	styles.Focused.Text = focused
	styles.Focused.CursorLine = focused
	styles.Focused.LineNumber = muted
	styles.Focused.CursorLineNumber = muted
	styles.Focused.EndOfBuffer = muted
	styles.Focused.Placeholder = muted
	styles.Focused.Prompt = muted
	styles.Blurred.Base = lipgloss.NewStyle()
	styles.Blurred.Text = muted
	styles.Blurred.CursorLine = muted
	styles.Blurred.LineNumber = muted
	styles.Blurred.CursorLineNumber = muted
	styles.Blurred.EndOfBuffer = muted
	styles.Blurred.Placeholder = muted
	styles.Blurred.Prompt = muted
	styles.Cursor.Color = colorFgBright
	composer.SetStyles(styles)
}

func (m *uiModel) applyTranscriptViewportContent(content string) {
	if m == nil {
		return
	}
	setter := m.setTranscriptViewportContent
	if setter == nil {
		setter = setTranscriptViewportContent
	}
	setter(&m.transcriptViewport, content)
}

func (m *uiModel) applySidebarViewportContent(content string) {
	if m == nil {
		return
	}
	setter := m.setSidebarViewportContent
	if setter == nil {
		setter = setSidebarViewportContent
	}
	setter(&m.sidebarViewport, content)
}

func (m *uiModel) Init() tea.Cmd {
	return m.clockTick()
}

func (m *uiModel) clockTick() tea.Cmd {
	return tea.Every(uiClockTickInterval, func(at time.Time) tea.Msg {
		return clockTickMsg{at: at}
	})
}

func (m *uiModel) spinnerTick() tea.Cmd {
	return tea.Tick(uiSpinnerTickInterval, func(at time.Time) tea.Msg {
		return spinnerTickMsg{at: at}
	})
}

func newUIController(ctx context.Context, total int, cfg *config, bus *events.Bus[events.Event]) *uiController {
	if ctx == nil {
		ctx = context.Background()
	}
	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	mdl := newUIModel(total)
	mdl.ctx = dispatchCtx
	mdl.cfg = cfg
	ctrl := &uiController{
		model:          mdl,
		done:           make(chan error, 1),
		dispatchWake:   make(chan struct{}, 1),
		dispatchDone:   make(chan struct{}),
		dispatchCtx:    dispatchCtx,
		cancelDispatch: cancelDispatch,
		translator:     newUIEventTranslator(),
	}
	mdl.onQuit = ctrl.requestQuit
	mdl.onJobControl = ctrl.requestJobControl
	ctrl.prog = tea.NewProgram(mdl, tea.WithoutSignalHandler())
	stopEvents, adapterDone := startUIEventAdapter(ctx, bus, ctrl.EnqueueEvent)
	ctrl.stopEvents = stopEvents
	ctrl.adapterDone = adapterDone
	go ctrl.dispatchLoop()
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
	c.pendingMu.Lock()
	if c.dispatchCtx != nil && c.dispatchCtx.Err() != nil {
		c.pendingMu.Unlock()
		return
	}
	c.pendingInputs = append(c.pendingInputs, msg)
	if inputRequiresImmediateDispatch(msg) {
		c.pendingUrgent = true
	}
	c.pendingMu.Unlock()
	c.signalDispatch()
}

func (c *uiController) enqueue(msg uiMsg) {
	c.Enqueue(msg)
}

func (c *uiController) EnqueueEvent(ev events.Event) {
	c.Enqueue(ev)
}

func (c *uiController) signalDispatch() {
	if c == nil {
		return
	}
	select {
	case c.dispatchWake <- struct{}{}:
	default:
	}
}

func (c *uiController) SetQuitHandler(fn func(uiQuitRequest)) {
	c.quitHandlerMu.Lock()
	defer c.quitHandlerMu.Unlock()
	c.quitHandler = fn
}

func (c *uiController) SetJobControlHandler(
	fn func(context.Context, uiJobControlRequest) (model.JobControlResponse, error),
) {
	c.jobControlHandlerMu.Lock()
	defer c.jobControlHandlerMu.Unlock()
	c.jobControlHandler = fn
}

func (c *uiController) requestJobControl(
	ctx context.Context,
	req uiJobControlRequest,
) (model.JobControlResponse, error) {
	c.jobControlHandlerMu.RLock()
	fn := c.jobControlHandler
	c.jobControlHandlerMu.RUnlock()
	if fn == nil {
		return model.JobControlResponse{}, model.ErrJobControlNotFound
	}
	return fn(ctx, req)
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
		if c.cancelDispatch != nil {
			c.cancelDispatch()
		}
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
	if c.cancelDispatch != nil {
		c.cancelDispatch()
	}
	if c.dispatchDone != nil {
		<-c.dispatchDone
	}
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
	if cfg != nil && cfg.JobControls != nil {
		ctrl.SetJobControlHandler(func(ctx context.Context, req uiJobControlRequest) (model.JobControlResponse, error) {
			runID := firstNonEmpty(strings.TrimSpace(req.RunID), strings.TrimSpace(cfg.RunID))
			switch req.Action {
			case uiJobControlPause:
				return cfg.JobControls.Pause(ctx, runID, req.JobID)
			case uiJobControlMessage:
				return cfg.JobControls.SendMessage(ctx, runID, req.JobID, req.Message)
			default:
				return model.JobControlResponse{}, model.ErrJobControlConflict
			}
		})
	}
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
			Index:           idx,
			CodeFile:        codeFileLabel,
			CodeFiles:       jb.CodeFiles,
			Issues:          totalIssues,
			TaskNumber:      jb.TaskNumber,
			TaskTitle:       jb.TaskTitle,
			TaskType:        jb.TaskType,
			SafeName:        jb.SafeName,
			IDE:             jb.IDE,
			Model:           jb.Model,
			ReasoningEffort: jb.ReasoningEffort,
			OutLog:          jb.OutLog,
			ErrLog:          jb.ErrLog,
			OutBuffer:       jb.OutBuffer,
			ErrBuffer:       jb.ErrBuffer,
		})
	}
	return ctrl
}

func setupUI(ctx context.Context, jobs []job, cfg *config, bus *events.Bus[events.Event], enabled bool) uiSession {
	return Setup(ctx, jobs, cfg, bus, enabled)
}

func (c *uiController) dispatchLoop() {
	ticker := time.NewTicker(uiDispatchInterval)
	defer ticker.Stop()
	defer close(c.dispatchDone)

	for {
		select {
		case <-c.dispatchCtx.Done():
			return
		case <-c.dispatchWake:
			if c.hasUrgentDispatch() {
				c.flushDispatch()
			}
		case <-ticker.C:
			c.flushDispatch()
		}
	}
}

func (c *uiController) hasUrgentDispatch() bool {
	if c == nil {
		return false
	}
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	return c.pendingUrgent && len(c.pendingInputs) > 0
}

func (c *uiController) flushDispatch() {
	if c == nil || c.prog == nil {
		return
	}
	inputs := c.takePendingInputs()
	if len(inputs) == 0 {
		return
	}

	msg := c.prepareDispatchBatch(inputs)
	if len(msg.msgs) == 0 {
		return
	}
	c.prog.Send(msg)
}

func (c *uiController) takePendingInputs() []any {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if len(c.pendingInputs) == 0 {
		return nil
	}
	inputs := c.pendingInputs
	c.pendingInputs = nil
	c.pendingUrgent = false
	return inputs
}

func (c *uiController) prepareDispatchBatch(inputs []any) dispatchBatchMsg {
	translator := c.translator
	if translator == nil {
		translator = newUIEventTranslator()
		c.translator = translator
	}
	accumulator := newUIDispatchAccumulator()
	for _, input := range inputs {
		switch value := input.(type) {
		case events.Event:
			for _, msg := range translator.translateMessages(value) {
				accumulator.add(msg)
			}
		case *events.Event:
			if value == nil {
				continue
			}
			for _, msg := range translator.translateMessages(*value) {
				accumulator.add(msg)
			}
		case uiMsg:
			if update, ok := value.(jobUpdateMsg); ok && update.HydrateTranslator {
				translator.hydrateSessionView(update.Index, update.Snapshot)
			}
			accumulator.add(value)
		}
	}
	return accumulator.batch()
}

type uiDispatchAccumulator struct {
	jobUpdates []jobUpdateMsg
	usages     map[int]model.Usage
	msgs       []uiMsg
}

func newUIDispatchAccumulator() *uiDispatchAccumulator {
	return &uiDispatchAccumulator{
		usages: make(map[int]model.Usage),
	}
}

func (a *uiDispatchAccumulator) add(msg uiMsg) {
	switch value := msg.(type) {
	case jobUpdateMsg:
		a.addJobUpdate(value)
	case usageUpdateMsg:
		current := a.usages[value.Index]
		current.Add(value.Usage)
		a.usages[value.Index] = current
	default:
		a.flushCoalesced()
		a.msgs = append(a.msgs, msg)
	}
}

func (a *uiDispatchAccumulator) addJobUpdate(msg jobUpdateMsg) {
	if len(a.jobUpdates) == 0 {
		a.jobUpdates = append(a.jobUpdates, msg)
		return
	}
	last := &a.jobUpdates[len(a.jobUpdates)-1]
	if canCoalesceJobUpdate(*last, msg) {
		*last = msg
		return
	}
	a.jobUpdates = append(a.jobUpdates, msg)
}

func (a *uiDispatchAccumulator) flushCoalesced() {
	if len(a.jobUpdates) > 0 {
		a.msgs = append(a.msgs, a.jobUpdatesAsUI()...)
		a.jobUpdates = nil
	}
	if len(a.usages) > 0 {
		indexes := make([]int, 0, len(a.usages))
		for index := range a.usages {
			indexes = append(indexes, index)
		}
		sort.Ints(indexes)
		for _, index := range indexes {
			a.msgs = append(a.msgs, usageUpdateMsg{
				Index: index,
				Usage: a.usages[index],
			})
		}
		clear(a.usages)
	}
}

func (a *uiDispatchAccumulator) jobUpdatesAsUI() []uiMsg {
	if len(a.jobUpdates) == 0 {
		return nil
	}
	msgs := make([]uiMsg, 0, len(a.jobUpdates))
	for idx := range a.jobUpdates {
		msgs = append(msgs, a.jobUpdates[idx])
	}
	return msgs
}

func (a *uiDispatchAccumulator) batch() dispatchBatchMsg {
	a.flushCoalesced()
	if len(a.msgs) == 0 {
		return dispatchBatchMsg{}
	}
	return dispatchBatchMsg{msgs: append([]uiMsg(nil), a.msgs...)}
}

func canCoalesceJobUpdate(previous jobUpdateMsg, next jobUpdateMsg) bool {
	if !sameJobUpdateSurface(previous, next) {
		return false
	}
	previousTail, ok := snapshotTailEntry(previous.Snapshot)
	if !ok {
		return false
	}
	nextTail, ok := snapshotTailEntry(next.Snapshot)
	if !ok {
		return false
	}
	if previousTail.ID == "" || previousTail.ID != nextTail.ID || previousTail.Kind != nextTail.Kind {
		return false
	}

	switch next.UpdateKind {
	case model.UpdateKindAgentMessageChunk:
		return tailMatchesTranscriptKind(nextTail, transcriptEntryAssistantMessage)
	case model.UpdateKindAgentThoughtChunk:
		return tailMatchesTranscriptKind(nextTail, transcriptEntryAssistantThinking)
	case model.UpdateKindToolCallUpdated:
		return sameToolCallProgressUpdate(previous, next) &&
			tailMatchesTranscriptKind(nextTail, transcriptEntryToolCall)
	default:
		return false
	}
}

func sameJobUpdateSurface(previous jobUpdateMsg, next jobUpdateMsg) bool {
	if previous.Index != next.Index || previous.HydrateTranslator || next.HydrateTranslator {
		return false
	}
	if previous.UpdateKind != next.UpdateKind || next.UpdateKind == model.UpdateKindUnknown {
		return false
	}
	return previous.SessionStatus == next.SessionStatus
}

func sameToolCallProgressUpdate(previous jobUpdateMsg, next jobUpdateMsg) bool {
	return previous.ToolCallID != "" &&
		previous.ToolCallID == next.ToolCallID &&
		previous.ToolCallState == next.ToolCallState
}

func tailMatchesTranscriptKind(entry TranscriptEntry, kind transcriptEntryKind) bool {
	return entry.Kind == kind
}

func snapshotTailEntry(snapshot SessionViewSnapshot) (TranscriptEntry, bool) {
	if len(snapshot.Entries) == 0 {
		return TranscriptEntry{}, false
	}
	return snapshot.Entries[len(snapshot.Entries)-1], true
}

func inputRequiresImmediateDispatch(msg any) bool {
	switch value := msg.(type) {
	case jobQueuedMsg,
		jobStartedMsg,
		jobRetryMsg,
		jobFinishedMsg,
		jobUpdateMsg,
		runStatusMsg,
		shutdownStatusMsg,
		jobFailureMsg:
		return true
	case jobPausingMsg, jobPausedMsg, jobResumedMsg, jobControlResultMsg:
		return true
	case events.Event:
		switch value.Kind {
		case events.EventKindJobQueued,
			events.EventKindJobStarted,
			events.EventKindJobPausing,
			events.EventKindJobPaused,
			events.EventKindJobResumed,
			events.EventKindJobCompleted,
			events.EventKindJobRetryScheduled,
			events.EventKindJobFailed,
			events.EventKindJobCancelled,
			events.EventKindRunCompleted,
			events.EventKindRunFailed,
			events.EventKindRunCancelled,
			events.EventKindRunCrashed,
			events.EventKindSessionUpdate,
			events.EventKindShutdownRequested,
			events.EventKindShutdownDraining,
			events.EventKindShutdownTerminated:
			return true
		default:
			return false
		}
	case *events.Event:
		if value == nil {
			return false
		}
		return inputRequiresImmediateDispatch(*value)
	default:
		return false
	}
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
	deliver func(events.Event),
) (func(), <-chan struct{}) {
	done := make(chan struct{})
	var closeDoneOnce sync.Once
	closeDone := func() {
		closeDoneOnce.Do(func() {
			close(done)
		})
	}
	if bus == nil {
		return closeDone, done
	}

	_, updates, unsubscribe := bus.Subscribe()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		defer closeDone()
		defer unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-updates:
				if !ok {
					return
				}
				if deliver != nil {
					deliver(ev)
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
	if msg, ok := translateRunEvent(ev); ok {
		return msg, true
	}
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

func translateRunEvent(ev events.Event) (uiMsg, bool) {
	switch ev.Kind {
	case events.EventKindRunStarted:
		return runStatusMsg{Status: remoteRunStatusRunning}, true
	case events.EventKindRunCompleted:
		return runStatusMsg{Status: remoteRunStatusCompleted}, true
	case events.EventKindRunFailed:
		return runStatusMsg{Status: remoteRunStatusFailed}, true
	case events.EventKindRunCancelled:
		return runStatusMsg{Status: remoteRunStatusCanceled}, true
	case events.EventKindRunCrashed:
		return runStatusMsg{Status: remoteRunStatusCrashed}, true
	default:
		return nil, false
	}
}

func (t *uiEventTranslator) translateJobEvent(ev events.Event) (uiMsg, bool) {
	switch ev.Kind {
	case events.EventKindJobQueued:
		return translateJobQueuedEvent(ev)
	case events.EventKindJobStarted:
		return translateJobStartedEvent(ev)
	case events.EventKindJobCompleted:
		return translateJobCompletedEvent(ev)
	case events.EventKindJobRetryScheduled:
		return translateJobRetryScheduledEvent(ev)
	case events.EventKindJobPausing:
		return translateJobPausingEvent(ev)
	case events.EventKindJobPaused:
		return translateJobPausedEvent(ev)
	case events.EventKindJobResumed:
		return translateJobResumedEvent(ev)
	case events.EventKindJobFailed:
		return translateJobFailedEvent(ev)
	case events.EventKindJobCancelled:
		return translateJobCancelledEvent(ev)
	default:
		return nil, false
	}
}

func translateJobQueuedEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobQueuedPayload](ev)
	if !ok {
		return nil, false
	}
	codeFile := strings.TrimSpace(payload.CodeFile)
	if codeFile == "" && len(payload.CodeFiles) > 0 {
		codeFile = payload.CodeFiles[0]
	}
	return jobQueuedMsg{
		Index:           payload.Index,
		CodeFile:        codeFile,
		CodeFiles:       append([]string(nil), payload.CodeFiles...),
		Issues:          payload.Issues,
		TaskNumber:      payload.TaskNumber,
		TaskTitle:       payload.TaskTitle,
		TaskType:        payload.TaskType,
		SafeName:        payload.SafeName,
		IDE:             payload.IDE,
		Model:           payload.Model,
		ReasoningEffort: payload.ReasoningEffort,
		OutLog:          payload.OutLog,
		ErrLog:          payload.ErrLog,
	}, true
}

func translateJobStartedEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobStartedPayload](ev)
	if !ok {
		return nil, false
	}
	return jobStartedMsg{
		Index:           payload.Index,
		Attempt:         payload.Attempt,
		MaxAttempts:     payload.MaxAttempts,
		IDE:             payload.IDE,
		Model:           payload.Model,
		ReasoningEffort: payload.ReasoningEffort,
	}, true
}

func translateJobCompletedEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobCompletedPayload](ev)
	if !ok {
		return nil, false
	}
	return jobFinishedMsg{
		Index:    payload.Index,
		Success:  true,
		ExitCode: payload.ExitCode,
	}, true
}

func translateJobRetryScheduledEvent(ev events.Event) (uiMsg, bool) {
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
}

func translateJobPausingEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobPausingPayload](ev)
	if !ok {
		return nil, false
	}
	return jobPausingMsg{Index: payload.Index}, true
}

func translateJobPausedEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobPausedPayload](ev)
	if !ok {
		return nil, false
	}
	return jobPausedMsg{Index: payload.Index}, true
}

func translateJobResumedEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobResumedPayload](ev)
	if !ok {
		return nil, false
	}
	return jobResumedMsg{Index: payload.Index, MessageID: payload.MessageID}, true
}

func translateJobFailedEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobFailedPayload](ev)
	if !ok {
		return nil, false
	}
	return jobFailureMsg{Failure: jobFailureFromPayload(payload)}, true
}

func translateJobCancelledEvent(ev events.Event) (uiMsg, bool) {
	payload, ok := decodeUIEventPayload[kinds.JobCancelledPayload](ev)
	if !ok {
		return nil, false
	}
	return jobFinishedMsg{
		Index:    payload.Index,
		Success:  false,
		ExitCode: exitCodeCanceled,
	}, true
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
		Index:         payload.Index,
		Snapshot:      snapshot,
		UpdateKind:    update.Kind,
		ToolCallID:    update.ToolCallID,
		ToolCallState: update.ToolCallState,
		SessionStatus: update.Status,
	}, true
}

func (t *uiEventTranslator) hydrateSessionView(index int, snapshot SessionViewSnapshot) {
	t.sessionView(index).LoadSnapshot(snapshot)
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
