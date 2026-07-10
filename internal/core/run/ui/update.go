package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

const (
	keyPageUp   = "pgup"
	keyPageDown = "pgdown"
	keyHome     = "home"
	keyEnd      = "end"
	keyCtrlC    = "ctrl+c"
	keyEscape   = "esc"
	keyTab      = "tab"
	keyShiftTab = "shift+tab"
	keyEnter    = "enter"
	keyLeft     = "left"
	keyRight    = "right"
)

var setSidebarViewportContent = func(vp *viewport.Model, content string) {
	vp.SetContent(content)
}

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyPressMsg:
		return m, m.handleKey(v)
	case tea.MouseWheelMsg:
		m.handleMouseWheel(v)
		return m, nil
	case tea.WindowSizeMsg:
		m.handleWindowSize(v)
		return m, nil
	case clockTickMsg:
		return m, m.handleClockTick(v)
	case spinnerTickMsg:
		return m, m.handleSpinnerTick(v)
	case dispatchBatchMsg:
		return m, m.handleDispatchBatch(v)
	case drainMsg:
		return m, nil
	default:
		if cmd, ok := m.dispatchSingleUIMsg(msg); ok {
			return m, cmd
		}
		return m, nil
	}
}

func (m *uiModel) dispatchSingleUIMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch v := msg.(type) {
	case jobQueuedMsg:
		return m.applyUIMsg(v), true
	case jobStartedMsg:
		return m.applyUIMsg(v), true
	case jobRetryMsg:
		return m.applyUIMsg(v), true
	case jobPausingMsg:
		return m.applyUIMsg(v), true
	case jobPausedMsg:
		return m.applyUIMsg(v), true
	case jobResumedMsg:
		return m.applyUIMsg(v), true
	case jobFinishedMsg:
		return m.applyUIMsg(v), true
	case jobUpdateMsg:
		return m.applyUIMsg(v), true
	case usageUpdateMsg:
		return m.applyUIMsg(v), true
	case runStatusMsg:
		return m.applyUIMsg(v), true
	case shutdownStatusMsg:
		return m.applyUIMsg(v), true
	case remoteConnectionStatusMsg:
		return m.applyUIMsg(v), true
	case jobFailureMsg:
		return m.applyUIMsg(v), true
	case jobControlResultMsg:
		return m.applyUIMsg(v), true
	default:
		return m.applyParallelUIMsg(v)
	}
}

