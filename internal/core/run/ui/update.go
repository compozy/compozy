package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

const (
	keyPageUp   = "pgup"
	keyPageDown = "pgdown"
	keyHome     = "home"
	keyEnd      = "end"
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
	case jobFinishedMsg:
		return m.applyUIMsg(v), true
	case jobUpdateMsg:
		return m.applyUIMsg(v), true
	case usageUpdateMsg:
		return m.applyUIMsg(v), true
	case shutdownStatusMsg:
		return m.applyUIMsg(v), true
	case jobFailureMsg:
		return m.applyUIMsg(v), true
	default:
		return nil, false
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
	case jobFinishedMsg:
		return m.handleJobFinished(value)
	case jobUpdateMsg:
		return m.handleJobUpdate(value)
	case usageUpdateMsg:
		return m.handleUsageUpdate(value)
	case shutdownStatusMsg:
		return m.handleShutdownStatus(value)
	case jobFailureMsg:
		m.failures = append(m.failures, value.Failure)
		return nil
	default:
		return nil
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
	if m.cfg != nil && m.cfg.DetachOnly {
		return tea.Quit
	}

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
			setSidebarViewportContent(&m.sidebarViewport, "")
			m.sidebarContent = ""
		}
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
	items := make([]string, 0, len(m.jobs))
	for i := range m.jobs {
		items = append(items, m.renderSidebarItem(&m.jobs[i], i == m.selectedJob))
	}
	content := strings.Join(items, "\n")
	if content != m.sidebarContent {
		setSidebarViewportContent(&m.sidebarViewport, content)
		m.sidebarContent = content
	}
	m.sidebarDirty = false

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
		if !job.startedAt.IsZero() {
			job.completedAt = finishedAt
			job.duration = job.completedAt.Sub(job.startedAt)
		}
		if finishedAt.After(m.now) {
			m.now = finishedAt
		}
		m.selectNextRunningJob()
		m.sidebarDirty = true
	}
	if m.isRunComplete() {
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
	if job, created := m.ensureJobSlot(v.Index); job != nil {
		if job.tokenUsage == nil {
			job.tokenUsage = &model.Usage{}
		}
		job.tokenUsage.Add(v.Usage)
		if created {
			m.sidebarDirty = true
		}
	}
	if m.aggregateUsage != nil {
		m.aggregateUsage.Add(v.Usage)
	}
	m.refreshViewportContent()
	return nil
}

func (m *uiModel) handleShutdownStatus(v shutdownStatusMsg) tea.Cmd {
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
		case jobRunning, jobRetrying:
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
