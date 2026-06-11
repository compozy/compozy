package ui

import (
	"context"
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
	reviewWatchTabsHeight = 2
	reviewWatchMaxLines   = 200

	reviewWatchStatusWatching = "watching"
	reviewWatchStatusWaiting  = "waiting"
	reviewWatchStatusFixing   = "fixing"
	reviewWatchStatusPushing  = "pushing"
	reviewWatchStatusClean    = "clean"
	reviewWatchStatusDone     = "done"
	reviewWatchStatusFailed   = "failed"
	reviewWatchStatusCanceled = "canceled"
)

var (
	setupRemoteReviewWatchUISession = newReviewWatchController
	newReviewWatchTeaProgram        = defaultNewReviewWatchTeaProgram
)

// RemoteReviewWatchAttachOptions configures a daemon-backed review-watch UI attach session.
type RemoteReviewWatchAttachOptions struct {
	Snapshot          apicore.RunSnapshot
	Config            *config
	OwnerSession      bool
	LoadSnapshot      func(context.Context) (apicore.RunSnapshot, error)
	LoadChildSnapshot func(context.Context, string) (apicore.RunSnapshot, error)
	OpenParentStream  func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error)
	OpenChildStream   func(context.Context, string, apicore.StreamCursor) (apiclient.RunStream, error)
}

type reviewWatchOverview struct {
	provider        string
	pr              string
	workflow        string
	round           int
	headSHA         string
	reviewID        string
	reviewState     string
	status          string
	remote          string
	branch          string
	total           int
	resolved        int
	unresolved      int
	dirty           bool
	unpushedCommits int
	phase           string
	lastError       string
	lines           []string
}

type reviewWatchModel struct {
	parentRun  apicore.Run
	overview   reviewWatchOverview
	children   []multiRunTab
	activeTab  int
	width      int
	height     int
	cfg        *config
	quitDialog quitDialogState
	shutdown   shutdownState
	onQuit     func(uiQuitRequest)
	now        time.Time
}

type reviewWatchController struct {
	model         *reviewWatchModel
	prog          *tea.Program
	done          chan error
	quitHandler   func(uiQuitRequest)
	quitHandlerMu sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	workers       sync.WaitGroup
	shutdownOnce  sync.Once
}

// AttachRemoteReviewWatch boots the tabbed review-watch cockpit from a daemon-owned parent snapshot.
func AttachRemoteReviewWatch(ctx context.Context, opts RemoteReviewWatchAttachOptions) (Session, error) {
	mdl := newRemoteReviewWatchModel(opts)
	session := setupRemoteReviewWatchUISession(ctx, mdl)
	if session == nil {
		return nil, errors.New("remote review-watch ui session is required")
	}

	observedChildren := make(map[string]struct{})
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
			followRemoteReviewWatchChild(workerCtx, session, opts, trimmedRunID, cursor, bootstrap)
		})
	}

	if opts.OpenParentStream != nil {
		stream, err := opts.OpenParentStream(ctx, apicore.StreamCursor{})
		if err != nil {
			session.Shutdown()
			return nil, fmt.Errorf("open remote review-watch parent stream: %w", err)
		}
		startRemoteWorker(session, func(workerCtx context.Context) {
			followRemoteReviewWatchParent(workerCtx, session, opts, stream, observeChild)
		})
	}
	return session, nil
}

func newRemoteReviewWatchModel(opts RemoteReviewWatchAttachOptions) *reviewWatchModel {
	cfg := opts.Config
	if cfg == nil {
		cfg = &config{}
	}
	localCfg := *cfg
	localCfg.DetachOnly = !opts.OwnerSession
	localCfg.DaemonOwned = true

	workflow := strings.TrimSpace(opts.Snapshot.Run.WorkflowSlug)
	mdl := &reviewWatchModel{
		parentRun: opts.Snapshot.Run,
		overview: reviewWatchOverview{
			workflow: workflow,
			phase:    reviewWatchStatusWatching,
			status:   strings.TrimSpace(opts.Snapshot.Run.Status),
		},
		width:      120,
		height:     40,
		cfg:        &localCfg,
		quitDialog: newQuitDialogState(),
		now:        time.Now(),
	}
	return mdl
}