func (m *uiModel) handleDispatchBatch(v dispatchBatchMsg) tea.Cmd {
	if len(v.msgs) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(v.msgs))
	for _, msg := range v.msgs {
		if cmd := m.applyUIMsg(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *uiModel) applyUIMsg(msg uiMsg) tea.Cmd {
	switch value := msg.(type) {
	case jobQueuedMsg:
		return m.handleJobQueued(&value)
	case jobStartedMsg:
		return m.handleJobStarted(value)
	case jobRetryMsg:
		return m.handleJobRetry(value)
	case jobPausingMsg:
		return m.handleJobPausing(value)
	case jobPausedMsg:
		return m.handleJobPaused(value)
	case jobResumedMsg:
		return m.handleJobResumed(value)
	case jobFinishedMsg:
		return m.handleJobFinished(value)
	case jobUpdateMsg:
		return m.handleJobUpdate(value)
	case usageUpdateMsg:
		return m.handleUsageUpdate(value)
	case runStatusMsg:
		return m.handleRunStatus(value)
	case shutdownStatusMsg:
		return m.handleShutdownStatus(value)
	case jobControlResultMsg:
		return m.handleJobControlResult(value)
	case dispatchBatchMsg:
		return m.handleDispatchBatch(value)
	default:
		if m.applyPassiveUIMsg(value) {
			return nil
		}
		cmd, _ := m.applyParallelUIMsg(value)
		return cmd
	}
}

func (m *uiModel) applyPassiveUIMsg(msg uiMsg) bool {
	switch value := msg.(type) {
	case remoteConnectionStatusMsg:
		m.remoteReconnecting = value.Reconnecting
		return true
	case jobFailureMsg:
		m.failures = append(m.failures, value.Failure)
		return true
	default:
		return false
	}
}

// applyParallelUIMsg routes task.parallel.* translated messages to their handlers,
// returning ok=false for any non-parallel message. Kept separate from applyUIMsg
// to bound that switch's cyclomatic complexity.
func (m *uiModel) applyParallelUIMsg(msg uiMsg) (tea.Cmd, bool) {
	switch value := msg.(type) {
	case parallelPlanStartedMsg:
		m.handleParallelPlanStarted(value)
	case parallelWaveStartedMsg:
		m.handleParallelWaveStarted(value)
	case parallelTaskStartedMsg:
		m.handleParallelTaskStarted(value)
	case parallelTaskCompletedMsg:
		m.handleParallelTaskCompleted(value)
	case parallelPhaseChangedMsg:
		m.handleParallelPhaseChanged(value)
	case parallelMergeStartedMsg:
		m.handleParallelMergeStarted(value)
	case parallelConflictMsg:
		m.handleParallelConflict(value)
	case parallelMergedMsg:
		m.handleParallelMerged(value)
	case parallelWaveCompletedMsg:
		m.handleParallelWaveCompleted(value)
	case parallelRolledBackMsg:
		m.handleParallelRolledBack(value)
	case parallelFailedMsg:
		m.handleParallelFailed(value)
	case parallelSettledMsg:
		m.handleParallelSettled(value)
	default:
		return nil, false
	}
	return m.afterParallelUpdate(), true
}

// afterParallelUpdate refreshes derived viewport content after a parallel-state
// mutation and keeps the wave-grouped sidebar spinners animating.
func (m *uiModel) afterParallelUpdate() tea.Cmd {
	m.refreshViewportContent()
	return m.ensureSpinnerTick()
}

func (m *uiModel) handleRunStatus(v runStatusMsg) tea.Cmd {
	m.runStatus = strings.TrimSpace(v.Status)
	return nil
}

func (m *uiModel) handleKey(v tea.KeyPressMsg) tea.Cmd {
	if m.quitDialog.Active {
		return m.handleQuitDialogKey(v)
	}
	if cmd, handled := m.handleFocusedComposerKey(v); handled {
		return cmd
	}
	key := v.String()
	switch key {
	case keyCtrlC, "q":
		return m.handleQuitKey()
	case "s":
		return m.handleSummaryToggle()
	case keyEscape:
		return m.handleEscape()
	case keyTab:
		return m.cycleFocusedPane(1)
	case keyShiftTab:
		return m.cycleFocusedPane(-1)
	case keyEnter:
		m.toggleSelectedEntryExpansion()
		return nil
	case "p":
		return m.requestPauseSelectedJob()
	case "up", "k":
		m.moveFocusedSelection(-1)
		return nil
	case "down", "j":
		m.moveFocusedSelection(1)
		return nil
	case keyPageUp, keyPageDown, keyHome, keyEnd:
		m.scrollFocusedPane(key)
		return nil
	default:
		return nil
	}
}

func (m *uiModel) handleFocusedComposerKey(v tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.focusedPane != uiPaneComposer {
		return nil, false
	}
	switch v.String() {
	case keyCtrlC:
		return m.handleQuitKey(), true
	case keyTab:
		return m.cycleFocusedPane(1), true
	case keyShiftTab:
		return m.cycleFocusedPane(-1), true
	default:
		return m.handleComposerKey(v), true
	}
}

func (m *uiModel) handleComposerKey(v tea.KeyPressMsg) tea.Cmd {
	switch v.String() {
	case keyEscape:
		m.composer.Blur()
		m.focusedPane = uiPaneTimeline
		return nil
	case keyEnter:
		return m.submitComposerMessage()
	default:
		if !m.composerEnabled(m.currentJob()) {
			return nil
		}
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(v)
		return cmd
	}
}

func (m *uiModel) requestPauseSelectedJob() tea.Cmd {
	job := m.currentJob()
	if job == nil || !m.jobCanPause(job) || m.onJobControl == nil {
		return nil
	}
	index := m.selectedJob
	jobID := job.safeName
	job.state = jobPausing
	job.sidebarCacheValid = false
	m.sidebarDirty = true
	m.composerError = ""
	m.refreshViewportContent()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.jobControlContext(), 30*time.Second)
		defer cancel()
		resp, err := m.onJobControl(ctx, uiJobControlRequest{
			Action: uiJobControlPause,
			RunID:  m.runID(),
			JobID:  jobID,
		})
		return jobControlResultMsg{Index: index, Action: uiJobControlPause, Response: resp, Err: err}
	}
}

func (m *uiModel) submitComposerMessage() tea.Cmd {
	job := m.currentJob()
	if job == nil || !m.composerEnabled(job) || m.onJobControl == nil {
		return nil
	}
	message := strings.TrimSpace(m.composer.Value())
	if message == "" {
		m.composerError = "Message required"
		return nil
	}
	index := m.selectedJob
	jobID := job.safeName
	m.composerBusy = true
	m.composerError = ""
	m.composer.Blur()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.jobControlContext(), 30*time.Second)
		defer cancel()
		resp, err := m.onJobControl(ctx, uiJobControlRequest{
			Action:  uiJobControlMessage,
			RunID:   m.runID(),
			JobID:   jobID,
			Message: message,
		})
		return jobControlResultMsg{Index: index, Action: uiJobControlMessage, Response: resp, Err: err}
	}
}

