package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	multiRunTabsHeight = 2

	taskMultiStatusQueued    = "queued"
	taskMultiStatusRunning   = "running"
	taskMultiStatusCompleted = "completed"
	taskMultiStatusFailed    = "failed"
	taskMultiStatusCanceled  = "canceled"
)

var (
	setupRemoteMultiRunUISession = newMultiRunController
	newMultiRunTeaProgram        = defaultNewMultiRunTeaProgram
)

// RemoteMultiRunAttachOptions configures a daemon-backed multi-run UI attach session.
type RemoteMultiRunAttachOptions struct {
	Snapshot          apicore.TaskRunMultipleSnapshot
	Config            *config
	OwnerSession      bool
	LoadSnapshot      func(context.Context) (apicore.TaskRunMultipleSnapshot, error)
	LoadChildSnapshot func(context.Context, string) (apicore.RunSnapshot, error)
	OpenParentStream  func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error)
	OpenChildStream   func(context.Context, string, apicore.StreamCursor) (apiclient.RunStream, error)
}

type multiRunTab struct {
	slug       string
	status     string
	runID      string
	errorText  string
	child      *uiModel
	translator *uiEventTranslator
	terminal   bool
}

type multiRunModel struct {
	parentRun  apicore.Run
	tabs       []multiRunTab
	activeTab  int
	width      int
	height     int
	cfg        *config
	quitDialog quitDialogState
	shutdown   shutdownState
	onQuit     func(uiQuitRequest)
	now        time.Time
}

type multiRunController struct {
	model         *multiRunModel
	prog          *tea.Program
	done          chan error
	quitHandler   func(uiQuitRequest)
	quitHandlerMu sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	workers       sync.WaitGroup
	shutdownOnce  sync.Once
}

type multiRunChildBootstrapMsg struct {
	RunID    string
	Snapshot apicore.RunSnapshot
}

type multiRunChildEventMsg struct {
	RunID string
	Event events.Event
}

type remoteWorkerSession interface {
	Session
	StartRemoteWorker(func(context.Context))
}

type initialMultiRunChild struct {
	runID    string
	cursor   apicore.StreamCursor
	terminal bool
}

// AttachRemoteMultiple boots the tabbed Bubble Tea cockpit from a daemon-owned parent run snapshot.
func AttachRemoteMultiple(ctx context.Context, opts RemoteMultiRunAttachOptions) (Session, error) {
	mdl, initialChildren, err := newRemoteMultiRunModel(ctx, opts)
	if err != nil {
		return nil, err
	}
	session := setupRemoteMultiRunUISession(ctx, mdl)
	if session == nil {
		return nil, errors.New("remote multi-run ui session is required")
	}

	observedChildren := make(map[string]struct{}, len(initialChildren))
	var observedMu sync.Mutex
	observeChild := func(runID string, cursor apicore.StreamCursor, bootstrap bool) {
		trimmedRunID := strings.TrimSpace(runID)
		if trimmedRunID == "" {
			return
		}
		observedMu.Lock()
		if _, ok := observedChildren[trimmedRunID]; ok {
			observedMu.Unlock()
			return
		}
		observedChildren[trimmedRunID] = struct{}{}
		observedMu.Unlock()

		startRemoteWorker(session, func(workerCtx context.Context) {
			followRemoteMultiRunChild(workerCtx, session, opts, trimmedRunID, cursor, bootstrap)
		})
	}

	for _, child := range initialChildren {
		observeChild(child.runID, child.cursor, false)
	}
	if opts.OpenParentStream != nil && !isTerminalRunStatus(opts.Snapshot.Run.Status) {
		stream, err := opts.OpenParentStream(ctx, apicore.StreamCursor{})
		if err != nil {
			session.Shutdown()
			return nil, fmt.Errorf("open remote multi-run parent stream: %w", err)
		}
		startRemoteWorker(session, func(workerCtx context.Context) {
			followRemoteMultiRunParent(workerCtx, session, opts, stream, observeChild)
		})
	}
	return session, nil
}

