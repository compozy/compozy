package run

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/model"

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
	prog            *tea.Program
	done            chan error
	quitHandler     func(uiQuitRequest)
	quitHandlerMu   sync.RWMutex
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

func newUIController(_ context.Context, total int, cfg *config) *uiController {
	uiCh := make(chan uiMsg, max(total*4, 4))
	mdl := newUIModel(total)
	mdl.cfg = cfg
	mdl.setEventSource(uiCh)

	ctrl := &uiController{
		ch:   uiCh,
		done: make(chan error, 1),
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

func (c *uiController) events() chan uiMsg {
	return c.ch
}

func (c *uiController) setQuitHandler(fn func(uiQuitRequest)) {
	c.quitHandlerMu.Lock()
	defer c.quitHandlerMu.Unlock()
	c.quitHandler = fn
}

func (c *uiController) requestQuit(req uiQuitRequest) {
	c.quitHandlerMu.RLock()
	fn := c.quitHandler
	c.quitHandlerMu.RUnlock()
	if fn != nil {
		fn(req)
	}
}

func (c *uiController) closeEvents() {
	c.closeEventsOnce.Do(func() {
		close(c.ch)
	})
}

func (c *uiController) shutdown() {
	c.shutdownOnce.Do(func() {
		if c.prog != nil {
			c.prog.Quit()
		}
	})
}

func (c *uiController) wait() error {
	err, ok := <-c.done
	if !ok {
		return nil
	}
	return err
}

func setupUI(ctx context.Context, jobs []job, cfg *config, enabled bool) uiSession {
	if !enabled {
		return nil
	}
	ctrl := newUIController(ctx, len(jobs), cfg)
	uiCh := ctrl.events()
	for idx := range jobs {
		jb := &jobs[idx]
		totalIssues := 0
		for _, items := range jb.groups {
			totalIssues += len(items)
		}
		codeFileLabel := strings.Join(jb.codeFiles, ", ")
		if len(jb.codeFiles) > 3 {
			codeFileLabel = fmt.Sprintf("%s and %d more", strings.Join(jb.codeFiles[:3], ", "), len(jb.codeFiles)-3)
		}
		uiCh <- jobQueuedMsg{
			Index:     idx,
			CodeFile:  codeFileLabel,
			CodeFiles: jb.codeFiles,
			Issues:    totalIssues,
			TaskTitle: jb.taskTitle,
			TaskType:  jb.taskType,
			SafeName:  jb.safeName,
			OutLog:    jb.outLog,
			ErrLog:    jb.errLog,
			OutBuffer: jb.outBuffer,
			ErrBuffer: jb.errBuffer,
		}
	}
	return ctrl
}
