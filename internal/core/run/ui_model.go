package run

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type uiModel struct {
	jobs            []uiJob
	total           int
	completed       int
	failed          int
	frame           int
	events          <-chan uiMsg
	onQuit          func()
	viewport        viewport.Model
	sidebarViewport viewport.Model
	selectedJob     int
	width           int
	height          int
	sidebarWidth    int
	mainWidth       int
	contentHeight   int
	currentView     uiViewState
	ctx             context.Context
	failures        []failInfo
	aggregateUsage  *TokenUsage
}

func newUIModel(ctx context.Context, total int) *uiModel {
	vp := viewport.New(80, 24)
	sidebarVp := viewport.New(30, 24)
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
	return &uiModel{
		total:           total,
		viewport:        vp,
		sidebarViewport: sidebarVp,
		selectedJob:     0,
		width:           defaultWidth,
		height:          defaultHeight,
		sidebarWidth:    initialSidebarWidth,
		mainWidth:       initialMainWidth,
		contentHeight:   initialContentHeight,
		currentView:     uiViewJobs,
		ctx:             ctx,
		failures:        []failInfo{},
		aggregateUsage:  &TokenUsage{},
	}
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

func setupUI(ctx context.Context, jobs []job, _ *config, enabled bool) (chan uiMsg, *tea.Program) {
	if !enabled {
		return nil, nil
	}
	total := len(jobs)
	uiCh := make(chan uiMsg, total*4)
	mdl := newUIModel(ctx, total)
	mdl.setEventSource(uiCh)
	prog := tea.NewProgram(mdl, tea.WithAltScreen())
	go func() {
		if _, runErr := prog.Run(); runErr != nil {
			fmt.Fprintf(os.Stderr, "UI program error: %v\n", runErr)
		}
	}()
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
			SafeName:  jb.safeName,
			OutLog:    jb.outLog,
			ErrLog:    jb.errLog,
		}
	}
	go func() {
		<-ctx.Done()
		prog.Quit()
	}()
	return uiCh, prog
}