func newRemoteMultiRunModel(
	ctx context.Context,
	opts RemoteMultiRunAttachOptions,
) (*multiRunModel, []initialMultiRunChild, error) {
	cfg := opts.Config
	if cfg == nil {
		cfg = &config{}
	}
	localCfg := *cfg
	localCfg.DetachOnly = !opts.OwnerSession
	localCfg.DaemonOwned = true

	mdl := &multiRunModel{
		parentRun:  opts.Snapshot.Run,
		width:      120,
		height:     40,
		cfg:        &localCfg,
		quitDialog: newQuitDialogState(),
		now:        time.Now(),
	}
	children := make([]initialMultiRunChild, 0, len(opts.Snapshot.Items))
	for _, item := range opts.Snapshot.Items {
		tab := newMultiRunTab(item)
		if tab.runID != "" && opts.LoadChildSnapshot != nil {
			snapshot, err := opts.LoadChildSnapshot(ctx, tab.runID)
			if err != nil {
				return nil, nil, fmt.Errorf("load child run snapshot %s: %w", tab.runID, err)
			}
			tab.applyChildSnapshot(snapshot, mdl.cfg, mdl.childWidth(), mdl.childHeight())
			children = append(children, initialMultiRunChild{
				runID:    tab.runID,
				cursor:   streamCursorOrZero(snapshot.NextCursor),
				terminal: isTerminalRunStatus(snapshot.Run.Status),
			})
		}
		mdl.tabs = append(mdl.tabs, tab)
	}
	mdl.activeTab = mdl.initialActiveTab()
	return mdl, nonTerminalInitialChildren(children), nil
}

func nonTerminalInitialChildren(children []initialMultiRunChild) []initialMultiRunChild {
	result := make([]initialMultiRunChild, 0, len(children))
	for _, child := range children {
		if !child.terminal {
			result = append(result, child)
		}
	}
	return result
}

func newMultiRunTab(item apicore.TaskRunMultipleItem) multiRunTab {
	status := strings.TrimSpace(item.Status)
	if status == "" {
		status = taskMultiStatusQueued
	}
	return multiRunTab{
		slug:       strings.TrimSpace(item.Slug),
		status:     status,
		runID:      strings.TrimSpace(item.RunID),
		errorText:  strings.TrimSpace(item.ErrorText),
		translator: newUIEventTranslator(),
		terminal:   isTerminalTaskMultiStatus(status),
	}
}

func newMultiRunController(ctx context.Context, mdl *multiRunModel) remoteWorkerSession {
	if ctx == nil {
		ctx = context.Background()
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	if mdl == nil {
		mdl = &multiRunModel{
			width:      120,
			height:     40,
			cfg:        &config{},
			quitDialog: newQuitDialogState(),
			now:        time.Now(),
		}
	}
	ctrl := &multiRunController{
		model:  mdl,
		done:   make(chan error, 1),
		ctx:    sessionCtx,
		cancel: cancel,
	}
	mdl.onQuit = ctrl.requestQuit
	ctrl.prog = newMultiRunTeaProgram(mdl)
	go func() {
		_, runErr := ctrl.prog.Run()
		if runErr != nil {
			ctrl.done <- runErr
		}
		close(ctrl.done)
	}()
	return ctrl
}

func defaultNewMultiRunTeaProgram(mdl tea.Model) *tea.Program {
	return tea.NewProgram(mdl, tea.WithoutSignalHandler())
}

func (c *multiRunController) Enqueue(msg any) {
	if c == nil || c.prog == nil {
		return
	}
	c.prog.Send(msg)
}

func (c *multiRunController) SetQuitHandler(fn func(uiQuitRequest)) {
	if c == nil {
		return
	}
	c.quitHandlerMu.Lock()
	defer c.quitHandlerMu.Unlock()
	c.quitHandler = fn
}

func (c *multiRunController) requestQuit(req uiQuitRequest) {
	c.quitHandlerMu.RLock()
	fn := c.quitHandler
	c.quitHandlerMu.RUnlock()
	if fn != nil {
		fn(req)
	}
}

func (c *multiRunController) CloseEvents() {}

func (c *multiRunController) Shutdown() {
	if c == nil {
		return
	}
	c.shutdownOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.prog != nil {
			c.prog.Quit()
		}
	})
}

func (c *multiRunController) Wait() error {
	if c == nil {
		return nil
	}
	err, ok := <-c.done
	if c.cancel != nil {
		c.cancel()
	}
	c.workers.Wait()
	if !ok {
		return nil
	}
	return err
}

func (c *multiRunController) StartRemoteWorker(fn func(context.Context)) {
	if c == nil || fn == nil {
		return
	}
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		fn(c.ctx)
	}()
}

func startRemoteWorker(session remoteWorkerSession, fn func(context.Context)) {
	if fn == nil {
		return
	}
	session.StartRemoteWorker(fn)
}