func (m *uiModel) jobControlContext() context.Context {
	if m == nil || m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func (m *uiModel) runID() string {
	if m == nil || m.cfg == nil {
		return ""
	}
	return strings.TrimSpace(m.cfg.RunID)
}

func (m *uiModel) jobCanPause(job *uiJob) bool {
	if job == nil || m.shutdown.Active() {
		return false
	}
	switch job.state {
	case jobRunning, jobRetrying:
		return true
	default:
		return false
	}
}

func (m *uiModel) composerEnabled(job *uiJob) bool {
	return job != nil && job.state == jobPaused && !m.composerBusy
}

func (m *uiModel) handleQuitKey() tea.Cmd {
	if m.cfg != nil && m.cfg.DetachOnly {
		return tea.Quit
	}

	if m.isRunComplete() {
		return tea.Quit
	}

	if !m.shutdown.Active() {
		m.openQuitDialog()
		return nil
	}

	return m.requestRunStopFromQuit()
}

func (m *uiModel) requestRunStopFromQuit() tea.Cmd {
	req, ok := m.nextQuitRequest()
	if !ok {
		return nil
	}
	if m.currentView == uiViewJobs {
		m.refreshSidebarContent()
	}
	if m.onQuit == nil {
		return nil
	}
	// Run quit callbacks as a Bubble Tea command so handlers that close the
	// session do not synchronously call Program.Quit from inside Update.
	return func() tea.Msg {
		m.onQuit(req)
		return drainMsg{}
	}
}

func (m *uiModel) handleQuitDialogKey(v tea.KeyPressMsg) tea.Cmd {
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
		m.closeQuitDialog()
		return nil
	default:
		return nil
	}
}

func (m *uiModel) confirmQuitDialog() tea.Cmd {
	selected := m.quitDialog.Selected
	m.closeQuitDialog()
	switch selected {
	case quitDialogActionClose:
		return tea.Quit
	case quitDialogActionStop:
		return m.requestRunStopFromQuit()
	default:
		return nil
	}
}

func (m *uiModel) openQuitDialog() {
	m.quitDialog.Open()
}

func (m *uiModel) closeQuitDialog() {
	m.quitDialog.Close()
}

