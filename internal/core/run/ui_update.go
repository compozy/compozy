package run

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		return m, m.handleKey(v)
	case tea.WindowSizeMsg:
		m.handleWindowSize(v)
		return m, nil
	case tickMsg:
		return m, m.handleTick()
	case jobQueuedMsg:
		return m, m.handleJobQueued(&v)
	case jobStartedMsg:
		return m, m.handleJobStarted(v)
	case jobFinishedMsg:
		return m, m.handleJobFinished(v)
	case jobLogUpdateMsg:
		return m, m.handleJobLogUpdate(v)
	case tokenUsageUpdateMsg:
		return m, m.handleTokenUsageUpdate(v)
	case jobFailureMsg:
		m.failures = append(m.failures, v.Failure)
		return m, nil
	case drainMsg:
		return m, nil
	default:
		return m, nil
	}
}

func (m *uiModel) handleKey(v tea.KeyMsg) tea.Cmd {
	key := v.String()
	switch key {
	case "ctrl+c", "q":
		if m.onQuit != nil {
			m.onQuit()
		}
		return tea.Quit
	case "s", "tab", "esc":
		return m.handleViewSwitchKeys(key)
	case "up", "k", "down", "j":
		return m.handleNavigationKeys(key)
	case "left", "h", "right", "l", "pgup", "b", "u", "pgdown", "f", "d", "home", "end":
		return m.handleScrollKeys(key)
	default:
		return m.waitEvent()
	}
}

func (m *uiModel) handleViewSwitchKeys(key string) tea.Cmd {
	switch key {
	case "s", "tab":
		m.showSummaryView()
	case "esc":
		m.showJobsView()
	}
	return nil
}

func (m *uiModel) showJobsView() {
	m.currentView = uiViewJobs
	m.refreshViewportContent()
}

func (m *uiModel) showSummaryView() {
	if m.completed+m.failed < m.total {
		return
	}
	m.currentView = uiViewSummary
}

func (m *uiModel) handleNavigationKeys(key string) tea.Cmd {
	switch key {
	case "up", "k":
		if m.selectedJob > 0 {
			m.selectedJob--
		}
	case "down", "j":
		if m.selectedJob < len(m.jobs)-1 {
			m.selectedJob++
		}
	}
	m.refreshViewportContent()
	return nil
}

func (m *uiModel) handleScrollKeys(key string) tea.Cmd {
	switch key {
	case "left", "h":
		m.viewport.ScrollUp(1)
	case "right", "l":
		m.viewport.ScrollDown(1)
	case "pgup", "b", "u":
		m.viewport.HalfPageUp()
	case "pgdown", "f", "d":
		m.viewport.HalfPageDown()
	case "home":
		m.viewport.GotoTop()
	case "end":
		m.viewport.GotoBottom()
	}
	return nil
}

func (m *uiModel) handleWindowSize(v tea.WindowSizeMsg) {
	m.width = v.Width
	m.height = v.Height
	sidebarWidth, mainWidth := m.computePaneWidths(v.Width)
	contentHeight := m.computeContentHeight(v.Height)
	m.configureViewports(sidebarWidth, mainWidth, contentHeight)
	m.sidebarWidth = sidebarWidth
	m.mainWidth = mainWidth
	m.contentHeight = contentHeight
	m.refreshViewportContent()
}

func (m *uiModel) refreshViewportContent() {
	if len(m.jobs) == 0 {
		m.viewport.SetContent("")
		return
	}
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		m.selectedJob = 0
	}
	m.updateViewportForJob(&m.jobs[m.selectedJob])
}

func (m *uiModel) selectNextRunningJob() {
	for i := range m.jobs {
		if m.jobs[i].state == jobRunning {
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
	m.frame++
	return m.tick()
}

func (m *uiModel) handleJobQueued(v *jobQueuedMsg) tea.Cmd {
	if v.Index >= len(m.jobs) {
		grow := v.Index - len(m.jobs) + 1
		m.jobs = append(m.jobs, make([]uiJob, grow)...)
	}
	m.jobs[v.Index] = uiJob{
		codeFile:  v.CodeFile,
		codeFiles: v.CodeFiles,
		issues:    v.Issues,
		safeName:  v.SafeName,
		outLog:    v.OutLog,
		errLog:    v.ErrLog,
		state:     jobPending,
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobStarted(v jobStartedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
		job.state = jobRunning
		if job.startedAt.IsZero() {
			job.startedAt = time.Now()
			job.duration = 0
		}
		m.selectedJob = v.Index
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobFinished(v jobFinishedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
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
	if m.total > 0 && m.completed+m.failed >= m.total && m.failed > 0 && m.currentView != uiViewSummary {
		m.showSummaryView()
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobLogUpdate(v jobLogUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.jobs[v.Index].lastOut = v.Out
		m.jobs[v.Index].lastErr = v.Err
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleTokenUsageUpdate(v tokenUsageUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		if m.jobs[v.Index].tokenUsage == nil {
			m.jobs[v.Index].tokenUsage = &TokenUsage{}
		}
		m.jobs[v.Index].tokenUsage.Add(v.Usage)
	}
	if m.aggregateUsage != nil {
		m.aggregateUsage.Add(v.Usage)
	}
	m.refreshViewportContent()
	return m.waitEvent()
}