func followRemoteMultiRunParent(
	ctx context.Context,
	session Session,
	opts RemoteMultiRunAttachOptions,
	stream apiclient.RunStream,
	observeChild func(string, apicore.StreamCursor, bool),
) {
	parentSession := multiRunParentSession{
		Session:      session,
		observeChild: observeChild,
	}
	followOpts := RemoteAttachOptions{
		LoadSnapshot: func(loadCtx context.Context) (apicore.RunSnapshot, error) {
			if opts.LoadSnapshot == nil {
				return apicore.RunSnapshot{Run: opts.Snapshot.Run}, nil
			}
			snapshot, err := opts.LoadSnapshot(loadCtx)
			if err != nil {
				return apicore.RunSnapshot{}, err
			}
			return apicore.RunSnapshot{Run: snapshot.Run}, nil
		},
		OpenStream: opts.OpenParentStream,
	}
	followRemoteRun(ctx, parentSession, followOpts, stream, apicore.StreamCursor{})
}

type multiRunParentSession struct {
	Session
	observeChild func(string, apicore.StreamCursor, bool)
}

func (s multiRunParentSession) Enqueue(msg any) {
	if ev, ok := msg.(events.Event); ok {
		if childRunID := childRunIDFromTaskMultiEvent(ev); childRunID != "" && s.observeChild != nil {
			s.observeChild(childRunID, apicore.StreamCursor{}, true)
		}
	}
	s.Session.Enqueue(msg)
}

func followRemoteMultiRunChild(
	ctx context.Context,
	session Session,
	opts RemoteMultiRunAttachOptions,
	runID string,
	cursor apicore.StreamCursor,
	bootstrap bool,
) {
	if bootstrap {
		if opts.LoadChildSnapshot == nil {
			return
		}
		snapshot, err := opts.LoadChildSnapshot(ctx, runID)
		if err != nil {
			return
		}
		session.Enqueue(multiRunChildBootstrapMsg{RunID: runID, Snapshot: snapshot})
		cursor = streamCursorOrZero(snapshot.NextCursor)
		if isTerminalRunStatus(snapshot.Run.Status) {
			return
		}
	}
	if opts.OpenChildStream == nil {
		return
	}
	stream, err := opts.OpenChildStream(ctx, runID, cursor)
	if err != nil {
		return
	}
	followRemoteRun(ctx, multiRunChildSession{Session: session, runID: runID}, RemoteAttachOptions{
		LoadSnapshot: func(loadCtx context.Context) (apicore.RunSnapshot, error) {
			if opts.LoadChildSnapshot == nil {
				return apicore.RunSnapshot{}, nil
			}
			return opts.LoadChildSnapshot(loadCtx, runID)
		},
		OpenStream: func(streamCtx context.Context, after apicore.StreamCursor) (apiclient.RunStream, error) {
			return opts.OpenChildStream(streamCtx, runID, after)
		},
	}, stream, cursor)
}

type multiRunChildSession struct {
	Session
	runID string
}

func (s multiRunChildSession) Enqueue(msg any) {
	if ev, ok := msg.(events.Event); ok {
		s.Session.Enqueue(multiRunChildEventMsg{RunID: s.runID, Event: ev})
		return
	}
	s.Session.Enqueue(msg)
}

func childRunIDFromTaskMultiEvent(ev events.Event) string {
	switch ev.Kind {
	case events.EventKindTaskRunMultipleChildStarted,
		events.EventKindTaskRunMultipleChildCompleted,
		events.EventKindTaskRunMultipleChildFailed,
		events.EventKindTaskRunMultipleItemCanceled:
		payload, ok := decodeTaskMultiPayload(ev)
		if !ok {
			return ""
		}
		return strings.TrimSpace(payload.ChildRunID)
	default:
		return ""
	}
}

func (m *multiRunModel) Init() tea.Cmd {
	if child := m.activeChild(); child != nil {
		return child.Init()
	}
	return nil
}

func (m *multiRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case tea.KeyPressMsg:
		return m, m.handleKey(value)
	case tea.WindowSizeMsg:
		m.handleWindowSize(value)
		return m, nil
	case clockTickMsg:
		return m, m.handleClockTick(value)
	case spinnerTickMsg:
		return m, m.handleSpinnerTick(value)
	case events.Event:
		m.handleParentEvent(value)
		return m, nil
	case multiRunChildBootstrapMsg:
		m.handleChildBootstrap(value)
		return m, nil
	case multiRunChildEventMsg:
		return m, m.handleChildEvent(value)
	default:
		if child := m.activeChild(); child != nil {
			_, cmd := child.Update(msg)
			return m, cmd
		}
		return m, nil
	}
}