func (m *uiModel) nextQuitRequest() (uiQuitRequest, bool) {
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

func (m *uiModel) handleSummaryToggle() tea.Cmd {
	if m.currentView == uiViewSummary {
		m.showJobsView()
		return nil
	}
	if m.isRunComplete() {
		m.showSummaryView()
	}
	return nil
}

func (m *uiModel) handleEscape() tea.Cmd {
	if m.currentView == uiViewSummary {
		m.showJobsView()
		return nil
	}
	return nil
}

func (m *uiModel) showJobsView() {
	m.currentView = uiViewJobs
	m.refreshViewportContent()
}

func (m *uiModel) showSummaryView() {
	if !m.isRunComplete() {
		return
	}
	m.currentView = uiViewSummary
}

func (m *uiModel) cycleFocusedPane(direction int) tea.Cmd {
	if m.currentView != uiViewJobs {
		return nil
	}
	order := m.visiblePanes()
	if len(order) == 0 {
		return nil
	}

	currentIdx := 0
	for i, pane := range order {
		if pane == m.focusedPane {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + direction + len(order)) % len(order)
	nextPane := order[nextIdx]
	m.focusedPane = nextPane
	if nextPane == uiPaneComposer && m.composerEnabled(m.currentJob()) {
		return m.composer.Focus()
	}
	m.composer.Blur()
	return nil
}

func (m *uiModel) visiblePanes() []uiPane {
	if m.layoutMode == uiLayoutResizeBlocked {
		return nil
	}
	if m.composerEnabled(m.currentJob()) || m.focusedPane == uiPaneComposer {
		return []uiPane{uiPaneJobs, uiPaneTimeline, uiPaneComposer}
	}
	return []uiPane{uiPaneJobs, uiPaneTimeline}
}

func (m *uiModel) moveFocusedSelection(delta int) {
	if m.currentView != uiViewJobs {
		return
	}
	switch m.focusedPane {
	case uiPaneJobs:
		m.moveSelectedJob(delta)
	case uiPaneTimeline:
		m.moveSelectedEntry(delta)
	}
}

func (m *uiModel) moveSelectedJob(delta int) {
	if len(m.jobs) == 0 {
		return
	}
	m.persistSelectedViewportState()
	order := m.visualJobOrder()
	position := 0
	for index, jobIndex := range order {
		if jobIndex == m.selectedJob {
			position = index
			break
		}
	}
	next := position + delta
	if next < 0 {
		next = 0
	}
	if next >= len(order) {
		next = len(order) - 1
	}
	m.selectedJob = order[next]
	m.sidebarDirty = true
	m.refreshViewportContent()
}

func (m *uiModel) moveSelectedEntry(delta int) {
	job := m.currentJob()
	if job == nil || len(job.snapshot.Entries) == 0 {
		return
	}
	if job.selectedEntry < 0 {
		job.selectedEntry = 0
	}
	job.selectedEntry += delta
	if job.selectedEntry < 0 {
		job.selectedEntry = 0
	}
	if job.selectedEntry >= len(job.snapshot.Entries) {
		job.selectedEntry = len(job.snapshot.Entries) - 1
	}
	job.transcriptFollowTail = job.selectedEntry == len(job.snapshot.Entries)-1
	m.refreshViewportContent()
}

func (m *uiModel) toggleSelectedEntryExpansion() {
	if m.currentView != uiViewJobs || m.focusedPane != uiPaneTimeline {
		return
	}
	job := m.currentJob()
	if job == nil || len(job.snapshot.Entries) == 0 {
		return
	}
	entry := job.snapshot.Entries[job.selectedEntry]
	if job.expandedEntryIDs == nil {
		job.expandedEntryIDs = make(map[string]bool)
	}
	job.expandedEntryIDs[entry.ID] = !m.isEntryExpanded(job, entry)
	job.expansionRevision++
	m.refreshViewportContent()
}

func (m *uiModel) scrollFocusedPane(key string) {
	if m.currentView != uiViewJobs {
		return
	}
	switch m.focusedPane {
	case uiPaneJobs:
		scrollSidebarViewport(&m.sidebarViewport, key)
	case uiPaneTimeline:
		scrollSidebarViewport(&m.transcriptViewport, key)
		if job := m.currentJob(); job != nil {
			job.transcriptFollowTail = m.transcriptViewport.AtBottom()
		}
	}
	m.persistSelectedViewportState()
}

type scrollableViewport interface {
	PageUp()
	PageDown()
	GotoTop() []string
	GotoBottom() []string
}

func scrollSidebarViewport(viewport scrollableViewport, key string) {
	switch key {
	case keyPageUp:
		viewport.PageUp()
	case keyPageDown:
		viewport.PageDown()
	case keyHome:
		viewport.GotoTop()
	case keyEnd:
		viewport.GotoBottom()
	}
}

func (m *uiModel) handleMouseWheel(v tea.MouseWheelMsg) {
	if m.currentView != uiViewJobs {
		return
	}
	switch m.focusedPane {
	case uiPaneJobs:
		updated, _ := m.sidebarViewport.Update(v)
		m.sidebarViewport = updated
	case uiPaneTimeline:
		updated, _ := m.transcriptViewport.Update(v)
		m.transcriptViewport = updated
		if job := m.currentJob(); job != nil {
			job.transcriptFollowTail = m.transcriptViewport.AtBottom()
		}
	}
	m.persistSelectedViewportState()
}

func (m *uiModel) handleWindowSize(v tea.WindowSizeMsg) {
	m.width = v.Width
	m.height = v.Height
	layout := m.computeLayout(v.Width, v.Height)
	m.layoutMode = layout.mode
	m.sidebarWidth = layout.sidebarWidth
	m.timelineWidth = layout.timelineWidth
	m.contentHeight = layout.contentHeight
	m.configureViewports(layout)
	m.sidebarDirty = true
	m.refreshViewportContent()
}

func (m *uiModel) refreshViewportContent() {
	if len(m.jobs) == 0 {
		if m.sidebarContent != "" {
			m.applySidebarViewportContent("")
			m.sidebarContent = ""
		}
		m.timelineMounted = invalidTimelineMountState()
		m.sidebarDirty = false
		return
	}
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		m.selectedJob = 0
		m.sidebarDirty = true
	}

	if m.sidebarDirty {
		m.refreshSidebarContent()
	}
	job := &m.jobs[m.selectedJob]
	m.syncSelectedEntry(job)
}

func (m *uiModel) refreshSidebarContent() {
	width := m.sidebarViewport.Width()
	var content string
	var lineOffset int
	if m.parallel.grouped() {
		// Parallel runs group cards under wave headers; everything else renders as
		// the flat single-run/multi-run card stack untouched.
		content, lineOffset = m.renderWaveGroupedSidebar(width)
	} else {
		items := make([]string, 0, len(m.jobs))
		for i := range m.jobs {
			items = append(items, m.renderSidebarItem(i, &m.jobs[i], i == m.selectedJob))
		}
		// Each item is a bordered card; adjacent cards share one separator border row
		// so there is no blank gap or duplicated top/bottom border in the viewport.
		content = renderSidebarStack(items, width)
		lineOffset = m.selectedJob * sidebarRowStride
	}
	if content != m.sidebarContent {
		m.applySidebarViewportContent(content)
		m.sidebarContent = content
	}
	m.sidebarDirty = false

	sidebarOffset := m.sidebarViewport.YOffset()
	sidebarHeight := m.sidebarViewport.Height()
	if lineOffset > sidebarOffset+sidebarHeight-sidebarRowLines {
		m.sidebarViewport.SetYOffset(lineOffset - sidebarHeight + sidebarRowLines)
	} else if lineOffset < sidebarOffset {
		m.sidebarViewport.SetYOffset(lineOffset)
	}
}

func (m *uiModel) isRunComplete() bool {
	return m.completed+m.failed >= m.total
}

func (m *uiModel) currentJob() *uiJob {
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		return nil
	}
	return &m.jobs[m.selectedJob]
}

