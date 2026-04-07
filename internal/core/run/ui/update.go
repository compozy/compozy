package ui

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"

	tea "charm.land/bubbletea/v2"
)

const (
	keyPageUp   = "pgup"
	keyPageDown = "pgdown"
	keyHome     = "home"
	keyEnd      = "end"
)

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
	case tickMsg:
		return m, m.handleTick()
	case jobQueuedMsg:
		return m, m.handleJobQueued(&v)
	case jobStartedMsg:
		return m, m.handleJobStarted(v)
	case jobRetryMsg:
		return m, m.handleJobRetry(v)
	case jobFinishedMsg:
		return m, m.handleJobFinished(v)
	case jobUpdateMsg:
		return m, m.handleJobUpdate(v)
	case usageUpdateMsg:
		return m, m.handleUsageUpdate(v)
	case shutdownStatusMsg:
		return m, m.handleShutdownStatus(v)
	case jobFailureMsg:
		m.failures = append(m.failures, v.Failure)
		return m, nil
	case drainMsg:
		return m, nil
	default:
		return m, nil
	}
}

func (m *uiModel) handleKey(v tea.KeyPressMsg) tea.Cmd {
	key := v.String()
	switch key {
	case "ctrl+c", "q":
		return m.handleQuitKey()
	case "s":
		return m.handleSummaryToggle()
	case "esc":
		return m.handleEscape()
	case "tab":
		m.cycleFocusedPane(1)
		return nil
	case "shift+tab":
		m.cycleFocusedPane(-1)
		return nil
	case "enter":
		m.toggleSelectedEntryExpansion()
		return nil
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

func (m *uiModel) handleQuitKey() tea.Cmd {
	if m.isRunComplete() {
		return tea.Quit
	}

	req, ok := m.nextQuitRequest()
	if !ok {
		return nil
	}
	if m.currentView == uiViewJobs {
		m.refreshSidebarContent()
	}
	if m.onQuit != nil {
		m.onQuit(req)
	}
	return nil
}

func (m *uiModel) nextQuitRequest() (uiQuitRequest, bool) {
	now := time.Now()
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

func (m *uiModel) cycleFocusedPane(direction int) {
	if m.currentView != uiViewJobs {
		return
	}
	order := m.visiblePanes()
	if len(order) == 0 {
		return
	}

	currentIdx := 0
	for i, pane := range order {
		if pane == m.focusedPane {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + direction + len(order)) % len(order)
	m.focusedPane = order[nextIdx]
}

func (m *uiModel) visiblePanes() []uiPane {
	if m.layoutMode == uiLayoutResizeBlocked {
		return nil
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
	next := m.selectedJob + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.jobs) {
		next = len(m.jobs) - 1
	}
	m.selectedJob = next
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
	m.refreshViewportContent()
}

func (m *uiModel) refreshViewportContent() {
	if len(m.jobs) == 0 {
		m.sidebarViewport.SetContent("")
		return
	}
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		m.selectedJob = 0
	}

	m.refreshSidebarContent()
	job := &m.jobs[m.selectedJob]
	m.syncSelectedEntry(job)
}

func (m *uiModel) refreshSidebarContent() {
	var items []string
	for i := range m.jobs {
		items = append(items, m.renderSidebarItem(&m.jobs[i], i == m.selectedJob))
	}
	m.sidebarViewport.SetContent(strings.Join(items, "\n"))

	lineOffset := m.selectedJob * 3
	sidebarOffset := m.sidebarViewport.YOffset()
	sidebarHeight := m.sidebarViewport.Height()
	if lineOffset > sidebarOffset+sidebarHeight-3 {
		m.sidebarViewport.SetYOffset(lineOffset - sidebarHeight + 3)
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

func (m *uiModel) handleTick() tea.Cmd {
	if m.isRunComplete() {
		return nil
	}
	m.frame++
	if m.currentView == uiViewJobs && len(m.jobs) > 0 {
		m.refreshSidebarContent()
	}
	return m.tick()
}

func (m *uiModel) handleJobQueued(v *jobQueuedMsg) tea.Cmd {
	if v.Index >= len(m.jobs) {
		grow := v.Index - len(m.jobs) + 1
		m.jobs = append(m.jobs, make([]uiJob, grow)...)
	}
	m.jobs[v.Index] = uiJob{
		codeFile:             v.CodeFile,
		codeFiles:            v.CodeFiles,
		issues:               v.Issues,
		taskTitle:            v.TaskTitle,
		taskType:             v.TaskType,
		safeName:             v.SafeName,
		outLog:               v.OutLog,
		errLog:               v.ErrLog,
		outBuffer:            v.OutBuffer,
		errBuffer:            v.ErrBuffer,
		state:                jobPending,
		selectedEntry:        -1,
		expandedEntryIDs:     make(map[string]bool),
		transcriptFollowTail: true,
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobStarted(v jobStartedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.persistSelectedViewportState()
		job := &m.jobs[v.Index]
		job.state = jobRunning
		job.attempt = max(v.Attempt, 1)
		job.maxAttempts = max(v.MaxAttempts, job.attempt)
		job.retrying = false
		job.retryReason = ""
		if job.startedAt.IsZero() {
			job.startedAt = time.Now()
			job.duration = 0
		}
		m.selectedJob = v.Index
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobRetry(v jobRetryMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.persistSelectedViewportState()
		job := &m.jobs[v.Index]
		job.state = jobRetrying
		job.attempt = max(v.Attempt, 1)
		job.maxAttempts = max(v.MaxAttempts, job.attempt)
		job.retrying = true
		job.retryReason = v.Reason
		m.selectedJob = v.Index
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobFinished(v jobFinishedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.persistSelectedViewportState()
		job := &m.jobs[v.Index]
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
		if !job.startedAt.IsZero() {
			job.completedAt = time.Now()
			job.duration = job.completedAt.Sub(job.startedAt)
		}
		m.selectNextRunningJob()
	}
	if m.isRunComplete() {
		m.shutdown = shutdownState{}
	}
	if m.total > 0 && m.completed+m.failed >= m.total && m.failed > 0 && m.currentView != uiViewSummary {
		m.showSummaryView()
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobUpdate(v jobUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
		wasAtEnd := job.selectedEntry >= len(job.snapshot.Entries)-1
		job.snapshot = v.Snapshot
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
	return m.waitEvent()
}

func (m *uiModel) handleUsageUpdate(v usageUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		if m.jobs[v.Index].tokenUsage == nil {
			m.jobs[v.Index].tokenUsage = &model.Usage{}
		}
		m.jobs[v.Index].tokenUsage.Add(v.Usage)
	}
	if m.aggregateUsage != nil {
		m.aggregateUsage.Add(v.Usage)
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleShutdownStatus(v shutdownStatusMsg) tea.Cmd {
	m.shutdown = v.State
	if m.currentView == uiViewJobs && len(m.jobs) > 0 {
		m.refreshSidebarContent()
	}
	return m.waitEvent()
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