func (m *multiRunModel) handleKey(v tea.KeyPressMsg) tea.Cmd {
	if m.quitDialog.Active {
		return m.handleQuitDialogKey(v)
	}
	switch strings.ToLower(v.String()) {
	case keyCtrlC, "q":
		return m.handleQuitKey()
	case "[", "shift+left":
		m.moveActiveTab(-1)
		return nil
	case "]", "shift+right":
		m.moveActiveTab(1)
		return nil
	default:
		if child := m.activeChild(); child != nil {
			_, cmd := child.Update(v)
			return cmd
		}
		return nil
	}
}

func (m *multiRunModel) handleQuitKey() tea.Cmd {
	if m.cfg != nil && m.cfg.DetachOnly {
		return tea.Quit
	}
	if m.isQueueComplete() {
		return tea.Quit
	}
	if !m.shutdown.Active() {
		m.quitDialog.Open()
		return nil
	}
	return m.requestQueueStopFromQuit()
}

func (m *multiRunModel) handleQuitDialogKey(v tea.KeyPressMsg) tea.Cmd {
	switch strings.ToLower(v.String()) {
	case "left", "h", keyShiftTab:
		m.quitDialog.Move(-1)
		return nil
	case "right", "l", keyTab:
		m.quitDialog.Move(1)
		return nil
	case keyEnter, "q", keyCtrlC:
		return m.confirmQuitDialog()
	case keyEscape:
		m.quitDialog.Close()
		return nil
	default:
		return nil
	}
}

func (m *multiRunModel) confirmQuitDialog() tea.Cmd {
	selected := m.quitDialog.Selected
	m.quitDialog.Close()
	switch selected {
	case quitDialogActionClose:
		return tea.Quit
	case quitDialogActionStop:
		return m.requestQueueStopFromQuit()
	default:
		return nil
	}
}

func (m *multiRunModel) requestQueueStopFromQuit() tea.Cmd {
	req, ok := m.nextQuitRequest()
	if !ok {
		return nil
	}
	m.markCancelableTabsCanceled("stop requested")
	if m.onQuit == nil {
		return nil
	}
	return func() tea.Msg {
		m.onQuit(req)
		return drainMsg{}
	}
}

func (m *multiRunModel) nextQuitRequest() (uiQuitRequest, bool) {
	now := time.Now()
	m.now = now
	switch m.shutdown.Phase {
	case shutdownPhaseIdle:
		m.shutdown = shutdownState{
			Phase:       shutdownPhaseDraining,
			Source:      shutdownSourceUI,
			RequestedAt: now,
			DeadlineAt:  now.Add(gracefulShutdownTimeout),
		}
		return uiQuitRequestDrain, true
	case shutdownPhaseDraining:
		m.shutdown = shutdownState{
			Phase:       shutdownPhaseForcing,
			Source:      shutdownSourceUI,
			RequestedAt: now,
		}
		return uiQuitRequestForce, true
	default:
		return uiQuitRequestDrain, false
	}
}

func (m *multiRunModel) markCancelableTabsCanceled(message string) {
	for idx := range m.tabs {
		switch m.tabs[idx].status {
		case "", taskMultiStatusQueued, taskMultiStatusRunning:
			m.tabs[idx].status = taskMultiStatusCanceled
			m.tabs[idx].terminal = true
			if strings.TrimSpace(m.tabs[idx].errorText) == "" {
				m.tabs[idx].errorText = message
			}
		}
	}
}

func (m *multiRunModel) handleWindowSize(v tea.WindowSizeMsg) {
	m.width = v.Width
	m.height = v.Height
	for idx := range m.tabs {
		if m.tabs[idx].child != nil {
			m.resizeChild(m.tabs[idx].child)
		}
	}
}

func (m *multiRunModel) handleClockTick(v clockTickMsg) tea.Cmd {
	if !v.at.IsZero() {
		m.now = v.at
	}
	if child := m.activeChild(); child != nil {
		return child.handleClockTick(v)
	}
	return nil
}

func (m *multiRunModel) handleSpinnerTick(v spinnerTickMsg) tea.Cmd {
	if child := m.activeChild(); child != nil {
		return child.handleSpinnerTick(v)
	}
	return nil
}