func (m *uiModel) persistSelectedViewportState() {
	job := m.currentJob()
	if job == nil {
		return
	}
	job.transcriptYOffset = m.transcriptViewport.YOffset()
	job.transcriptXOffset = m.transcriptViewport.XOffset()
	job.transcriptFollowTail = m.transcriptViewport.AtBottom()
}

func (m *uiModel) selectNextRunningJob() {
	for i := range m.jobs {
		if m.jobs[i].state == jobRunning || m.jobs[i].state == jobRetrying {
			m.selectedJob = i
			return
		}
	}
	for i := range m.jobs {
		if m.jobs[i].state == jobPending {
			m.selectedJob = i
			return
		}
	}
}

func (m *uiModel) handleClockTick(v clockTickMsg) tea.Cmd {
	if !v.at.IsZero() {
		m.now = v.at
	}
	if m.currentView == uiViewJobs && len(m.jobs) > 0 &&
		(m.sidebarDirty || m.sidebarNeedsClockRefresh()) {
		m.refreshSidebarContent()
	}
	return m.clockTick()
}

func (m *uiModel) handleSpinnerTick(v spinnerTickMsg) tea.Cmd {
	if !m.hasActiveJobs() {
		m.spinnerRunning = false
		return nil
	}
	if !v.at.IsZero() && v.at.After(m.now) {
		m.now = v.at
	}
	m.frame++
	if m.currentView == uiViewJobs && len(m.jobs) > 0 &&
		(m.sidebarDirty || m.sidebarNeedsActiveRefresh()) {
		m.refreshSidebarContent()
	}
	return m.spinnerTick()
}

func (m *uiModel) ensureSpinnerTick() tea.Cmd {
	if m.spinnerRunning || !m.hasActiveJobs() {
		return nil
	}
	m.spinnerRunning = true
	return m.spinnerTick()
}

func (m *uiModel) handleJobQueued(v *jobQueuedMsg) tea.Cmd {
	existing, _ := m.ensureJobSlot(v.Index)
	m.jobs[v.Index] = mergeQueuedJobState(existing, uiJob{
		codeFile:             v.CodeFile,
		codeFiles:            v.CodeFiles,
		issues:               v.Issues,
		taskNumber:           queuedTaskNumber(v),
		taskTitle:            v.TaskTitle,
		taskType:             v.TaskType,
		safeName:             firstNonEmpty(v.SafeName, placeholderJobSafeName(v.Index)),
		ide:                  v.IDE,
		model:                v.Model,
		reasoningEffort:      v.ReasoningEffort,
		outLog:               v.OutLog,
		errLog:               v.ErrLog,
		outBuffer:            v.OutBuffer,
		errBuffer:            v.ErrBuffer,
		state:                jobPending,
		selectedEntry:        -1,
		expandedEntryIDs:     make(map[string]bool),
		transcriptFollowTail: true,
	})
	m.sidebarDirty = true
	m.refreshViewportContent()
	return nil
}

