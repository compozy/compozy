package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/compozy/compozy/internal/core/tasks"

	"charm.land/lipgloss/v2"
)

// integrationPhase is the INTEGRATION pane's current lifecycle phase. The pane
// is omitted while idle/running/merged and renders during merging, conflicts,
// resolver progress, and rollback so the operator keeps full context.
type integrationPhase int

const (
	integrationPhaseIdle integrationPhase = iota
	integrationPhaseRunning
	integrationPhaseMerging
	integrationPhaseConflict
	integrationPhaseResolving
	integrationPhaseRolledBack
	integrationPhaseDone
)

// waveStatus is one wave's rollup status, rendered both in the sidebar wave
// headers and the INTEGRATION pane wave-progress strip.
type waveStatus int

const (
	waveStatusPending waveStatus = iota
	waveStatusRunning
	waveStatusMerging
	waveStatusConflict
	waveStatusMerged
)

const (
	waveGlyphPending  = "⏸"
	waveGlyphRunning  = "●"
	waveGlyphMerging  = "▸"
	waveGlyphConflict = "⚠"
	waveGlyphMerged   = "✓"
)

// integrationConflict captures the active merge-conflict context streamed into the
// INTEGRATION pane during conflict_* phases.
type integrationConflict struct {
	taskID      string
	taskNumber  int
	waveIndex   int
	files       []string
	attempt     int
	maxAttempts int
	resolving   bool
}

// parallelView holds all operator-facing state derived from task.parallel.* events.
// It is nil for non-parallel runs so the existing single-run/multi-run sidebar and
// main-pane layout are untouched.
type parallelView struct {
	waveTotal         int
	waves             []waveStatus
	taskWave          map[int]int
	integrationBranch string
	lifecyclePhase    string
	phase             integrationPhase
	conflict          *integrationConflict
	resolverLines     []string
}

func newParallelView() *parallelView {
	return &parallelView{taskWave: make(map[int]int)}
}

// parallel lazily initializes and returns the parallel view state.
func (m *uiModel) parallelState() *parallelView {
	if m.parallel == nil {
		m.parallel = newParallelView()
	}
	return m.parallel
}

func (p *parallelView) ensureWaves(count int) {
	if count > p.waveTotal {
		p.waveTotal = count
	}
	for len(p.waves) < p.waveTotal {
		p.waves = append(p.waves, waveStatusPending)
	}
}

func (p *parallelView) ensureWaveIndex(idx int) {
	if idx < 0 {
		return
	}
	p.ensureWaves(idx + 1)
}

// setWaveStatus advances a wave's status. Status is monotonic across a wave's
// lifecycle (pending -> running -> merging/conflict -> merged); a later event never
// regresses a wave already marked merged.
func (p *parallelView) setWaveStatus(idx int, status waveStatus) {
	p.ensureWaveIndex(idx)
	if idx < 0 || idx >= len(p.waves) {
		return
	}
	if p.waves[idx] == waveStatusMerged && status != waveStatusMerged {
		return
	}
	p.waves[idx] = status
}

func (p *parallelView) waveStatusAt(idx int) waveStatus {
	if idx < 0 || idx >= len(p.waves) {
		return waveStatusPending
	}
	return p.waves[idx]
}

// assignTask records which wave a task belongs to, keyed by its canonical task
// number so the sidebar can group job cards without the DAG.
func (p *parallelView) assignTask(taskID string, waveIndex int) {
	number := tasks.ExtractTaskIdentityNumber(taskID)
	if number <= 0 {
		return
	}
	if p.taskWave == nil {
		p.taskWave = make(map[int]int)
	}
	p.taskWave[number] = waveIndex
}

// waveOfJob returns the wave a job belongs to, or -1 when the job has no parallel
// wave assignment yet.
func (p *parallelView) waveOfJob(job *uiJob) int {
	if p == nil || job == nil || job.taskNumber <= 0 || p.taskWave == nil {
		return -1
	}
	if wave, ok := p.taskWave[job.taskNumber]; ok {
		return wave
	}
	return -1
}

func (p *parallelView) grouped() bool {
	return p != nil && p.observedWaveCount() > 0 && len(p.taskWave) > 0
}