func newReviewWatchController(ctx context.Context, mdl *reviewWatchModel) remoteWorkerSession {
	if ctx == nil {
		ctx = context.Background()
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	if mdl == nil {
		mdl = newRemoteReviewWatchModel(RemoteReviewWatchAttachOptions{})
	}
	ctrl := &reviewWatchController{
		model:  mdl,
		done:   make(chan error, 1),
		ctx:    sessionCtx,
		cancel: cancel,
	}
	mdl.onQuit = ctrl.requestQuit
	ctrl.prog = newReviewWatchTeaProgram(mdl)
	go func() {
		_, runErr := ctrl.prog.Run()
		if runErr != nil {
			ctrl.done <- runErr
		}
		close(ctrl.done)
	}()
	return ctrl
}

func defaultNewReviewWatchTeaProgram(mdl tea.Model) *tea.Program {
	return tea.NewProgram(mdl, tea.WithoutSignalHandler())
}

func (c *reviewWatchController) Enqueue(msg any) {
	if c == nil || c.prog == nil {
		return
	}
	c.prog.Send(msg)
}

func (c *reviewWatchController) SetQuitHandler(fn func(uiQuitRequest)) {
	if c == nil {
		return
	}
	c.quitHandlerMu.Lock()
	defer c.quitHandlerMu.Unlock()
	c.quitHandler = fn
}

func (c *reviewWatchController) requestQuit(req uiQuitRequest) {
	c.quitHandlerMu.RLock()
	fn := c.quitHandler
	c.quitHandlerMu.RUnlock()
	if fn != nil {
		fn(req)
	}
}

func (c *reviewWatchController) CloseEvents() {}

func (c *reviewWatchController) Shutdown() {
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

func (c *reviewWatchController) Wait() error {
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

func (c *reviewWatchController) StartRemoteWorker(fn func(context.Context)) {
	if c == nil || fn == nil {
		return
	}
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		fn(c.ctx)
	}()
}

func followRemoteReviewWatchParent(
	ctx context.Context,
	session Session,
	opts RemoteReviewWatchAttachOptions,
	stream apiclient.RunStream,
	observeChild func(string, apicore.StreamCursor, bool),
) {
	parentSession := reviewWatchParentSession{
		Session:      session,
		observeChild: observeChild,
	}
	followOpts := RemoteAttachOptions{
		LoadSnapshot: func(loadCtx context.Context) (apicore.RunSnapshot, error) {
			if opts.LoadSnapshot == nil {
				return opts.Snapshot, nil
			}
			return opts.LoadSnapshot(loadCtx)
		},
		OpenStream: opts.OpenParentStream,
	}
	followRemoteRun(ctx, parentSession, followOpts, stream, apicore.StreamCursor{})
}

type reviewWatchParentSession struct {
	Session
	observeChild func(string, apicore.StreamCursor, bool)
}

func (s reviewWatchParentSession) Enqueue(msg any) {
	s.Session.Enqueue(msg)
	if ev, ok := msg.(events.Event); ok {
		if childRunID := childRunIDFromReviewWatchEvent(ev); childRunID != "" && s.observeChild != nil {
			s.observeChild(childRunID, apicore.StreamCursor{}, true)
		}
	}
}

func followRemoteReviewWatchChild(
	ctx context.Context,
	session Session,
	opts RemoteReviewWatchAttachOptions,
	runID string,
	cursor apicore.StreamCursor,
	bootstrap bool,
) {
	if bootstrap && opts.LoadChildSnapshot != nil {
		snapshot, err := opts.LoadChildSnapshot(ctx, runID)
		if err == nil {
			session.Enqueue(multiRunChildBootstrapMsg{RunID: runID, Snapshot: snapshot})
			cursor = streamCursorOrZero(snapshot.NextCursor)
			if isTerminalRunStatus(snapshot.Run.Status) {
				return
			}
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

func childRunIDFromReviewWatchEvent(ev events.Event) string {
	switch ev.Kind {
	case events.EventKindReviewWatchFixStarted, events.EventKindReviewWatchFixCompleted:
		payload, ok := decodeReviewWatchPayload(ev)
		if !ok {
			return ""
		}
		return strings.TrimSpace(payload.ChildRunID)
	default:
		return ""
	}
}

func (m *reviewWatchModel) Init() tea.Cmd {
	return m.clockTick()
}

func (m *reviewWatchModel) clockTick() tea.Cmd {
	return tea.Every(uiClockTickInterval, func(at time.Time) tea.Msg {
		return clockTickMsg{at: at}
	})
}

func (m *reviewWatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *reviewWatchModel) handleKey(v tea.KeyPressMsg) tea.Cmd {
	if m.quitDialog.Active {
		return m.handleQuitDialogKey(v)
	}
	switch strings.ToLower(v.String()) {
	case keyCtrlC, "q":
		return m.handleQuitKey()
	case keyLeft, "h":
		m.moveActiveTab(-1)
		return nil
	case keyRight, "l":
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

func (m *reviewWatchModel) handleQuitKey() tea.Cmd {
	if m.cfg != nil && m.cfg.DetachOnly {
		return tea.Quit
	}
	if isTerminalRunStatus(m.parentRun.Status) {
		return tea.Quit
	}
	if !m.shutdown.Active() {
		m.quitDialog.Open()
		return nil
	}
	return m.requestStopFromQuit()
}

func (m *reviewWatchModel) handleQuitDialogKey(v tea.KeyPressMsg) tea.Cmd {
	switch strings.ToLower(v.String()) {
	case keyLeft, "h", keyShiftTab:
		m.quitDialog.Move(-1)
		return nil
	case keyRight, "l", keyTab:
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

func (m *reviewWatchModel) confirmQuitDialog() tea.Cmd {
	selected := m.quitDialog.Selected
	m.quitDialog.Close()
	switch selected {
	case quitDialogActionClose:
		return tea.Quit
	case quitDialogActionStop:
		return m.requestStopFromQuit()
	default:
		return nil
	}
}

func (m *reviewWatchModel) requestStopFromQuit() tea.Cmd {
	req, ok := m.nextQuitRequest()
	if !ok {
		return nil
	}
	m.markActiveChildrenCanceled("stop requested")
	if m.onQuit == nil {
		return nil
	}
	return func() tea.Msg {
		m.onQuit(req)
		return drainMsg{}
	}
}

func (m *reviewWatchModel) nextQuitRequest() (uiQuitRequest, bool) {
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

func (m *reviewWatchModel) markActiveChildrenCanceled(message string) {
	for idx := range m.children {
		if !isTerminalTaskMultiStatus(m.children[idx].status) {
			m.children[idx].status = taskMultiStatusCanceled
			m.children[idx].terminal = true
			if strings.TrimSpace(m.children[idx].errorText) == "" {
				m.children[idx].errorText = message
			}
		}
	}
}

func (m *reviewWatchModel) handleWindowSize(v tea.WindowSizeMsg) {
	m.width = v.Width
	m.height = v.Height
	for idx := range m.children {
		m.resizeChild(m.children[idx].child)
	}
}

func (m *reviewWatchModel) handleClockTick(v clockTickMsg) tea.Cmd {
	if !v.at.IsZero() {
		m.now = v.at
	}
	if child := m.activeChild(); child != nil {
		child.handleClockTick(v)
	}
	return m.clockTick()
}

func (m *reviewWatchModel) handleSpinnerTick(v spinnerTickMsg) tea.Cmd {
	if child := m.activeChild(); child != nil {
		return child.handleSpinnerTick(v)
	}
	return nil
}

func (m *reviewWatchModel) handleParentEvent(ev events.Event) {
	switch ev.Kind {
	case events.EventKindReviewWatchStarted,
		events.EventKindReviewWatchWaiting,
		events.EventKindReviewWatchRoundFetched,
		events.EventKindReviewWatchFixStarted,
		events.EventKindReviewWatchFixCompleted,
		events.EventKindReviewWatchPushStarted,
		events.EventKindReviewWatchPushCompleted,
		events.EventKindReviewWatchPushFailed,
		events.EventKindReviewWatchClean,
		events.EventKindReviewWatchMaxRounds:
		payload, ok := decodeReviewWatchPayload(ev)
		if !ok {
			return
		}
		m.applyReviewWatchPayload(ev.Kind, payload)
	case events.EventKindRunCompleted:
		m.parentRun.Status = remoteRunStatusCompleted
		m.overview.phase = reviewWatchStatusDone
		m.quitDialog.Close()
		m.shutdown = shutdownState{}
	case events.EventKindRunFailed:
		m.parentRun.Status = remoteRunStatusFailed
		m.overview.phase = reviewWatchStatusFailed
		m.quitDialog.Close()
	case events.EventKindRunCancelled:
		m.parentRun.Status = remoteRunStatusCanceled
		m.overview.phase = reviewWatchStatusCanceled
		m.quitDialog.Close()
	default:
		return
	}
}

func (m *reviewWatchModel) applyReviewWatchPayload(kind events.EventKind, payload kinds.ReviewWatchPayload) {
	m.mergeOverview(payload)
	line := m.reviewWatchTimelineLine(kind, payload)
	if line != "" {
		m.appendTimeline(line)
	}
	switch kind {
	case events.EventKindReviewWatchStarted:
		m.overview.phase = reviewWatchStatusWatching
	case events.EventKindReviewWatchWaiting:
		m.overview.phase = reviewWatchStatusWaiting
	case events.EventKindReviewWatchRoundFetched:
		m.overview.phase = reviewWatchStatusWatching
	case events.EventKindReviewWatchFixStarted:
		m.overview.phase = reviewWatchStatusFixing
		idx := m.ensureChildTab(payload)
		if idx >= 0 && m.activeTab == 0 {
			m.activeTab = idx + 1
		}
	case events.EventKindReviewWatchFixCompleted:
		m.applyFixCompleted(payload)
	case events.EventKindReviewWatchPushStarted:
		m.overview.phase = reviewWatchStatusPushing
	case events.EventKindReviewWatchPushCompleted:
		m.overview.phase = reviewWatchStatusWatching
	case events.EventKindReviewWatchPushFailed:
		m.overview.phase = reviewWatchStatusFailed
		m.overview.lastError = strings.TrimSpace(payload.Error)
	case events.EventKindReviewWatchClean:
		m.parentRun.Status = remoteRunStatusCompleted
		m.overview.phase = reviewWatchStatusClean
		m.quitDialog.Close()
	case events.EventKindReviewWatchMaxRounds:
		m.parentRun.Status = remoteRunStatusCompleted
		m.overview.phase = reviewWatchStatusDone
		m.quitDialog.Close()
	}
}

func (m *reviewWatchModel) mergeOverview(payload kinds.ReviewWatchPayload) {
	m.mergeOverviewIdentity(payload)
	m.mergeOverviewReview(payload)
	m.mergeOverviewCounts(payload)
	m.mergeOverviewPush(payload)
}

func (m *reviewWatchModel) mergeOverviewIdentity(payload kinds.ReviewWatchPayload) {
	if payload.Provider != "" {
		m.overview.provider = payload.Provider
	}
	if payload.PR != "" {
		m.overview.pr = payload.PR
	}
	if payload.Workflow != "" {
		m.overview.workflow = payload.Workflow
	}
	if payload.Round > 0 {
		m.overview.round = payload.Round
	}
	if payload.HeadSHA != "" {
		m.overview.headSHA = payload.HeadSHA
	}
}

func (m *reviewWatchModel) mergeOverviewReview(payload kinds.ReviewWatchPayload) {
	if payload.ReviewID != "" {
		m.overview.reviewID = payload.ReviewID
	}
	if payload.ReviewState != "" {
		m.overview.reviewState = payload.ReviewState
	}
	if payload.Status != "" {
		m.overview.status = payload.Status
	}
	if payload.Error != "" {
		m.overview.lastError = payload.Error
	}
}

func (m *reviewWatchModel) mergeOverviewPush(payload kinds.ReviewWatchPayload) {
	if payload.Remote != "" {
		m.overview.remote = payload.Remote
	}
	if payload.Branch != "" {
		m.overview.branch = payload.Branch
	}
	m.overview.dirty = payload.Dirty
	if payload.UnpushedCommits > 0 {
		m.overview.unpushedCommits = payload.UnpushedCommits
	}
}

func (m *reviewWatchModel) mergeOverviewCounts(payload kinds.ReviewWatchPayload) {
	if payload.Total > 0 {
		m.overview.total = payload.Total
	}
	if payload.Resolved > 0 {
		m.overview.resolved = payload.Resolved
	}
	if payload.Unresolved > 0 {
		m.overview.unresolved = payload.Unresolved
	}
}

func (m *reviewWatchModel) reviewWatchTimelineLine(
	kind events.EventKind,
	payload kinds.ReviewWatchPayload,
) string {
	round := ""
	if payload.Round > 0 {
		round = fmt.Sprintf(" round=%03d", payload.Round)
	}
	switch kind {
	case events.EventKindReviewWatchStarted:
		return fmt.Sprintf(
			"watch started provider=%s pr=%s head=%s",
			payload.Provider,
			payload.PR,
			shortSHA(payload.HeadSHA),
		)
	case events.EventKindReviewWatchWaiting:
		return fmt.Sprintf(
			"waiting status=%s review=%s head=%s",
			payload.Status,
			payload.ReviewState,
			shortSHA(payload.HeadSHA),
		)
	case events.EventKindReviewWatchRoundFetched:
		return fmt.Sprintf("fetched%s unresolved=%d resolved=%d", round, payload.Unresolved, payload.Resolved)
	case events.EventKindReviewWatchFixStarted:
		return fmt.Sprintf("fix started%s child=%s", round, payload.ChildRunID)
	case events.EventKindReviewWatchFixCompleted:
		line := fmt.Sprintf("fix completed%s child=%s status=%s", round, payload.ChildRunID, payload.Status)
		if payload.Error != "" {
			line += " error=" + payload.Error
		}
		return line
	case events.EventKindReviewWatchPushStarted:
		return fmt.Sprintf("push started%s remote=%s branch=%s", round, payload.Remote, payload.Branch)
	case events.EventKindReviewWatchPushCompleted:
		return fmt.Sprintf("push completed%s remote=%s branch=%s", round, payload.Remote, payload.Branch)
	case events.EventKindReviewWatchPushFailed:
		return fmt.Sprintf("push failed%s error=%s", round, payload.Error)
	case events.EventKindReviewWatchClean:
		return fmt.Sprintf("clean status=%s head=%s", payload.Status, shortSHA(payload.HeadSHA))
	case events.EventKindReviewWatchMaxRounds:
		return fmt.Sprintf("max rounds reached%s head=%s", round, shortSHA(payload.HeadSHA))
	default:
		return ""
	}
}

func (m *reviewWatchModel) appendTimeline(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	m.overview.lines = append(m.overview.lines, line)
	if len(m.overview.lines) > reviewWatchMaxLines {
		m.overview.lines = append([]string(nil), m.overview.lines[len(m.overview.lines)-reviewWatchMaxLines:]...)
	}
}

func (m *reviewWatchModel) ensureChildTab(payload kinds.ReviewWatchPayload) int {
	childRunID := strings.TrimSpace(payload.ChildRunID)
	if childRunID == "" {
		return -1
	}
	if idx := m.findChildByRunID(childRunID); idx >= 0 {
		m.children[idx].status = taskMultiStatusRunning
		return idx
	}
	label := reviewWatchChildLabel(payload)
	m.children = append(m.children, multiRunTab{
		slug:       label,
		status:     taskMultiStatusRunning,
		runID:      childRunID,
		translator: newUIEventTranslator(),
	})
	return len(m.children) - 1
}

func (m *reviewWatchModel) applyFixCompleted(payload kinds.ReviewWatchPayload) {
	m.overview.phase = reviewWatchStatusWatching
	idx := m.ensureChildTab(payload)
	if idx < 0 {
		return
	}
	status := taskMultiStatusFromRunStatus(payload.Status)
	if status == "" {
		status = strings.TrimSpace(payload.Status)
	}
	if status == "" {
		status = taskMultiStatusCompleted
	}
	m.children[idx].status = status
	m.children[idx].terminal = isTerminalTaskMultiStatus(status)
	m.children[idx].errorText = strings.TrimSpace(payload.Error)
}

func reviewWatchChildLabel(payload kinds.ReviewWatchPayload) string {
	if payload.Round > 0 {
		return fmt.Sprintf("round %03d", payload.Round)
	}
	return firstNonEmpty(strings.TrimSpace(payload.ChildRunID), "fix run")
}

func (m *reviewWatchModel) handleChildBootstrap(msg multiRunChildBootstrapMsg) {
	idx := m.findChildByRunID(msg.RunID)
	if idx < 0 {
		return
	}
	m.children[idx].applyChildSnapshot(msg.Snapshot, m.cfg, m.childWidth(), m.childHeight())
	if status := taskMultiStatusFromRunStatus(msg.Snapshot.Run.Status); status != "" {
		m.children[idx].status = status
		m.children[idx].terminal = isTerminalTaskMultiStatus(status)
	}
}

func (m *reviewWatchModel) handleChildEvent(msg multiRunChildEventMsg) tea.Cmd {
	idx := m.findChildByRunID(msg.RunID)
	if idx < 0 {
		return nil
	}
	tab := &m.children[idx]
	if tab.child == nil {
		tab.child = newPlaceholderChildModel(tab.slug, m.cfg, m.childWidth(), m.childHeight())
	}
	if tab.translator == nil {
		tab.translator = newUIEventTranslator()
	}
	var cmd tea.Cmd
	for _, uiMsg := range tab.translator.translateMessages(msg.Event) {
		if nextCmd := applyChildUIMsg(tab, uiMsg); nextCmd != nil && idx+1 == m.activeTab {
			cmd = nextCmd
		}
	}
	if status := taskMultiStatusFromChildRunEvent(msg.Event.Kind); status != "" {
		tab.status = status
		tab.terminal = isTerminalTaskMultiStatus(status)
	}
	return cmd
}

func (m *reviewWatchModel) View() tea.View {
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

func (m *reviewWatchModel) renderRoot(content string) tea.View {
	v := tea.NewView(rootScreenStyle(m.width, m.height).Render(content))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *reviewWatchModel) renderTabs() string {
	bg := colorBgBase
	chunks := make([]string, 0, len(m.children)+1)
	watchLabel := fmt.Sprintf("1 Watch %s", strings.ToUpper(firstNonEmpty(m.overview.phase, reviewWatchStatusWatching)))
	watchStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorBgSurface).
		Foreground(reviewWatchStatusColor(m.overview.phase))
	if m.activeTab == 0 {
		watchStyle = watchStyle.Bold(true).Background(colorAccent).Foreground(colorBgBase)
	}
	chunks = append(chunks, watchStyle.Render(truncateString(watchLabel, 32)))
	for idx := range m.children {
		tab := m.children[idx]
		label := fmt.Sprintf(
			"%d %s %s",
			idx+2,
			firstNonEmpty(tab.slug, tab.runID, "fix"),
			strings.ToUpper(tabStatus(&tab)),
		)
		style := lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorBgSurface).
			Foreground(multiRunStatusColor(tabStatus(&tab)))
		if idx+1 == m.activeTab {
			style = style.Bold(true).Background(colorAccent).Foreground(colorBgBase)
		}
		chunks = append(chunks, style.Render(truncateString(label, 32)))
	}
	left := renderGap(bg, 1) + strings.Join(chunks, renderGap(bg, 1))
	hint := renderKeycap("left/right", bg) + renderGap(bg, 1) + renderStyledOnBackground(styleMutedText, bg, "TABS")
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(hint)-1, 1)
	line := renderOwnedLineKnownOwned(m.width, bg, left+renderGap(bg, gap)+hint)
	separator := renderOwnedLineKnownOwned(
		m.width,
		bg,
		renderStyledOnBackground(styleSeparator, bg, strings.Repeat("─", m.width)),
	)
	return line + "\n" + separator
}

func (m *reviewWatchModel) renderActiveTabContent() string {
	if m.activeTab == 0 {
		return m.renderWatchTabContent()
	}
	idx := m.activeTab - 1
	if idx < 0 || idx >= len(m.children) {
		return m.renderWatchTabContent()
	}
	tab := &m.children[idx]
	if tab.child == nil {
		return m.renderQueuedChildContent(tab)
	}
	m.resizeChild(tab.child)
	return tab.child.View().Content
}

func (m *reviewWatchModel) renderWatchTabContent() string {
	width := max(m.width, 1)
	height := max(m.childHeight(), 1)
	panelWidth := max(width-4, 20)
	innerStyle := techPanelStyle(panelWidth, reviewWatchStatusColor(m.overview.phase)).Padding(1, 2)
	innerWidth := max(panelWidth-innerStyle.GetHorizontalFrameSize(), 1)
	bg := colorBgSurface
	lines := []string{
		renderOwnedLineKnownOwned(innerWidth, bg, renderTechLabel("review.watch", bg)),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(innerWidth, bg, renderStyledOnBackground(
			styleBodyText,
			bg,
			truncateString(m.watchTitle(innerWidth), innerWidth),
		)),
		renderOwnedLineKnownOwned(innerWidth, bg, renderStyledOnBackground(
			styleMutedText,
			bg,
			truncateString(m.watchMetaLine(), innerWidth),
		)),
		renderOwnedLineKnownOwned(innerWidth, bg, renderStyledOnBackground(
			styleMutedText,
			bg,
			truncateString(m.watchReviewLine(), innerWidth),
		)),
	}
	if pushLine := m.watchPushLine(); pushLine != "" {
		lines = append(lines, renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(styleMutedText, bg, truncateString(pushLine, innerWidth)),
		))
	}
	if errText := strings.TrimSpace(m.overview.lastError); errText != "" {
		lines = append(lines, renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(styleMutedText.Foreground(colorError), bg, truncateString(errText, innerWidth)),
		))
	}
	lines = append(lines, renderOwnedLineKnownOwned(innerWidth, bg, ""))
	lines = append(lines, m.renderTimelineLines(innerWidth, max(height-len(lines)-3, 1))...)
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

func (m *reviewWatchModel) watchTitle(width int) string {
	workflow := firstNonEmpty(m.overview.workflow, m.parentRun.WorkflowSlug, "review workflow")
	return truncateString(fmt.Sprintf("%s / PR %s", workflow, firstNonEmpty(m.overview.pr, "unknown")), width)
}

func (m *reviewWatchModel) watchMetaLine() string {
	parts := []string{
		"phase=" + firstNonEmpty(m.overview.phase, reviewWatchStatusWatching),
		"status=" + firstNonEmpty(m.overview.status, m.parentRun.Status, "pending"),
	}
	if m.overview.provider != "" {
		parts = append(parts, "provider="+m.overview.provider)
	}
	if m.overview.headSHA != "" {
		parts = append(parts, "head="+shortSHA(m.overview.headSHA))
	}
	if m.overview.round > 0 {
		parts = append(parts, fmt.Sprintf("round=%03d", m.overview.round))
	}
	return strings.Join(parts, "  ")
}

func (m *reviewWatchModel) watchReviewLine() string {
	parts := []string{
		fmt.Sprintf(
			"issues total=%d unresolved=%d resolved=%d",
			m.overview.total,
			m.overview.unresolved,
			m.overview.resolved,
		),
	}
	if m.overview.reviewState != "" {
		parts = append(parts, "review="+m.overview.reviewState)
	}
	if m.overview.reviewID != "" {
		parts = append(parts, "review_id="+m.overview.reviewID)
	}
	return strings.Join(parts, "  ")
}

func (m *reviewWatchModel) watchPushLine() string {
	parts := make([]string, 0, 4)
	if m.overview.remote != "" {
		parts = append(parts, "remote="+m.overview.remote)
	}
	if m.overview.branch != "" {
		parts = append(parts, "branch="+m.overview.branch)
	}
	if m.overview.unpushedCommits > 0 {
		parts = append(parts, fmt.Sprintf("unpushed=%d", m.overview.unpushedCommits))
	}
	if m.overview.dirty {
		parts = append(parts, "dirty=true")
	}
	return strings.Join(parts, "  ")
}

func (m *reviewWatchModel) renderTimelineLines(width int, maxLines int) []string {
	bg := colorBgSurface
	if len(m.overview.lines) == 0 {
		return []string{
			renderOwnedLineKnownOwned(width, bg, renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Waiting for review-watch events...", width),
			)),
		}
	}
	start := max(len(m.overview.lines)-maxLines, 0)
	lines := make([]string, 0, len(m.overview.lines)-start)
	for _, line := range m.overview.lines[start:] {
		lines = append(lines, renderOwnedLineKnownOwned(
			width,
			bg,
			renderStyledOnBackground(styleMutedText, bg, truncateString(line, width)),
		))
	}
	return lines
}

func (m *reviewWatchModel) renderQueuedChildContent(tab *multiRunTab) string {
	width := max(m.width, 1)
	height := max(m.childHeight(), 1)
	panelWidth := max(width-4, 20)
	innerStyle := techPanelStyle(panelWidth, multiRunStatusColor(tabStatus(tab))).Padding(1, 2)
	innerWidth := max(panelWidth-innerStyle.GetHorizontalFrameSize(), 1)
	name := "fix run"
	status := taskMultiStatusQueued
	if tab != nil {
		name = firstNonEmpty(tab.slug, tab.runID, name)
		status = tabStatus(tab)
	}
	bg := colorBgSurface
	lines := []string{
		renderOwnedLineKnownOwned(innerWidth, bg, renderTechLabel("review.fix."+status, bg)),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(innerWidth, bg, renderStyledOnBackground(
			styleBodyText,
			bg,
			truncateString(name, innerWidth),
		)),
		renderOwnedLineKnownOwned(innerWidth, bg, renderStyledOnBackground(
			styleMutedText,
			bg,
			truncateString("Fix run has not emitted a snapshot yet.", innerWidth),
		)),
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

func (m *reviewWatchModel) renderQuitDialogView() tea.View {
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

func (m *reviewWatchModel) renderQuitDialogPanel() string {
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
				truncateString("Leave Active Watch?", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(innerWidth, bg, ""),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleBodyText,
				bg,
				truncateString("This review watch is still active.", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Close the TUI and keep watching in the daemon.", innerWidth),
			),
		),
		renderOwnedLineKnownOwned(
			innerWidth,
			bg,
			renderStyledOnBackground(
				styleMutedText,
				bg,
				truncateString("Choose Stop Run to cancel this watch and its active fix run.", innerWidth),
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

func (m *reviewWatchModel) renderQuitDialogActions(width int, bg color.Color) string {
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

func (m *reviewWatchModel) renderQuitDialogAction(label string, action quitDialogAction) string {
	baseStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if m.quitDialog.Selected == action {
		return baseStyle.Foreground(colorBgSurface).Background(colorAccent).Render(label)
	}
	return baseStyle.Foreground(colorFgBright).Background(colorBgBase).Render(label)
}

func (m *reviewWatchModel) activeChild() *uiModel {
	idx := m.activeTab - 1
	if idx < 0 || idx >= len(m.children) {
		return nil
	}
	return m.children[idx].child
}

func (m *reviewWatchModel) moveActiveTab(delta int) {
	totalTabs := len(m.children) + 1
	if totalTabs <= 0 {
		return
	}
	if child := m.activeChild(); child != nil {
		child.persistSelectedViewportState()
	}
	m.activeTab = (m.activeTab + delta + totalTabs) % totalTabs
	if child := m.activeChild(); child != nil {
		m.resizeChild(child)
		child.refreshViewportContent()
	}
}

func (m *reviewWatchModel) findChildByRunID(runID string) int {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return -1
	}
	for idx := range m.children {
		if strings.TrimSpace(m.children[idx].runID) == trimmed {
			return idx
		}
	}
	return -1
}

func (m *reviewWatchModel) resizeChild(child *uiModel) {
	if child == nil {
		return
	}
	child.handleWindowSize(tea.WindowSizeMsg{
		Width:  m.childWidth(),
		Height: m.childHeight(),
	})
}

func (m *reviewWatchModel) childWidth() int {
	return max(m.width, 1)
}

func (m *reviewWatchModel) childHeight() int {
	return max(m.height-reviewWatchTabsHeight, 1)
}

func reviewWatchStatusColor(status string) color.Color {
	switch strings.TrimSpace(status) {
	case reviewWatchStatusFixing, reviewWatchStatusPushing:
		return colorAccentAlt
	case reviewWatchStatusClean, reviewWatchStatusDone:
		return colorSuccess
	case reviewWatchStatusFailed:
		return colorError
	case reviewWatchStatusCanceled:
		return colorWarning
	default:
		return colorMuted
	}
}

func shortSHA(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 12 {
		return trimmed
	}
	return trimmed[:12]
}

func decodeReviewWatchPayload(ev events.Event) (kinds.ReviewWatchPayload, bool) {
	return decodeUIEventPayload[kinds.ReviewWatchPayload](ev)
}