func queuedTaskNumber(v *jobQueuedMsg) int {
	if v == nil {
		return 0
	}
	if v.TaskNumber > 0 {
		return v.TaskNumber
	}
	if strings.TrimSpace(v.TaskTitle) == "" && strings.TrimSpace(v.TaskType) == "" {
		return 0
	}
	if number := tasks.ExtractTaskIdentityNumber(v.CodeFile); number > 0 {
		return number
	}
	for _, codeFile := range v.CodeFiles {
		if number := tasks.ExtractTaskIdentityNumber(codeFile); number > 0 {
			return number
		}
	}
	return 0
}

func (m *uiModel) handleJobStarted(v jobStartedMsg) tea.Cmd {
	startedAt := time.Now()
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		m.persistSelectedViewportState()
		job.state = jobRunning
		job.attempt = max(v.Attempt, 1)
		job.maxAttempts = max(v.MaxAttempts, job.attempt)
		if strings.TrimSpace(v.IDE) != "" {
			job.ide = v.IDE
		}
		if strings.TrimSpace(v.Model) != "" {
			job.model = v.Model
		}
		if strings.TrimSpace(v.ReasoningEffort) != "" {
			job.reasoningEffort = v.ReasoningEffort
		}
		job.retrying = false
		job.retryReason = ""
		if job.startedAt.IsZero() {
			job.startedAt = startedAt
			job.duration = 0
		}
		if startedAt.After(m.now) {
			m.now = startedAt
		}
		m.selectedJob = v.Index
		m.sidebarDirty = true
	}
	m.refreshViewportContent()
	return m.ensureSpinnerTick()
}

func (m *uiModel) handleJobRetry(v jobRetryMsg) tea.Cmd {
	retryAt := time.Now()
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		m.persistSelectedViewportState()
		job.state = jobRetrying
		job.attempt = max(v.Attempt, 1)
		job.maxAttempts = max(v.MaxAttempts, job.attempt)
		job.retrying = true
		job.retryReason = v.Reason
		if retryAt.After(m.now) {
			m.now = retryAt
		}
		m.selectedJob = v.Index
		m.sidebarDirty = true
	}
	m.refreshViewportContent()
	return nil
}

func (m *uiModel) handleJobPausing(v jobPausingMsg) tea.Cmd {
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		m.persistSelectedViewportState()
		job.state = jobPausing
		job.retrying = false
		job.retryReason = ""
		m.selectedJob = v.Index
		m.sidebarDirty = true
	}
	m.refreshViewportContent()
	return m.ensureSpinnerTick()
}

func (m *uiModel) handleJobPaused(v jobPausedMsg) tea.Cmd {
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		m.persistSelectedViewportState()
		job.state = jobPaused
		job.retrying = false
		job.retryReason = ""
		m.selectedJob = v.Index
		m.focusedPane = uiPaneComposer
		m.composerBusy = false
		m.composerError = ""
		m.sidebarDirty = true
	}
	m.refreshViewportContent()
	return m.composer.Focus()
}

func (m *uiModel) handleJobResumed(v jobResumedMsg) tea.Cmd {
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		job.state = jobRunning
		job.retrying = false
		job.retryReason = ""
		m.selectedJob = v.Index
		m.focusedPane = uiPaneTimeline
		m.composerBusy = false
		m.composerError = ""
		m.sidebarDirty = true
	}
	m.refreshViewportContent()
	return m.ensureSpinnerTick()
}

func (m *uiModel) handleJobControlResult(v jobControlResultMsg) tea.Cmd {
	if v.Err != nil {
		m.composerBusy = false
		m.composerError = v.Err.Error()
		if job, _ := m.ensureJobSlot(v.Index); job != nil && v.Action == uiJobControlPause && job.state == jobPausing {
			job.state = jobRunning
			m.sidebarDirty = true
		}
		if m.focusedPane == uiPaneComposer {
			return m.composer.Focus()
		}
		m.refreshViewportContent()
		return nil
	}
	switch v.Response.Status {
	case model.JobControlStatusPausing:
		return m.handleJobPausing(jobPausingMsg{Index: v.Index})
	case model.JobControlStatusPaused:
		return m.handleJobPaused(jobPausedMsg{Index: v.Index})
	case model.JobControlStatusResumed:
		m.composer.Reset()
		return m.handleJobResumed(jobResumedMsg{Index: v.Index, MessageID: v.Response.MessageID})
	default:
		return nil
	}
}