func (m *multiRunModel) handleParentEvent(ev events.Event) {
	switch ev.Kind {
	case events.EventKindTaskRunMultipleStarted:
		m.applyTaskMultiStarted(ev)
	case events.EventKindTaskRunMultipleItemQueued,
		events.EventKindTaskRunMultipleChildStarted,
		events.EventKindTaskRunMultipleChildCompleted,
		events.EventKindTaskRunMultipleChildFailed,
		events.EventKindTaskRunMultipleItemCanceled:
		m.applyTaskMultiItem(ev)
	case events.EventKindTaskRunMultipleQueueCompleted:
		m.parentRun.Status = remoteRunStatusCompleted
		m.quitDialog.Close()
		m.shutdown = shutdownState{}
	case events.EventKindTaskRunMultipleQueueCanceled:
		m.parentRun.Status = remoteRunStatusCanceled
		m.applyTaskMultiQueueCanceled(ev)
		m.quitDialog.Close()
	case events.EventKindRunCompleted:
		m.parentRun.Status = remoteRunStatusCompleted
		m.quitDialog.Close()
	case events.EventKindRunFailed:
		m.parentRun.Status = remoteRunStatusFailed
		m.quitDialog.Close()
	case events.EventKindRunCancelled:
		m.parentRun.Status = remoteRunStatusCanceled
		m.quitDialog.Close()
	default:
		return
	}
}

func (m *multiRunModel) applyTaskMultiStarted(ev events.Event) {
	payload, ok := decodeTaskMultiPayload(ev)
	if !ok {
		return
	}
	if payload.Status != "" {
		m.parentRun.Status = payload.Status
	}
	for _, slug := range payload.Slugs {
		m.ensureTab(strings.TrimSpace(slug))
	}
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		m.activeTab = m.initialActiveTab()
	}
}

func (m *multiRunModel) applyTaskMultiItem(ev events.Event) {
	payload, ok := decodeTaskMultiPayload(ev)
	if !ok {
		return
	}
	idx := m.ensureTabForPayload(payload)
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	tab := &m.tabs[idx]
	if payload.Status != "" {
		tab.status = strings.TrimSpace(payload.Status)
	}
	if childRunID := strings.TrimSpace(payload.ChildRunID); childRunID != "" {
		tab.runID = childRunID
	}
	if errorText := strings.TrimSpace(payload.Error); errorText != "" {
		tab.errorText = errorText
	}
	tab.terminal = isTerminalTaskMultiStatus(tab.status)
	if idx == m.activeTab && tab.terminal {
		m.advanceActiveTabAfterTerminal()
	}
}

func (m *multiRunModel) applyTaskMultiQueueCanceled(ev events.Event) {
	payload, ok := decodeTaskMultiPayload(ev)
	if !ok {
		return
	}
	message := strings.TrimSpace(payload.Error)
	for idx := range m.tabs {
		if !isTerminalTaskMultiStatus(m.tabs[idx].status) {
			m.tabs[idx].status = taskMultiStatusCanceled
			m.tabs[idx].terminal = true
			if m.tabs[idx].errorText == "" {
				m.tabs[idx].errorText = message
			}
		}
	}
}

func (m *multiRunModel) handleChildBootstrap(msg multiRunChildBootstrapMsg) {
	idx := m.findTabByRunID(msg.RunID)
	if idx < 0 {
		return
	}
	m.tabs[idx].applyChildSnapshot(msg.Snapshot, m.cfg, m.childWidth(), m.childHeight())
	if status := taskMultiStatusFromRunStatus(msg.Snapshot.Run.Status); status != "" {
		m.tabs[idx].status = status
		m.tabs[idx].terminal = isTerminalTaskMultiStatus(status)
	}
}

func (m *multiRunModel) handleChildEvent(msg multiRunChildEventMsg) tea.Cmd {
	idx := m.findTabByRunID(msg.RunID)
	if idx < 0 {
		return nil
	}
	tab := &m.tabs[idx]
	if tab.child == nil {
		tab.child = newPlaceholderChildModel(tab.slug, m.cfg, m.childWidth(), m.childHeight())
	}
	if tab.translator == nil {
		tab.translator = newUIEventTranslator()
	}
	var cmd tea.Cmd
	for _, uiMsg := range tab.translator.translateMessages(msg.Event) {
		if nextCmd := applyChildUIMsg(tab, uiMsg); nextCmd != nil && idx == m.activeTab {
			cmd = nextCmd
		}
	}
	if status := taskMultiStatusFromChildRunEvent(msg.Event.Kind); status != "" {
		tab.status = status
		tab.terminal = isTerminalTaskMultiStatus(status)
	}
	if idx == m.activeTab && tab.terminal {
		m.advanceActiveTabAfterTerminal()
	}
	return cmd
}

func (m *multiRunModel) View() tea.View {
	if m.quitDialog.Active {
		return m.renderQuitDialogView()
	}
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderTabs(),
		m.renderActiveTabContent(),
	)
	return m.renderRoot(content)
}