func (p *parallelView) expanded() bool {
	if p == nil {
		return false
	}
	switch p.phase {
	case integrationPhaseMerging, integrationPhaseConflict, integrationPhaseResolving, integrationPhaseRolledBack:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Event handlers (invoked from update.go)
// ---------------------------------------------------------------------------

func (m *uiModel) handleParallelPlanStarted(v parallelPlanStartedMsg) {
	p := m.parallelState()
	if len(v.Waves) > 0 {
		p.ensureWaves(len(v.Waves))
	}
	if strings.TrimSpace(v.IntegrationBranch) != "" {
		p.integrationBranch = strings.TrimSpace(v.IntegrationBranch)
	}
	for _, wave := range v.Waves {
		p.ensureWaveIndex(wave.Index)
		for _, taskID := range wave.TaskIDs {
			p.assignTask(taskID, wave.Index)
		}
	}
	for _, task := range v.Tasks {
		number := task.Number
		if number <= 0 {
			number = tasks.ExtractTaskIdentityNumber(firstNonEmpty(task.ID, task.File))
		}
		if number <= 0 {
			continue
		}
		taskID := strings.TrimSpace(task.ID)
		if taskID == "" {
			taskID = fmt.Sprintf("task_%02d", number)
		}
		file := strings.TrimSpace(task.File)
		if file == "" {
			file = fmt.Sprintf("task_%02d.md", number)
		}
		p.assignTask(taskID, task.WaveIndex)
		if !m.hasTaskNumber(number) {
			m.handleJobQueued(&jobQueuedMsg{
				Index:      len(m.jobs),
				CodeFile:   file,
				TaskNumber: number,
				TaskTitle:  firstNonEmpty(task.Title, taskID),
				SafeName:   taskID,
			})
		}
	}
	if p.phase == integrationPhaseIdle || p.phase == integrationPhaseDone {
		p.phase = integrationPhaseRunning
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelWaveStarted(v parallelWaveStartedMsg) {
	m.markParallelTaskRunning(v.WaveTotal, v.WaveIndex, v.TaskID, "", "", v.IntegrationBranch)
}

func (m *uiModel) handleParallelTaskStarted(v parallelTaskStartedMsg) {
	m.markParallelTaskRunning(v.WaveTotal, v.WaveIndex, v.TaskID, v.ChildRunID, v.WorktreePath, v.IntegrationBranch)
}

func (m *uiModel) handleParallelTaskCompleted(v parallelTaskCompletedMsg) {
	p := m.parallelState()
	p.ensureWaveIndex(v.WaveIndex)
	p.assignTask(v.TaskID, v.WaveIndex)
	number := tasks.ExtractTaskIdentityNumber(v.TaskID)
	index, ok := m.taskNumberIndex(number)
	if !ok {
		m.sidebarDirty = true
		return
	}
	success := v.Status == "merged" || v.Status == "recovered"
	finishParallelAggregateTask(m, index, success)
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelPhaseChanged(v parallelPhaseChangedMsg) {
	p := m.parallelState()
	p.ensureWaveIndex(v.WaveIndex)
	if branch := strings.TrimSpace(v.IntegrationBranch); branch != "" {
		p.integrationBranch = branch
	}
	p.lifecyclePhase = strings.TrimSpace(v.Phase)
	switch p.lifecyclePhase {
	case "completed", "canceled", "failed":
		p.phase = integrationPhaseDone
	case "rolled_back":
		p.phase = integrationPhaseRolledBack
	default:
		p.phase = integrationPhaseMerging
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelSettled(v parallelSettledMsg) {
	p := m.parallelState()
	p.lifecyclePhase = strings.TrimSpace(v.Status)
	p.conflict = nil
	p.resolverLines = nil
	p.phase = integrationPhaseDone
	if v.Status == "completed" {
		finishActiveParallelAggregateJobs(m, true)
	} else {
		failActiveParallelAggregateJobs(m)
	}
	m.sidebarDirty = true
}

func (m *uiModel) markParallelTaskRunning(
	waveTotal int,
	waveIndex int,
	taskID string,
	childRunID string,
	worktreePath string,
	integrationBranch string,
) {
	p := m.parallelState()
	p.ensureWaves(waveTotal)
	p.ensureWaveIndex(waveIndex)
	p.assignTask(taskID, waveIndex)
	m.markParallelTaskRuntimeLink(taskID, childRunID, worktreePath)
	p.setWaveStatus(waveIndex, waveStatusRunning)
	if strings.TrimSpace(integrationBranch) != "" {
		p.integrationBranch = strings.TrimSpace(integrationBranch)
	}
	if p.phase == integrationPhaseIdle || p.phase == integrationPhaseDone {
		p.phase = integrationPhaseRunning
	}
	m.sidebarDirty = true
}

func (m *uiModel) markParallelTaskRuntimeLink(taskID string, childRunID string, worktreePath string) {
	childRunID = strings.TrimSpace(childRunID)
	worktreePath = strings.TrimSpace(worktreePath)
	if childRunID == "" && worktreePath == "" {
		return
	}
	number := tasks.ExtractTaskIdentityNumber(taskID)
	index, ok := m.taskNumberIndex(number)
	if !ok {
		return
	}
	if childRunID != "" {
		m.jobs[index].childRunID = childRunID
	}
	if worktreePath != "" {
		m.jobs[index].worktreePath = worktreePath
	}
}

func (m *uiModel) handleParallelFailed(v parallelFailedMsg) {
	p := m.parallelState()
	p.ensureWaveIndex(v.WaveIndex)
	if strings.TrimSpace(v.IntegrationBranch) != "" {
		p.integrationBranch = strings.TrimSpace(v.IntegrationBranch)
	}
	p.phase = integrationPhaseDone
	p.lifecyclePhase = "failed"
	if v.Err != nil {
		m.failures = append(m.failures, failInfo{ExitCode: 1, Err: v.Err})
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelMergeStarted(v parallelMergeStartedMsg) {
	p := m.parallelState()
	p.ensureWaves(v.WaveTotal)
	p.ensureWaveIndex(v.WaveIndex)
	p.setWaveStatus(v.WaveIndex, waveStatusMerging)
	if strings.TrimSpace(v.IntegrationBranch) != "" {
		p.integrationBranch = strings.TrimSpace(v.IntegrationBranch)
	}
	if p.conflict == nil {
		p.phase = integrationPhaseMerging
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelConflict(v parallelConflictMsg) {
	p := m.parallelState()
	p.ensureWaveIndex(v.WaveIndex)
	p.setWaveStatus(v.WaveIndex, waveStatusConflict)
	p.conflict = &integrationConflict{
		taskID:      strings.TrimSpace(v.TaskID),
		taskNumber:  tasks.ExtractTaskIdentityNumber(v.TaskID),
		waveIndex:   v.WaveIndex,
		files:       append([]string(nil), v.Files...),
		attempt:     v.Attempt,
		maxAttempts: v.MaxAttempts,
		resolving:   v.Resolving,
	}
	if v.Resolving {
		p.phase = integrationPhaseResolving
	} else {
		p.phase = integrationPhaseConflict
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelMerged(v parallelMergedMsg) {
	p := m.parallelState()
	p.ensureWaveIndex(v.WaveIndex)
	p.assignTask(v.TaskID, v.WaveIndex)
	number := tasks.ExtractTaskIdentityNumber(v.TaskID)
	if p.conflict != nil && (p.conflict.taskNumber == number || p.conflict.taskID == strings.TrimSpace(v.TaskID)) {
		p.conflict = nil
		p.resolverLines = nil
	}
	if p.conflict == nil {
		p.phase = integrationPhaseMerging
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelWaveCompleted(v parallelWaveCompletedMsg) {
	p := m.parallelState()
	p.ensureWaves(v.WaveTotal)
	p.ensureWaveIndex(v.WaveIndex)
	p.setWaveStatus(v.WaveIndex, waveStatusMerged)
	p.conflict = nil
	p.resolverLines = nil
	if allWavesMerged(p) {
		p.phase = integrationPhaseDone
	} else {
		p.phase = integrationPhaseRunning
	}
	m.sidebarDirty = true
}

func (m *uiModel) handleParallelRolledBack(v parallelRolledBackMsg) {
	p := m.parallelState()
	p.ensureWaveIndex(v.WaveIndex)
	if strings.TrimSpace(v.IntegrationBranch) != "" {
		p.integrationBranch = strings.TrimSpace(v.IntegrationBranch)
	}
	p.phase = integrationPhaseRolledBack
	p.lifecyclePhase = "rolled_back"
	m.sidebarDirty = true
}

func allWavesMerged(p *parallelView) bool {
	if p == nil || p.waveTotal == 0 {
		return false
	}
	for i := 0; i < p.waveTotal; i++ {
		if p.waveStatusAt(i) != waveStatusMerged {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Sidebar wave grouping
// ---------------------------------------------------------------------------

// visualJobOrder returns the same wave-major order used by the sidebar renderer.
// Keyboard navigation consumes this order so moving down never jumps to a card
// that is visually above the current selection.
func (m *uiModel) visualJobOrder() []int {
	order := make([]int, 0, len(m.jobs))
	if m.parallel == nil || !m.parallel.grouped() {
		for index := range m.jobs {
			order = append(order, index)
		}
		return order
	}
	rendered := make([]bool, len(m.jobs))
	for waveIndex := 0; waveIndex < m.parallel.observedWaveCount(); waveIndex++ {
		for index := range m.jobs {
			if m.parallel.waveOfJob(&m.jobs[index]) != waveIndex {
				continue
			}
			order = append(order, index)
			rendered[index] = true
		}
	}
	for index := range m.jobs {
		if !rendered[index] {
			order = append(order, index)
		}
	}
	return order
}

// renderWaveGroupedSidebar groups job cards under wave headers, reusing
// renderSidebarItem for the cards. Jobs without a wave assignment render in a
// trailing ungrouped stack so nothing is dropped.
func (m *uiModel) renderWaveGroupedSidebar(width int) (string, int) {
	p := m.parallel
	waveCount := p.observedWaveCount()
	sections := make([]string, 0, waveCount+1)
	rendered := make([]bool, len(m.jobs))
	selectedOffset := 0
	lineCount := 0
	for waveIdx := 0; waveIdx < waveCount; waveIdx++ {
		header := m.renderWaveHeader(waveIdx, width)
		items := make([]string, 0)
		for i := range m.jobs {
			if p.waveOfJob(&m.jobs[i]) != waveIdx {
				continue
			}
			if i == m.selectedJob {
				selectedOffset = lineCount + 1 + len(items)*sidebarRowStride
			}
			items = append(items, m.renderSidebarItem(i, &m.jobs[i], i == m.selectedJob))
			rendered[i] = true
		}
		section := header
		sectionLines := 1
		if len(items) > 0 {
			section += "\n" + renderSidebarStack(items, width)
			sectionLines += sidebarStackLineCount(len(items))
		}
		sections = append(sections, section)
		lineCount += sectionLines
	}
	ungrouped := make([]string, 0)
	for i := range m.jobs {
		if rendered[i] {
			continue
		}
		if i == m.selectedJob {
			selectedOffset = lineCount + len(ungrouped)*sidebarRowStride
		}
		ungrouped = append(ungrouped, m.renderSidebarItem(i, &m.jobs[i], i == m.selectedJob))
	}
	if len(ungrouped) > 0 {
		sections = append(sections, renderSidebarStack(ungrouped, width))
	}
	return strings.Join(sections, "\n"), selectedOffset
}

func sidebarStackLineCount(items int) int {
	if items <= 0 {
		return 0
	}
	return sidebarRowLines + (items-1)*sidebarRowStride
}

// renderWaveHeader draws one wave grouping header: WAVE N <glyphs> <label>. Running
// waves repeat the running glyph once per concurrently-running job so concurrent
// activity reads at a glance. Foreground/border only per the theme constraint.
func (m *uiModel) renderWaveHeader(waveIdx, width int) string {
	status := m.parallel.waveStatusAt(waveIdx)
	glyph := m.waveHeaderGlyphs(waveIdx, status)
	label := waveStatusLabel(status)
	text := fmt.Sprintf("WAVE %d %s %s", waveIdx+1, glyph, label)
	style := lipgloss.NewStyle().Bold(true).Foreground(waveStatusColor(status))
	return renderOwnedLineKnownOwned(width, style.Render(truncateString(text, max(width, 1))))
}

// waveHeaderGlyphs returns the wave's status glyph. A running wave shows one
// spinner frame per running job (capped) so concurrency is visible.
func (m *uiModel) waveHeaderGlyphs(waveIdx int, status waveStatus) string {
	if status != waveStatusRunning {
		return waveStatusGlyph(status)
	}
	running := m.runningJobsInWave(waveIdx)
	if running <= 0 {
		return waveGlyphRunning
	}
	if running > 4 {
		running = 4
	}
	frame := waveGlyphRunning
	if len(spinnerFrames) > 0 {
		frame = spinnerFrames[m.frame%len(spinnerFrames)]
	}
	return strings.Repeat(frame, running)
}

func (m *uiModel) runningJobsInWave(waveIdx int) int {
	if m.parallel == nil {
		return 0
	}
	count := 0
	for i := range m.jobs {
		if m.parallel.waveOfJob(&m.jobs[i]) != waveIdx {
			continue
		}
		switch m.jobs[i].state {
		case jobRunning, jobPausing, jobRetrying:
			count++
		}
	}
	return count
}

func waveStatusGlyph(status waveStatus) string {
	switch status {
	case waveStatusRunning:
		return waveGlyphRunning
	case waveStatusMerging:
		return waveGlyphMerging
	case waveStatusConflict:
		return waveGlyphConflict
	case waveStatusMerged:
		return waveGlyphMerged
	default:
		return waveGlyphPending
	}
}

func waveStatusLabel(status waveStatus) string {
	switch status {
	case waveStatusRunning:
		return "running"
	case waveStatusMerging:
		return "merging"
	case waveStatusConflict:
		return "conflict"
	case waveStatusMerged:
		return "merged"
	default:
		return "pending"
	}
}

func waveStatusColor(status waveStatus) color.Color {
	switch status {
	case waveStatusRunning:
		return colorAccentAlt
	case waveStatusMerging:
		return colorInfo
	case waveStatusConflict:
		return colorWarning
	case waveStatusMerged:
		return colorSuccess
	default:
		return colorMuted
	}
}

// ---------------------------------------------------------------------------
// INTEGRATION pane
// ---------------------------------------------------------------------------

// renderIntegrationContent renders the INTEGRATION pane body only when parallel
// integration needs operator attention or merge context. It returns "" when the
// run is not parallel or the phase is routine running/done state.
func (m *uiModel) renderIntegrationContent(contentWidth int) string {
	if m.parallel == nil {
		return ""
	}
	p := m.parallel
	if !p.expanded() {
		return ""
	}
	lines := []string{m.renderIntegrationStatusLine(contentWidth)}
	lines = append(lines, m.renderIntegrationDetailLines(contentWidth)...)
	return strings.Join(lines, "\n")
}

func (m *uiModel) renderIntegrationStatusLine(width int) string {
	p := m.parallel
	label := stylePanelLabel.Render("INTEGRATION")
	progress := p.waveProgressString()
	used := lipgloss.Width("INTEGRATION")
	parts := []string{label}
	if progress != "" && used+1+lipgloss.Width(progress) <= width {
		parts = append(parts, m.renderWaveProgress(width))
		used += 1 + lipgloss.Width(progress)
	}
	branch := strings.TrimSpace(p.integrationBranch)
	if branch != "" {
		remaining := width - used - 1
		if remaining > 4 {
			parts = append(parts, styleDimText.Render(truncateString(branch, remaining)))
		}
	}
	return renderOwnedLineKnownOwned(width, strings.Join(parts, renderGap(1)))
}

// renderWaveProgress renders the per-wave status strip `w1✓ w2⚠ w3⏸` with each
// token colored by its wave status.
func (m *uiModel) renderWaveProgress(width int) string {
	p := m.parallel
	waveCount := p.observedWaveCount()
	tokens := make([]string, 0, waveCount)
	for i := 0; i < waveCount; i++ {
		status := p.waveStatusAt(i)
		token := fmt.Sprintf("w%d%s", i+1, waveStatusGlyph(status))
		tokens = append(tokens, lipgloss.NewStyle().Foreground(waveStatusColor(status)).Render(token))
	}
	joined := strings.Join(tokens, " ")
	if lipgloss.Width(joined) <= width {
		return joined
	}
	return truncateString(joined, max(width, 1))
}

func (m *uiModel) renderIntegrationDetailLines(width int) []string {
	p := m.parallel
	var lines []string
	if p.phase == integrationPhaseRolledBack {
		banner := lipgloss.NewStyle().Bold(true).Foreground(colorError).
			Render(truncateString("✗ ROLLED BACK · working branch untouched", max(width, 1)))
		lines = append(lines, renderOwnedLineKnownOwned(width, banner))
		return lines
	}
	if p.conflict != nil {
		lines = append(lines, m.renderConflictBanner(width))
		if filesLine := conflictFilesLine(p.conflict.files); filesLine != "" {
			lines = append(
				lines,
				renderOwnedLineKnownOwned(width, styleDimText.Render(truncateString(filesLine, max(width, 1)))),
			)
		}
		lines = append(
			lines,
			renderOwnedLineKnownOwned(width, styleMutedText.Render(truncateString("rollback armed", max(width, 1)))),
		)
		lines = append(lines, m.renderResolverLines(width)...)
		return lines
	}
	if phase := strings.TrimSpace(p.lifecyclePhase); phase != "" {
		label := strings.ReplaceAll(phase, "_", " ")
		return append(lines, renderOwnedLineKnownOwned(
			width,
			styleMutedText.Render(truncateString(label+"…", max(width, 1))),
		))
	}
	// Merging without an active conflict: a single muted progress line.
	return append(lines, renderOwnedLineKnownOwned(
		width,
		styleMutedText.Render(truncateString("merging task worktrees into integration branch", max(width, 1))),
	))
}

func (m *uiModel) renderConflictBanner(width int) string {
	c := m.parallel.conflict
	verb := "CONFLICT"
	bannerColor := colorWarning
	if c.resolving {
		verb = "RESOLVE"
	}
	task := strings.TrimSpace(c.taskID)
	if task == "" && c.taskNumber > 0 {
		task = fmt.Sprintf("task_%02d", c.taskNumber)
	}
	banner := fmt.Sprintf("⚠ %s · %s", verb, task)
	if c.attempt > 0 && c.maxAttempts > 0 {
		banner += fmt.Sprintf(" · try %d/%d", c.attempt, c.maxAttempts)
	}
	style := lipgloss.NewStyle().Bold(true).Foreground(bannerColor)
	return renderOwnedLineKnownOwned(width, style.Render(truncateString(banner, max(width, 1))))
}

func (m *uiModel) renderResolverLines(width int) []string {
	p := m.parallel
	if len(p.resolverLines) == 0 {
		if p.conflict != nil && p.conflict.resolving {
			return []string{renderOwnedLineKnownOwned(
				width,
				styleDimText.Render(truncateString("resolving…", max(width, 1))),
			)}
		}
		return nil
	}
	lines := make([]string, 0, len(p.resolverLines))
	for _, line := range p.resolverLines {
		lines = append(
			lines,
			renderOwnedLineKnownOwned(width, styleDimText.Render(truncateString(line, max(width, 1)))),
		)
	}
	return lines
}

func conflictFilesLine(files []string) string {
	cleaned := make([]string, 0, len(files))
	for _, file := range files {
		if trimmed := strings.TrimSpace(file); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return "conflicts: " + strings.Join(cleaned, ", ")
}

func (p *parallelView) waveProgressString() string {
	waveCount := p.observedWaveCount()
	tokens := make([]string, 0, waveCount)
	for i := 0; i < waveCount; i++ {
		tokens = append(tokens, fmt.Sprintf("w%d%s", i+1, waveStatusGlyph(p.waveStatusAt(i))))
	}
	return strings.Join(tokens, " ")
}

// observedWaveCount is the planned render count. The plan event seeds pending
// future waves before any task starts so the sidebar reflects the full DAG.
func (p *parallelView) observedWaveCount() int {
	if p == nil {
		return 0
	}
	return p.waveTotal
}

func (m *uiModel) integrationBorderColor() color.Color {
	if m.parallel == nil {
		return colorBorder
	}
	switch m.parallel.phase {
	case integrationPhaseConflict, integrationPhaseResolving:
		return colorWarning
	case integrationPhaseRolledBack:
		return colorError
	case integrationPhaseDone:
		return colorSuccess
	default:
		return colorBorder
	}
}