func (m *uiModel) handleJobFinished(v jobFinishedMsg) tea.Cmd {
	finishedAt := time.Now()
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		m.persistSelectedViewportState()
		job.retrying = false
		job.retryReason = ""
		if v.Success {
			job.state = jobSuccess
			m.completed++
		} else {
			job.state = jobFailed
			job.exitCode = v.ExitCode
			m.failed++
		}
		// The sidebar timer is driven by job.duration. Prefer the authoritative
		// duration reported with the completion, since the UI otherwise recomputes
		// elapsed time from startedAt — which is never seeded when a job's first
		// observed lifecycle message is a retry (retry attempts emit no fresh
		// start). Without this, the timer stays blank after a retried job succeeds.
		if v.DurationMs > 0 || !job.startedAt.IsZero() {
			job.completedAt = finishedAt
			if v.DurationMs > 0 {
				job.duration = time.Duration(v.DurationMs) * time.Millisecond
				// Backfill startedAt so the startedAt/completedAt/duration triple
				// stays coherent for consumers other than the sidebar timer.
				if job.startedAt.IsZero() {
					job.startedAt = job.completedAt.Add(-job.duration)
				}
			} else {
				job.duration = job.completedAt.Sub(job.startedAt)
			}
		}
		if finishedAt.After(m.now) {
			m.now = finishedAt
		}
		m.selectNextRunningJob()
		m.sidebarDirty = true
	}
	if m.isRunComplete() {
		m.closeQuitDialog()
		m.shutdown = shutdownState{}
	}
	if m.total > 0 && m.completed+m.failed >= m.total && m.failed > 0 && m.currentView != uiViewSummary {
		m.showSummaryView()
	}
	m.refreshViewportContent()
	if m.hasActiveJobs() {
		return m.ensureSpinnerTick()
	}
	m.spinnerRunning = false
	return nil
}

func (m *uiModel) handleJobUpdate(v jobUpdateMsg) tea.Cmd {
	updatedAt := time.Now()
	if job, created := m.ensureJobSlot(v.Index); job != nil {
		wasAtEnd := job.selectedEntry >= len(job.snapshot.Entries)-1
		job.snapshot = v.Snapshot
		if (created || job.state == jobPending) && v.Snapshot.Session.Status == model.StatusRunning {
			job.state = jobRunning
			if job.startedAt.IsZero() {
				job.startedAt = updatedAt
				job.duration = 0
			}
			if updatedAt.After(m.now) {
				m.now = updatedAt
			}
			m.selectedJob = v.Index
			m.sidebarDirty = true
		}
		job.timelineCacheValid = false
		if m.applyDefaultExpandedEntries(job) {
			job.expansionRevision++
		}
		m.syncSelectedEntry(job)
		if wasAtEnd && len(job.snapshot.Entries) > 0 {
			job.selectedEntry = len(job.snapshot.Entries) - 1
			job.transcriptFollowTail = true
		}
	}
	m.refreshViewportContent()
	return m.ensureSpinnerTick()
}

func (m *uiModel) handleUsageUpdate(v usageUpdateMsg) tea.Cmd {
	if job, _ := m.ensureJobSlot(v.Index); job != nil {
		if job.tokenUsage == nil {
			job.tokenUsage = &model.Usage{}
		}
		job.tokenUsage.Add(v.Usage)
		job.sidebarCacheValid = false
		m.sidebarDirty = true
	}
	if m.aggregateUsage != nil {
		m.aggregateUsage.Add(v.Usage)
	}
	m.refreshViewportContent()
	return nil
}

func (m *uiModel) handleShutdownStatus(v shutdownStatusMsg) tea.Cmd {
	if v.State.Active() {
		m.closeQuitDialog()
	}
	m.shutdown = v.State
	if !v.State.RequestedAt.IsZero() && v.State.RequestedAt.After(m.now) {
		m.now = v.State.RequestedAt
	}
	return nil
}

func (m *uiModel) sidebarNeedsActiveRefresh() bool {
	for i := range m.jobs {
		if m.jobs[i].state == jobRunning {
			return true
		}
	}
	return false
}

func (m *uiModel) sidebarNeedsClockRefresh() bool {
	for i := range m.jobs {
		if m.jobs[i].state == jobRunning {
			return true
		}
	}
	return false
}

func (m *uiModel) hasActiveJobs() bool {
	for i := range m.jobs {
		switch m.jobs[i].state {
		case jobRunning, jobPausing, jobRetrying:
			return true
		}
	}
	return false
}