func (m *multiRunModel) renderRoot(content string) tea.View {
	v := tea.NewView(rootScreenStyle(m.width, m.height).Render(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *multiRunModel) renderTabs() string {
	bg := colorBgBase
	chunks := make([]string, 0, len(m.tabs))
	for idx := range m.tabs {
		tab := m.tabs[idx]
		label := fmt.Sprintf(
			"%d %s %s",
			idx+1,
			firstNonEmpty(tab.slug, tab.runID, "workflow"),
			strings.ToUpper(tab.status),
		)
		style := lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorBgSurface).
			Foreground(multiRunStatusColor(tab.status))
		if idx == m.activeTab {
			style = style.Bold(true).Background(colorAccent).Foreground(colorBgBase)
		}
		chunks = append(chunks, style.Render(truncateString(label, 32)))
	}
	left := renderGap(bg, 1) + strings.Join(chunks, renderGap(bg, 1))
	hint := renderKeycap("[/]", bg) + renderGap(bg, 1) + renderStyledOnBackground(styleMutedText, bg, "TABS")
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(hint)-1, 1)
	line := renderOwnedLineKnownOwned(m.width, bg, left+renderGap(bg, gap)+hint)
	separator := renderOwnedLineKnownOwned(
		m.width,
		bg,
		renderStyledOnBackground(styleSeparator, bg, strings.Repeat("─", m.width)),
	)
	return line + "\n" + separator
}

func (m *multiRunModel) renderActiveTabContent() string {
	tab := m.activeTabState()
	if tab == nil {
		return m.renderQueuedTabContent(nil)
	}
	if tab.child == nil {
		return m.renderQueuedTabContent(tab)
	}
	m.resizeChild(tab.child)
	return tab.child.View().Content
}

func (m *multiRunModel) renderQueuedTabContent(tab *multiRunTab) string {
	width := max(m.width, 1)
	height := max(m.childHeight(), 1)
	panelWidth := max(width-4, 20)
	innerStyle := techPanelStyle(panelWidth, multiRunStatusColor(tabStatus(tab))).Padding(1, 2)
	innerWidth := max(panelWidth-innerStyle.GetHorizontalFrameSize(), 1)
	slug := "workflow"
	status := taskMultiStatusQueued
	errText := ""
	if tab != nil {
		slug = firstNonEmpty(tab.slug, tab.runID, "workflow")
		status = tabStatus(tab)
		errText = strings.TrimSpace(tab.errorText)
	}
	lines := []string{
		renderOwnedLineKnownOwned(innerWidth, colorBgSurface, renderTechLabel("queue."+status, colorBgSurface)),
		renderOwnedLineKnownOwned(innerWidth, colorBgSurface, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			colorBgSurface,
			renderStyledOnBackground(styleBodyText, colorBgSurface, truncateString(slug, innerWidth)),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			colorBgSurface,
			renderStyledOnBackground(
				styleMutedText,
				colorBgSurface,
				truncateString("Child run has not started yet.", innerWidth),
			),
		),
	}
	if errText != "" {
		lines = append(lines, renderOwnedLineKnownOwned(
			innerWidth,
			colorBgSurface,
			renderStyledOnBackground(styleMutedText, colorBgSurface, truncateString(errText, innerWidth)),
		))
	}
	panel := innerStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		panel,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(colorBgBase)),
	)
}

func (m *multiRunModel) renderQuitDialogView() tea.View {
	panel := m.renderQuitDialogPanel()
	content := lipgloss.Place(
		max(m.width, 1),
		max(m.height, 1),
		lipgloss.Center,
		lipgloss.Center,
		panel,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(colorBgBase)),
	)
	return m.renderRoot(content)
}

func (m *multiRunModel) renderQuitDialogPanel() string {
	availableWidth := max(m.width-4, 1)
	panelWidth := min(availableWidth, quitDialogMaxWidth)
	panelStyle := techPanelStyle(panelWidth, colorBorderFocus).Padding(1, 2)
	innerWidth := max(panelWidth-panelStyle.GetHorizontalFrameSize(), 1)
	bg := colorBgSurface
	lines := []string{
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				lipgloss.NewStyle().Bold(true).Foreground(colorAccentDeep),
				bg,
				truncateString("Leave Active Queue?", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(styleBodyText, bg, truncateString("This queue is still active.", innerWidth)),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Close the TUI and keep queued work running.", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Choose Stop Run to cancel active and queued work.", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedBlock(innerWidth, bg, m.renderQuitDialogActions(innerWidth, bg)),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleDimText,
				bg,
				truncateString("[enter/q] confirm  [tab/left/right] choice  [esc] back", innerWidth),
			),
		),
	}
	return panelStyle.Render(strings.Join(lines, "\n"))
}

