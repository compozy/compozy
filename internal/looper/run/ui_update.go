package run

import (
	"log/slog"
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
	case terminalOutputMsg:
		return m, m.handleTerminalOutput(v)
	case terminalReadyMsg:
		return m, m.handleTerminalReady(v)
	case composerSendMsg:
		return m, m.handleComposerSend(v)
	case jobDoneSignalMsg:
		return m, m.handleJobDoneSignal(v)
	case jobFinishedMsg:
		return m, m.handleJobFinished(v)
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
	if m.currentView != uiViewJobs {
		return m.handleNonJobViewKey(v)
	}

	if m.mode == modeTerminal {
		return m.handleTerminalModeKey(v)
	}

	return m.handleNavigateModeKey(v)
}

func (m *uiModel) handleNonJobViewKey(v tea.KeyMsg) tea.Cmd {
	switch v.String() {
	case "ctrl+c", "q":
		return tea.Quit
	case "esc":
		m.showJobsView()
		return nil
	case "s", "tab":
		m.showSummaryView()
		return nil
	default:
		return nil
	}
}

func (m *uiModel) handleNavigateModeKey(v tea.KeyMsg) tea.Cmd {
	switch v.String() {
	case "ctrl+c", "q":
		return tea.Quit
	case "s", "tab":
		m.showSummaryView()
		return nil
	case "up", "k", "down", "j":
		return m.handleNavigationKeys(v.String())
	case "enter":
		m.mode = modeTerminal
		return nil
	default:
		return nil
	}
}

func (m *uiModel) handleTerminalModeKey(v tea.KeyMsg) tea.Cmd {
	if v.Type == tea.KeyEsc {
		m.mode = modeNavigate
		return nil
	}

	term := m.currentTerminal()
	if term == nil {
		return nil
	}

	input := translateKey(v)
	if len(input) == 0 {
		return nil
	}

	if err := term.WriteInput(input); err != nil {
		slog.Warn("forward terminal input failed", "job_index", m.selectedJob, "error", err)
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
		return
	}
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		m.selectedJob = 0
	}
}

func (m *uiModel) selectNextPendingJob() bool {
	for i := range m.jobs {
		if m.jobs[i].state == jobPending {
			m.selectedJob = i
			return true
		}
	}
	return false
}

func (m *uiModel) selectNextRunningJob() {
	for i := range m.jobs {
		if m.jobs[i].state == jobRunning {
			m.selectedJob = i
			return
		}
	}
}

func (m *uiModel) currentTerminal() *Terminal {
	if m.selectedJob < 0 || m.selectedJob >= len(m.terminals) {
		return nil
	}
	return m.terminals[m.selectedJob]
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
		codeFile:   v.CodeFile,
		codeFiles:  v.CodeFiles,
		issues:     v.Issues,
		safeName:   v.SafeName,
		outLog:     v.OutLog,
		errLog:     v.ErrLog,
		state:      jobPending,
		statusText: "queued",
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobStarted(v jobStartedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
		job.state = jobRunning
		job.statusText = "starting terminal"
		if !job.startedAt.IsZero() {
			job.duration = 0
		} else {
			job.startedAt = time.Now()
		}
		if v.Terminal != nil && v.Index < len(m.terminals) {
			m.terminals[v.Index] = v.Terminal
		}
		m.selectedJob = v.Index
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleTerminalOutput(v terminalOutputMsg) tea.Cmd {
	if v.Index < len(m.jobs) && len(v.Data) > 0 {
		if m.jobs[v.Index].statusText == "queued" {
			m.jobs[v.Index].statusText = "receiving output"
		}
	}
	return m.waitEvent()
}

func (m *uiModel) handleTerminalReady(v terminalReadyMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.jobs[v.Index].statusText = "composer ready"
	}
	return m.waitEvent()
}

func (m *uiModel) handleComposerSend(v composerSendMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.jobs[v.Index].statusText = "prompt sent"
	}
	return m.waitEvent()
}

func (m *uiModel) handleJobDoneSignal(v jobDoneSignalMsg) tea.Cmd {
	for i := range m.jobs {
		if m.jobs[i].safeName != v.JobID {
			continue
		}

		job := &m.jobs[i]
		if job.state == jobSuccess || job.state == jobFailed {
			break
		}

		job.state = jobSuccess
		job.statusText = "done signaled"
		job.exitCode = 0
		job.completedAt = time.Now()
		if !job.startedAt.IsZero() {
			job.duration = job.completedAt.Sub(job.startedAt)
		}
		m.completed++
		if !m.selectNextPendingJob() {
			m.selectNextRunningJob()
		}
		break
	}
	m.refreshViewportContent()
	return m.waitSignal()
}

func (m *uiModel) handleJobFinished(v jobFinishedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
		if v.Success {
			if job.state != jobSuccess {
				job.state = jobSuccess
				job.statusText = "completed"
				m.completed++
			}
		} else {
			if job.state == jobSuccess && m.completed > 0 {
				m.completed--
			}
			job.state = jobFailed
			job.statusText = "failed"
			job.exitCode = v.ExitCode
			m.failed++
		}
		if !job.startedAt.IsZero() {
			job.completedAt = time.Now()
			job.duration = job.completedAt.Sub(job.startedAt)
		}
		if !m.selectNextPendingJob() {
			m.selectNextRunningJob()
		}
	}
	m.refreshViewportContent()
	return m.waitEvent()
}