func (m *uiModel) ensureJobSlot(index int) (*uiJob, bool) {
	if index < 0 {
		return nil, false
	}

	created := false
	if index >= len(m.jobs) {
		start := len(m.jobs)
		m.jobs = append(m.jobs, make([]uiJob, index-len(m.jobs)+1)...)
		for i := start; i <= index; i++ {
			m.jobs[i] = newPlaceholderUIJob(i)
		}
		created = true
	}
	if index+1 > m.total {
		m.total = index + 1
	}

	job := &m.jobs[index]
	if strings.TrimSpace(job.safeName) == "" {
		job.safeName = placeholderJobSafeName(index)
		created = true
	}
	if job.expandedEntryIDs == nil {
		job.expandedEntryIDs = make(map[string]bool)
	}
	if !job.transcriptFollowTail && len(job.snapshot.Entries) == 0 && job.selectedEntry == 0 &&
		job.startedAt.IsZero() && job.completedAt.IsZero() {
		job.selectedEntry = -1
		job.transcriptFollowTail = true
	}
	return job, created
}

func newPlaceholderUIJob(index int) uiJob {
	return uiJob{
		safeName:             placeholderJobSafeName(index),
		state:                jobPending,
		selectedEntry:        -1,
		expandedEntryIDs:     make(map[string]bool),
		transcriptFollowTail: true,
	}
}

func placeholderJobSafeName(index int) string {
	return fmt.Sprintf("job-%03d", index)
}

func mergeQueuedJobState(existing *uiJob, queued uiJob) uiJob {
	if existing == nil {
		return queued
	}
	mergeQueuedSnapshotState(existing, &queued)
	mergeQueuedTranscriptState(existing, &queued)
	mergeQueuedRuntimeState(existing, &queued)
	return queued
}

func mergeQueuedSnapshotState(existing *uiJob, queued *uiJob) {
	if existing == nil || queued == nil {
		return
	}
	if len(existing.snapshot.Entries) > 0 || existing.snapshot.Session.Status != "" ||
		len(existing.snapshot.Plan.Entries) > 0 {
		queued.snapshot = existing.snapshot
	}
	if existing.selectedEntry >= 0 {
		queued.selectedEntry = existing.selectedEntry
	}
	if len(existing.expandedEntryIDs) > 0 {
		queued.expandedEntryIDs = existing.expandedEntryIDs
	}
	if existing.expansionRevision > 0 {
		queued.expansionRevision = existing.expansionRevision
	}
	if existing.tokenUsage != nil {
		queued.tokenUsage = existing.tokenUsage
	}
}

func mergeQueuedTranscriptState(existing *uiJob, queued *uiJob) {
	if existing == nil || queued == nil {
		return
	}
	if existing.transcriptYOffset != 0 {
		queued.transcriptYOffset = existing.transcriptYOffset
	}
	if existing.transcriptXOffset != 0 {
		queued.transcriptXOffset = existing.transcriptXOffset
	}
}

func mergeQueuedRuntimeState(existing *uiJob, queued *uiJob) {
	if existing == nil || queued == nil {
		return
	}
	if existing.startedAt != (time.Time{}) {
		queued.startedAt = existing.startedAt
	}
	if existing.completedAt != (time.Time{}) {
		queued.completedAt = existing.completedAt
	}
	if existing.duration != 0 {
		queued.duration = existing.duration
	}
	if existing.attempt > 0 {
		queued.attempt = existing.attempt
	}
	if existing.maxAttempts > 0 {
		queued.maxAttempts = existing.maxAttempts
	}
	if existing.retrying {
		queued.retrying = true
		queued.retryReason = existing.retryReason
	}
	if existing.state != jobPending {
		queued.state = existing.state
	}
}

func (m *uiModel) syncSelectedEntry(job *uiJob) {
	if job == nil {
		return
	}
	if len(job.snapshot.Entries) == 0 {
		job.selectedEntry = -1
		return
	}
	if job.selectedEntry < 0 {
		job.selectedEntry = len(job.snapshot.Entries) - 1
	}
	if job.selectedEntry >= len(job.snapshot.Entries) {
		job.selectedEntry = len(job.snapshot.Entries) - 1
	}
}

func (m *uiModel) applyDefaultExpandedEntries(job *uiJob) bool {
	if job.expandedEntryIDs == nil {
		job.expandedEntryIDs = make(map[string]bool)
	}
	changed := false
	for i := range job.snapshot.Entries {
		entry := job.snapshot.Entries[i]
		if _, ok := job.expandedEntryIDs[entry.ID]; ok {
			continue
		}
		if entry.Kind == transcriptEntryToolCall {
			switch entry.ToolCallState {
			case model.ToolCallStateFailed, model.ToolCallStateWaitingForConfirmation:
				job.expandedEntryIDs[entry.ID] = true
				changed = true
			}
		}
	}
	return changed
}