func (m *multiRunModel) renderQuitDialogActions(width int, bg color.Color) string {
	actions := []string{
		m.renderQuitDialogAction("Close TUI", quitDialogActionClose),
		m.renderQuitDialogAction("Stop Run", quitDialogActionStop),
		m.renderQuitDialogAction("Cancel", quitDialogActionCancel),
	}
	if width < 44 {
		return strings.Join(actions, "\n")
	}
	return strings.Join(actions, renderGap(bg, 1))
}

func (m *multiRunModel) renderQuitDialogAction(label string, action quitDialogAction) string {
	baseStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if m.quitDialog.Selected == action {
		return baseStyle.Foreground(colorBgSurface).Background(colorAccent).Render(label)
	}
	return baseStyle.Foreground(colorFgBright).Background(colorBgBase).Render(label)
}

func (m *multiRunModel) activeChild() *uiModel {
	tab := m.activeTabState()
	if tab == nil {
		return nil
	}
	return tab.child
}

func (m *multiRunModel) activeTabState() *multiRunTab {
	if m == nil || m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return nil
	}
	return &m.tabs[m.activeTab]
}

func (m *multiRunModel) moveActiveTab(delta int) {
	if len(m.tabs) == 0 {
		return
	}
	if child := m.activeChild(); child != nil {
		child.persistSelectedViewportState()
	}
	next := (m.activeTab + delta + len(m.tabs)) % len(m.tabs)
	m.activeTab = next
	if child := m.activeChild(); child != nil {
		m.resizeChild(child)
		child.refreshViewportContent()
	}
}

func (m *multiRunModel) advanceActiveTabAfterTerminal() {
	if len(m.tabs) == 0 {
		return
	}
	for idx := m.activeTab + 1; idx < len(m.tabs); idx++ {
		if !isTerminalTaskMultiStatus(m.tabs[idx].status) {
			m.activeTab = idx
			return
		}
	}
	for idx := range m.tabs {
		if !isTerminalTaskMultiStatus(m.tabs[idx].status) {
			m.activeTab = idx
			return
		}
	}
}

func (m *multiRunModel) initialActiveTab() int {
	for idx := range m.tabs {
		if m.tabs[idx].status == taskMultiStatusRunning {
			return idx
		}
	}
	for idx := range m.tabs {
		if !isTerminalTaskMultiStatus(m.tabs[idx].status) {
			return idx
		}
	}
	return 0
}

func (m *multiRunModel) ensureTab(slug string) int {
	trimmed := strings.TrimSpace(slug)
	for idx := range m.tabs {
		if m.tabs[idx].slug == trimmed {
			return idx
		}
	}
	m.tabs = append(m.tabs, multiRunTab{
		slug:       trimmed,
		status:     taskMultiStatusQueued,
		translator: newUIEventTranslator(),
	})
	return len(m.tabs) - 1
}

func (m *multiRunModel) ensureTabForPayload(payload kinds.TaskRunMultiplePayload) int {
	if payload.Index >= 0 && payload.Index < len(m.tabs) {
		return payload.Index
	}
	if slug := strings.TrimSpace(payload.Slug); slug != "" {
		return m.ensureTab(slug)
	}
	if childRunID := strings.TrimSpace(payload.ChildRunID); childRunID != "" {
		if idx := m.findTabByRunID(childRunID); idx >= 0 {
			return idx
		}
	}
	return -1
}

func (m *multiRunModel) findTabByRunID(runID string) int {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return -1
	}
	for idx := range m.tabs {
		if m.tabs[idx].runID == trimmed {
			return idx
		}
	}
	return -1
}

func (m *multiRunModel) isQueueComplete() bool {
	if isTerminalRunStatus(m.parentRun.Status) {
		return true
	}
	if len(m.tabs) == 0 {
		return false
	}
	for idx := range m.tabs {
		if !isTerminalTaskMultiStatus(m.tabs[idx].status) {
			return false
		}
	}
	return true
}

func (m *multiRunModel) resizeChild(child *uiModel) {
	if child == nil {
		return
	}
	child.handleWindowSize(tea.WindowSizeMsg{
		Width:  m.childWidth(),
		Height: m.childHeight(),
	})
}

func (m *multiRunModel) childWidth() int {
	return max(m.width, 1)
}

func (m *multiRunModel) childHeight() int {
	return max(m.height-multiRunTabsHeight, 1)
}

func (t *multiRunTab) applyChildSnapshot(snapshot apicore.RunSnapshot, cfg *config, width int, height int) {
	t.runID = firstNonEmpty(t.runID, snapshot.Run.RunID)
	t.child = childModelFromRunSnapshot(snapshot, cfg, width, height)
	t.translator = newUIEventTranslator()
	for idx := range t.child.jobs {
		t.translator.hydrateSessionView(idx, t.child.jobs[idx].snapshot)
	}
}

func childModelFromRunSnapshot(snapshot apicore.RunSnapshot, cfg *config, width int, height int) *uiModel {
	jobs, msgs := remoteSnapshotBootstrap(snapshot)
	if len(jobs) == 0 {
		jobs = []job{{
			SafeName:  firstNonEmpty(snapshot.Run.WorkflowSlug, snapshot.Run.RunID, "workflow"),
			TaskTitle: firstNonEmpty(snapshot.Run.WorkflowSlug, "Starting workflow"),
		}}
	}
	mdl := newUIModel(len(jobs))
	mdl.cfg = cfg
	mdl.handleWindowSize(tea.WindowSizeMsg{Width: width, Height: height})
	applyBootstrapJobsToModel(mdl, jobs)
	for _, msg := range msgs {
		mdl.applyUIMsg(msg)
	}
	return mdl
}

func newPlaceholderChildModel(slug string, cfg *config, width int, height int) *uiModel {
	return childModelFromRunSnapshot(apicore.RunSnapshot{
		Run: apicore.Run{
			RunID:        slug,
			WorkflowSlug: slug,
			Status:       remoteRunStatusRunning,
		},
	}, cfg, width, height)
}

func applyBootstrapJobsToModel(mdl *uiModel, jobs []job) {
	for idx := range jobs {
		jb := jobs[idx]
		totalIssues := 0
		for _, items := range jb.Groups {
			totalIssues += len(items)
		}
		codeFileLabel := jb.CodeFileLabel()
		if len(jb.CodeFiles) > 3 {
			codeFileLabel = fmt.Sprintf("%s and %d more", strings.Join(jb.CodeFiles[:3], ", "), len(jb.CodeFiles)-3)
		}
		mdl.applyUIMsg(jobQueuedMsg{
			Index:           idx,
			CodeFile:        codeFileLabel,
			CodeFiles:       append([]string(nil), jb.CodeFiles...),
			Issues:          totalIssues,
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
}

func applyChildUIMsg(tab *multiRunTab, msg uiMsg) tea.Cmd {
	if tab == nil || tab.child == nil {
		return nil
	}
	if update, ok := msg.(jobUpdateMsg); ok && update.HydrateTranslator {
		if tab.translator == nil {
			tab.translator = newUIEventTranslator()
		}
		tab.translator.hydrateSessionView(update.Index, update.Snapshot)
	}
	return tab.child.applyUIMsg(msg)
}

func tabStatus(tab *multiRunTab) string {
	if tab == nil {
		return taskMultiStatusQueued
	}
	if status := strings.TrimSpace(tab.status); status != "" {
		return status
	}
	return taskMultiStatusQueued
}

func isTerminalTaskMultiStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case taskMultiStatusCompleted, taskMultiStatusFailed, taskMultiStatusCanceled:
		return true
	default:
		return false
	}
}

func taskMultiStatusFromRunStatus(status string) string {
	switch strings.TrimSpace(status) {
	case remoteRunStatusRunning, remoteRunStatusRetrying:
		return taskMultiStatusRunning
	case remoteRunStatusCompleted:
		return taskMultiStatusCompleted
	case remoteRunStatusFailed, remoteRunStatusCrashed:
		return taskMultiStatusFailed
	case remoteRunStatusCanceled:
		return taskMultiStatusCanceled
	default:
		return ""
	}
}

func taskMultiStatusFromChildRunEvent(kind events.EventKind) string {
	switch kind {
	case events.EventKindRunCompleted:
		return taskMultiStatusCompleted
	case events.EventKindRunFailed:
		return taskMultiStatusFailed
	case events.EventKindRunCancelled:
		return taskMultiStatusCanceled
	default:
		return ""
	}
}

func multiRunStatusColor(status string) color.Color {
	switch strings.TrimSpace(status) {
	case taskMultiStatusRunning:
		return colorAccentAlt
	case taskMultiStatusCompleted:
		return colorSuccess
	case taskMultiStatusFailed:
		return colorError
	case taskMultiStatusCanceled:
		return colorWarning
	default:
		return colorMuted
	}
}

func decodeTaskMultiPayload(ev events.Event) (kinds.TaskRunMultiplePayload, bool) {
	var payload kinds.TaskRunMultiplePayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return kinds.TaskRunMultiplePayload{}, false
	}
	return payload, true
}
