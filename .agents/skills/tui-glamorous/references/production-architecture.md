# Production TUI Architecture: Elite Patterns for Go + Bubble Tea

Battle-tested patterns from production Bubble Tea TUIs with 10,000+ line codebases,
18+ view modes, and sub-100ms time-to-first-render. Each section has a **Problem →
Solution → Code** structure you can copy directly.

---

## Table of Contents

**Architecture (make it fast)**
- [Two-Phase Async Architecture](#two-phase-async-architecture) — instant UI + background computation
- [Immutable Snapshot Pattern](#immutable-snapshot-pattern) — zero locks in View()
- [Background Worker with File Watching](#background-worker-with-file-watching) — debounced live reload
- [Multi-View Focus State Machine](#multi-view-focus-state-machine) — key routing for 5+ views
- [tea.Batch Command Accumulation](#teabatch-command-accumulation) — idiomatic multi-cmd returns
- [Stale Message Detection](#stale-message-detection) — prevent race conditions
- [Error Recovery in Background Goroutines](#error-recovery-in-background-goroutines) — crash-proof goroutines

**Performance (make it smooth)**
- [Pre-Computed Styles for Performance](#pre-computed-styles-for-performance) — eliminate per-frame allocations
- [strings.Builder in View()](#stringsbuilder-in-view) — O(n) rendering
- [Object Pooling & Memory Efficiency](#object-pooling--memory-efficiency) — reduce GC pressure
- [Viewport Virtualization](#viewport-virtualization) — only render visible rows
- [Cached Markdown Rendering](#cached-markdown-rendering) — Glamour caching
- [Idle-Time GC Management](#idle-time-gc-management) — GC during idle, not interaction
- [Multi-Layer Caching](#multi-layer-caching) — in-memory + disk
- [Size-Adaptive Algorithm Selection](#size-adaptive-algorithm-selection) — auto-tune by dataset size
- [Data Hash Fingerprinting](#data-hash-fingerprinting) — deterministic cache keys

**Layout & Theming (make it beautiful)**
- [Adaptive Layout Engine](#adaptive-layout-engine) — responsive breakpoints
- [Semantic Theming System](#semantic-theming-system) — AdaptiveColor + WCAG AA
- [Data Visualization: Sparklines & Heatmaps](#data-visualization-sparklines--heatmaps) — Tufte-inspired terminal viz
- [Custom ASCII/Unicode Graph Renderer](#custom-asciiunicode-graph-renderer) — DAGs in the terminal
- [Color-Coded Borders Encoding State](#color-coded-borders-encoding-state) — borders that mean something
- [Status Bar with Dynamic Segments](#status-bar-with-dynamic-segments) — flexible left/right layout
- [Breadcrumb Navigation](#breadcrumb-navigation) — path indicator for nested views

**Interaction (make it feel right)**
- [Composite FilterValue for Fuzzy Search](#composite-filtervalue-for-zero-allocation-fuzzy-search) — search all fields at once
- [Deterministic Stable Sorting](#deterministic-stable-sorting) — prevent list shuffling
- [Debounced Search](#debounced-search) — smooth typing
- [Vim Key Combo Tracking](#vim-key-combo-tracking) — gg, G, multi-key sequences
- [Focus Restoration](#focus-restoration) — return to exact position after overlay
- [Mouse Handling by Focus](#mouse-handling-by-focus) — per-component wheel routing
- [Smart Editor Dispatch](#smart-editor-dispatch) — suspend for vim, background for VS Code
- [Terminal Editor Suspension](#terminal-editor-suspension) — tea.ExecProcess pattern
- [Clipboard Integration](#clipboard-integration) — copy with toast feedback

**Components (make it feature-rich)**
- [Rich Multi-Line List Delegates](#rich-multi-line-list-delegates) — 4-line cards with metadata
- [Inline Expansion](#inline-expansion) — toggle detail in-place
- [Kanban Board with Swimlane Modes](#kanban-board-with-swimlane-modes) — pre-computed columns
- [Dashboard Panels with Drill-Down](#dashboard-panels-with-drill-down) — metric grids
- [Flattened Tree Navigation](#flattened-tree-navigation) — collapsible tree with h/l
- [Multi-Tier Help System](#multi-tier-help-system) — ? + ` + ; help levels

**Resilience (make it production-ready)**
- [Persistent UI State](#persistent-ui-state) — save/restore expand/collapse across sessions
- [Environment Variable Preferences](#environment-variable-preferences) — runtime tuning
- [Graceful Degradation](#graceful-degradation) — hide missing optional features
- [Production Pre-Flight Checklist](#production-pre-flight-checklist) — must-have + polish items

---

## Two-Phase Async Architecture

> **Use when:** your TUI does expensive work at startup (analysis, indexing, network calls) and you need the UI interactive immediately.

**The Problem:** Expensive computations block the UI. Users stare at a blank screen.

**The Solution:** Split work into two phases:
- **Phase 1 (instant, <50ms):** Cheap metrics computed on the main thread before UI renders
- **Phase 2 (background goroutine):** Expensive metrics computed async, UI refreshes when ready

```go
type Analyzer struct {
    mu         sync.RWMutex
    phase2Done chan struct{}

    // Phase 1 (instant) — populated by BuildGraph()
    inDegree       map[string]int
    outDegree      map[string]int
    topoOrder      []string
    density        float64

    // Phase 2 (async) — populated by computePhase2()
    pageRank       map[string]float64
    betweenness    map[string]float64
    eigenvector    map[string]float64
}

func (a *Analyzer) AnalyzeAsync(issues []Issue) {
    // Phase 1: instant metrics (blocks, completes in <50ms)
    a.buildGraph(issues)
    a.computeDegrees()
    a.computeTopoSort()
    a.computeDensity()

    // Phase 2: expensive metrics (background goroutine)
    a.phase2Done = make(chan struct{})
    go func() {
        defer close(a.phase2Done)
        defer func() {
            if r := recover(); r != nil {
                log.Printf("phase2 panic: %v", r)
            }
        }()
        a.computePhase2()
    }()
}

func (a *Analyzer) computePhase2() {
    // Compute locally, then atomically assign under lock
    pr := computePageRank(a.graph)
    bw := computeBetweenness(a.graph)
    ev := computeEigenvector(a.graph)

    a.mu.Lock()
    a.pageRank = pr
    a.betweenness = bw
    a.eigenvector = ev
    a.mu.Unlock()
}

func (a *Analyzer) IsPhase2Ready() bool {
    select {
    case <-a.phase2Done:
        return true
    default:
        return false
    }
}
```

**In the TUI Model:**
```go
// Custom message sent when Phase 2 completes
type Phase2ReadyMsg struct{ DataHash string }

// In Init(), start Phase 2 and poll for completion
func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        pollPhase2(m.analyzer),
    )
}

func pollPhase2(a *Analyzer) tea.Cmd {
    return func() tea.Msg {
        <-a.phase2Done // blocks until ready
        return Phase2ReadyMsg{DataHash: a.Hash()}
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case Phase2ReadyMsg:
        // Refresh views that depend on Phase 2 metrics
        m.rebuildInsights()
        m.rebuildGraphLayout()
        return m, nil
    }
    // ... UI is already interactive from Phase 1
}
```

**Why this matters:** The UI is interactive in <100ms. Users can browse, search, and navigate
while expensive computations run in the background. When Phase 2 completes, views silently
refresh with richer data.

---

## Immutable Snapshot Pattern

> **Use when:** a background thread produces data that the UI thread reads, and you're seeing jank from mutex contention in View().

**The Problem:** Mutexes in the render path cause jank. Stale data from concurrent reads causes visual glitches.

**The Solution:** Build an immutable `DataSnapshot` in the background, then atomically swap a pointer. Zero lock contention in the render path.

```go
// DataSnapshot is immutable once built — no mutexes needed to read
type DataSnapshot struct {
    Issues       []Issue
    BoardState   BoardState      // Pre-computed kanban columns
    GraphLayout  GraphLayout     // Pre-computed graph metrics & rankings
    TreeNodes    []TreeNode      // Pre-computed hierarchical tree
    TriageScores map[string]float64
    DataHash     string          // SHA256 fingerprint for cache invalidation
    BuiltAt      time.Time
}

// BoardState pre-computes all swimlane groupings for O(1) mode switching
type BoardState struct {
    ByStatus   [4][]Issue  // Open | In Progress | Blocked | Closed
    ByPriority [4][]Issue  // P0 | P1 | P2 | P3+
    ByType     [4][]Issue  // Bug | Feature | Task | Epic
}

// GraphLayout pre-computes all metric rankings
type GraphLayout struct {
    Blockers, Dependents map[string][]string
    SortedIDs            []string
    RankPageRank         map[string]int
    RankBetweenness      map[string]int
    RankCriticalPath     map[string]int
    // ... 8 different ranking dimensions
}

// SnapshotBuilder constructs snapshots off-thread
type SnapshotBuilder struct {
    issues  []Issue
    analyzer *Analyzer
}

func (b *SnapshotBuilder) Build() *DataSnapshot {
    snap := &DataSnapshot{
        Issues:      b.issues,
        DataHash:    computeHash(b.issues),
        BuiltAt:     time.Now(),
    }
    snap.BoardState = b.buildBoardState()
    snap.GraphLayout = b.buildGraphLayout()
    snap.TreeNodes = b.buildTree()
    snap.TriageScores = b.computeScores()
    return snap
}

// In the model: atomic pointer swap
type model struct {
    snapshot *DataSnapshot  // Read freely in View() — never locked
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case snapshotReadyMsg:
        m.snapshot = msg.snapshot  // Atomic swap — old snapshot becomes garbage
        return m, nil
}
```

**Key insight:** Pre-compute ALL rendering data in the snapshot. Board columns, graph rankings,
tree structures — everything. The View() function only reads; it never computes.

---

## Background Worker with File Watching

> **Use when:** your TUI watches a file/database for changes and needs live reload without UI stutter.

**The Problem:** The TUI needs to react to file changes (live reload) without blocking the UI
or processing every rapid-fire filesystem event.

**The Solution:** A background worker with debouncing, coalescing, and watchdog recovery.

```go
type WorkerState int
const (
    WorkerIdle WorkerState = iota
    WorkerProcessing
    WorkerStopped
)

type BackgroundWorker struct {
    state       WorkerState
    debounceMs  int           // Default: 200ms, configurable via BV_DEBOUNCE_MS
    pending     bool          // Coalescing flag: another change arrived during processing
    heartbeat   time.Time     // Watchdog: last activity timestamp
}

// WorkerPollTickMsg drives the spinner animation
type WorkerPollTickMsg struct{}

func (w *BackgroundWorker) HandleFileChange() tea.Cmd {
    if w.state == WorkerProcessing {
        w.pending = true  // Coalesce: will re-process after current build
        return nil
    }

    w.state = WorkerProcessing
    return tea.Tick(time.Duration(w.debounceMs)*time.Millisecond, func(t time.Time) tea.Msg {
        return debounceExpiredMsg{}
    })
}

// After debounce expires, rebuild the snapshot
func (w *BackgroundWorker) startBuild(issues []Issue) tea.Cmd {
    return func() tea.Msg {
        builder := &SnapshotBuilder{issues: issues}
        snap := builder.Build()
        return snapshotReadyMsg{snapshot: snap}
    }
}

// Watchdog: detect stuck workers
func (w *BackgroundWorker) checkHealth() {
    if w.state == WorkerProcessing && time.Since(w.heartbeat) > 30*time.Second {
        log.Warn("worker appears stuck, recovering")
        w.state = WorkerIdle
    }
}

// Spinner only animates when worker is processing
func (m model) View() string {
    if m.worker.state == WorkerProcessing {
        return m.spinnerFrames[m.spinnerIdx] + " Processing..."
    }
    // ...
}
```

**File watching setup with fsnotify:**
```go
import "github.com/fsnotify/fsnotify"

func watchFile(path string, onChange func()) {
    watcher, _ := fsnotify.NewWatcher()
    defer watcher.Close()

    watcher.Add(filepath.Dir(path))

    for {
        select {
        case event := <-watcher.Events:
            if event.Name == path && (event.Op&fsnotify.Write != 0) {
                onChange()
            }
        case err := <-watcher.Errors:
            log.Printf("watcher error: %v", err)
        }
    }
}
```

**Custom braille spinner (10 frames, only advances during WorkerProcessing):**
```go
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
```

---

## Adaptive Layout Engine

> **Use when:** your TUI needs to work on terminals from 80 cols to 300+ cols, or you want split-pane layouts that adapt.

**The Problem:** TUIs that hardcode column widths break on different terminal sizes. Users
have monitors from 80 cols to 300+ cols.

**The Solution:** Responsive breakpoints that progressively reveal content.

```go
const (
    MobileThreshold    = 80   // List only, no detail pane
    SplitViewThreshold = 100  // List + detail side by side
    WideViewThreshold  = 140  // Extra columns (sparklines, labels)
    UltraWideThreshold = 180  // Full dashboard with all panels
)

func (m model) View() string {
    w := m.width

    switch {
    case w < SplitViewThreshold:
        // Mobile: list takes 100% width
        return m.renderListOnly(w)

    case w < WideViewThreshold:
        // Split: 40% list + 60% detail
        listW := int(float64(w) * m.splitRatio)
        detailW := w - listW - 1  // -1 for divider
        left := m.renderList(listW)
        right := m.renderDetail(detailW)
        return lipgloss.JoinHorizontal(lipgloss.Top, left, "│", right)

    case w < UltraWideThreshold:
        // Wide: split view + extra list columns
        return m.renderWideView(w)

    default:
        // Ultra-wide: full dashboard
        return m.renderUltraWide(w)
    }
}

// Dynamic column visibility based on available width
func (m model) renderListRow(issue Issue, availWidth int) string {
    // Always show: status icon, ID, title
    row := renderStatus(issue) + " " + issue.ID + " " + issue.Title

    // Show age if width permits
    if availWidth > 90 {
        row += "  " + renderAge(issue.CreatedAt)
    }

    // Show assignee if width permits
    if availWidth > 110 {
        row += "  " + renderAssignee(issue.Assignee)
    }

    // Show sparkline if width permits (ultra-wide)
    if availWidth > 140 {
        row += "  " + renderSparkline(issue.GraphScore, 5)
    }

    // Show labels if width permits
    if availWidth > 160 {
        row += "  " + renderLabels(issue.Labels)
    }

    return row
}

// Adjustable split ratio with < and > keys
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.KeyMsg:
        switch msg.String() {
        case "<":
            m.splitRatio = max(0.2, m.splitRatio-0.05)
        case ">":
            m.splitRatio = min(0.8, m.splitRatio+0.05)
        }
}

// CRITICAL: Account for borders and padding to prevent wrapping
func contentWidth(totalWidth int) int {
    return totalWidth - 2 /* border */ - 2 /* padding */
}
```

**Multi-pane responsive layouts (e.g., 3-pane history view):**
```go
func (m model) renderHistoryView() string {
    switch {
    case m.width < 100:
        // Single pane: list with inline details
        return m.renderHistoryCompact()
    case m.width < 160:
        // Two panes: list + detail
        return m.renderHistoryTwoPanes()
    default:
        // Three panes: list + timeline + detail
        return m.renderHistoryThreePanes()
    }
}
```

---

## Multi-View Focus State Machine

> **Use when:** your TUI has 3+ views/screens and key routing is becoming a tangled mess of conditionals.

**The Problem:** Complex TUIs have many views (list, board, graph, insights, tree, etc.) plus
modal overlays. Key handling must route to the right view.

**The Solution:** An explicit focus enum with modal priority layering.

```go
type focus int
const (
    focusList focus = iota
    focusDetail
    focusBoard
    focusGraph
    focusTree
    focusInsights
    focusHistory
    focusActionable
    focusFlowMatrix
    focusAttention
    focusSprint
    focusTutorial
    focusRecipePicker
    focusLabelPicker
    focusRepoPicker
    // ... 18+ states
)

type model struct {
    focus           focus
    focusBeforeHelp focus    // For restoring after overlay dismiss
    showHelp        bool
    showQuitConfirm bool
    showAlerts      bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // MODAL PRIORITY: Handle overlays first (highest to lowest priority)
    if m.showQuitConfirm {
        return m.handleQuitConfirmKeys(msg)
    }
    if m.showAlerts {
        return m.handleAlertKeys(msg)
    }
    if m.showHelp {
        return m.handleHelpKeys(msg)
    }

    // GLOBAL KEYS: Always available regardless of focus
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c":
            m.showQuitConfirm = true
            return m, nil
        case "?":
            m.focusBeforeHelp = m.focus
            m.showHelp = true
            return m, nil
        case "b":
            m.focus = focusBoard
            return m, nil
        case "g":
            m.focus = focusGraph
            return m, nil
        case "i":
            m.focus = focusInsights
            return m, nil
        }
    }

    // FOCUS-SPECIFIC: Delegate to current view handler
    switch m.focus {
    case focusList:
        return m.handleListKeys(msg)
    case focusBoard:
        return m.handleBoardKeys(msg)
    case focusGraph:
        return m.handleGraphKeys(msg)
    case focusInsights:
        return m.handleInsightsKeys(msg)
    case focusHistory:
        return m.handleHistoryKeys(msg)
    case focusTree:
        return m.handleTreeKeys(msg)
    }
    return m, nil
}
```

**View rendering with modal overlay priority:**
```go
func (m model) View() string {
    // Base content: render current view
    var base string
    switch m.focus {
    case focusList:
        base = m.renderListView()
    case focusBoard:
        base = m.renderBoardView()
    case focusGraph:
        base = m.renderGraphView()
    // ...
    }

    // Modal overlays (last rendered = on top)
    if m.showHelp {
        base = m.renderHelpOverlay(base)
    }
    if m.showAlerts {
        base = m.renderAlertOverlay(base)
    }
    if m.showQuitConfirm {
        base = m.renderQuitConfirm(base)
    }

    return base
}
```

---

## Semantic Theming System

> **Use when:** you need colors that work on both light and dark terminals, or you're hardcoding hex values and it looks wrong on some setups.

**The Problem:** Hardcoded colors look wrong on light terminals, break on 16-color terminals,
and are inconsistent across views.

**The Solution:** Semantic color names with adaptive light/dark variants, WCAG-compliant
contrast, terminal capability detection, and design tokens.

```go
type Theme struct {
    // Semantic status colors
    Open       lipgloss.AdaptiveColor
    InProgress lipgloss.AdaptiveColor
    Blocked    lipgloss.AdaptiveColor
    Closed     lipgloss.AdaptiveColor

    // Semantic type colors
    Bug        lipgloss.AdaptiveColor
    Feature    lipgloss.AdaptiveColor
    Task       lipgloss.AdaptiveColor
    Epic       lipgloss.AdaptiveColor

    // UI chrome
    Primary    lipgloss.AdaptiveColor
    Secondary  lipgloss.AdaptiveColor
    Subtext    lipgloss.AdaptiveColor
    Border     lipgloss.AdaptiveColor
    Highlight  lipgloss.AdaptiveColor
    Muted      lipgloss.AdaptiveColor
}

// Dracula-inspired palette with WCAG AA compliance (contrast ≥ 4.5:1)
var DefaultTheme = Theme{
    Open:       lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#50fa7b"},
    InProgress: lipgloss.AdaptiveColor{Light: "#e65100", Dark: "#ffb86c"},
    Blocked:    lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff5555"},
    Closed:     lipgloss.AdaptiveColor{Light: "#616161", Dark: "#6272a4"},

    Bug:        lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff5555"},
    Feature:    lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#8be9fd"},
    Task:       lipgloss.AdaptiveColor{Light: "#4527a0", Dark: "#bd93f9"},
    Epic:       lipgloss.AdaptiveColor{Light: "#ad1457", Dark: "#ff79c6"},

    Primary:    lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#f8f8f2"},
    Border:     lipgloss.AdaptiveColor{Light: "#bdbdbd", Dark: "#44475a"},
    Highlight:  lipgloss.AdaptiveColor{Light: "#7c4dff", Dark: "#bd93f9"},
    Muted:      lipgloss.AdaptiveColor{Light: "#757575", Dark: "#6272a4"},
}

// Design token system for consistent spacing
const (
    SpaceXS = 1
    SpaceSM = 2
    SpaceMD = 3
    SpaceLG = 4
    SpaceXL = 6
)

// Terminal capability detection
func detectCapability() ColorMode {
    term := os.Getenv("TERM")
    colorterm := os.Getenv("COLORTERM")

    switch {
    case colorterm == "truecolor" || colorterm == "24bit":
        return TrueColor
    case strings.Contains(term, "256color"):
        return ANSI256
    default:
        return ANSI16
    }
}

// Theme override for terminal detection issues
// BV_THEME=dark forces dark mode
func resolveTheme() string {
    if t := os.Getenv("BV_THEME"); t != "" {
        return t
    }
    if term.HasDarkBackground() {
        return "dark"
    }
    return "light"
}
```

**Status indicators using Nerd Font glyphs + semantic colors:**
```go
func statusIcon(status string, theme Theme) string {
    switch status {
    case "open":
        return lipgloss.NewStyle().Foreground(theme.Open).Render("●")
    case "in_progress":
        return lipgloss.NewStyle().Foreground(theme.InProgress).Render("◐")
    case "blocked":
        return lipgloss.NewStyle().Foreground(theme.Blocked).Render("⚠")
    case "closed":
        return lipgloss.NewStyle().Foreground(theme.Closed).Render("○")
    }
    return "?"
}

func typeIcon(issueType string) string {
    switch issueType {
    case "bug":     return "🐛"
    case "feature": return "✨"
    case "task":    return "📝"
    case "epic":    return "🎯"
    case "chore":   return "🔧"
    }
    return "•"
}
```

---

## Data Visualization: Sparklines & Heatmaps

> **Use when:** you want to show numeric values, scores, or intensities visually in a list or table without consuming extra columns.

Tufte-inspired high-density data visualization for the terminal.

### Unicode Sparklines

```go
// 8-level unicode bar chart characters
var sparkBlocks = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline renders a 0.0-1.0 value as a unicode bar of given width
func RenderSparkline(value float64, width int) string {
    if value < 0 { value = 0 }
    if value > 1 { value = 1 }

    fullBlocks := int(value * float64(width))
    remainder := value*float64(width) - float64(fullBlocks)

    var buf strings.Builder
    for i := 0; i < width; i++ {
        if i < fullBlocks {
            buf.WriteRune('█')
        } else if i == fullBlocks {
            idx := int(remainder * float64(len(sparkBlocks)-1))
            buf.WriteRune(sparkBlocks[idx])
        } else {
            buf.WriteRune(' ')
        }
    }
    return buf.String()
}

// Usage in list view (ultra-wide mode):
// ▇▅▂   0.73  AUTH-001  Implement OAuth2 flow
// ████   0.95  CORE-123 Database schema migration
// ▃     0.12  DOCS-001 Update API docs
```

### Perceptual Heatmap Gradient

```go
// 8-color perceptually uniform gradient
// Maps 0.0-1.0 intensity to semantic colors
func GetHeatmapColor(intensity float64) lipgloss.Color {
    colors := []string{
        "#4a4a5a", // 0.0-0.125: Gray (low)
        "#2a3a6a", // 0.125-0.25: Dark blue
        "#3a5a9a", // 0.25-0.375: Blue
        "#5a8aba", // 0.375-0.5: Light blue
        "#ba9a3a", // 0.5-0.625: Gold (mid-high)
        "#da7a4a", // 0.625-0.75: Coral
        "#ea5a6a", // 0.75-0.875: Salmon
        "#fa3a8a", // 0.875-1.0: Hot pink (peak)
    }

    idx := int(intensity * float64(len(colors)-1))
    if idx >= len(colors) { idx = len(colors) - 1 }
    if idx < 0 { idx = 0 }
    return lipgloss.Color(colors[idx])
}

// Heatmap cell with background + contrasting foreground
func renderHeatmapCell(value float64, label string) string {
    bg := GetHeatmapColor(value)
    fg := lipgloss.Color("#ffffff")
    if value < 0.5 {
        fg = lipgloss.Color("#cccccc")
    }
    return lipgloss.NewStyle().
        Background(bg).
        Foreground(fg).
        Padding(0, 1).
        Render(label)
}
```

### Age Color Coding

```go
func ageColor(created time.Time) lipgloss.AdaptiveColor {
    days := int(time.Since(created).Hours() / 24)
    switch {
    case days < 7:
        return lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#50fa7b"}  // Fresh (green)
    case days < 30:
        return lipgloss.AdaptiveColor{Light: "#e65100", Dark: "#ffb86c"}  // Aging (yellow)
    default:
        return lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff5555"}  // Stale (red)
    }
}
```

---

## Custom ASCII/Unicode Graph Renderer

> **Use when:** you need to visualize a DAG, tree, or network graph directly in the terminal without Sixel or external renderers.

For dependency visualization without external tools or graphical protocols (Sixel).

```go
type Canvas struct {
    cells  [][]rune
    styles [][]*lipgloss.Style
    width  int
    height int
}

func NewCanvas(w, h int) *Canvas {
    c := &Canvas{width: w, height: h}
    c.cells = make([][]rune, h)
    c.styles = make([][]*lipgloss.Style, h)
    for i := range c.cells {
        c.cells[i] = make([]rune, w)
        c.styles[i] = make([]*lipgloss.Style, w)
        for j := range c.cells[i] {
            c.cells[i][j] = ' '
        }
    }
    return c
}

func (c *Canvas) Set(x, y int, r rune, s *lipgloss.Style) {
    if x >= 0 && x < c.width && y >= 0 && y < c.height {
        c.cells[y][x] = r
        c.styles[y][x] = s
    }
}

// Manhattan routing: orthogonal edges with Unicode corners
// Characters: ╭ ─ ╮ │ ╰ ╯ ┬ ┴ ├ ┤ ┼
func (c *Canvas) DrawEdge(x1, y1, x2, y2 int, style *lipgloss.Style) {
    // Horizontal segment
    midY := (y1 + y2) / 2
    for x := min(x1, x2); x <= max(x1, x2); x++ {
        c.Set(x, midY, '─', style)
    }
    // Vertical segments
    for y := min(y1, midY); y <= max(y1, midY); y++ {
        c.Set(x1, y, '│', style)
    }
    for y := min(midY, y2); y <= max(midY, y2); y++ {
        c.Set(x2, y, '│', style)
    }
    // Corners
    if y1 < midY {
        c.Set(x1, midY, '╰', style)
    } else {
        c.Set(x1, midY, '╭', style)
    }
    if y2 > midY {
        c.Set(x2, midY, '╮', style)
    } else {
        c.Set(x2, midY, '╯', style)
    }
}

// Topological layering: nodes arranged by dependency depth
func layoutNodes(issues []Issue, deps map[string][]string) [][]string {
    depths := computeDepths(issues, deps) // Longest path from roots
    maxDepth := 0
    for _, d := range depths {
        if d > maxDepth { maxDepth = d }
    }

    layers := make([][]string, maxDepth+1)
    for id, depth := range depths {
        layers[depth] = append(layers[depth], id)
    }
    return layers // Dependencies flow downward
}
```

**Key design decisions:**
- Canvas abstraction allows "pixel-level" terminal drawing
- Manhattan routing minimizes visual noise (no diagonal lines)
- Topological layering ensures dependencies always flow top-to-bottom
- Viewport clips rendering to visible area, panning with h/j/k/l

---

## Pre-Computed Styles for Performance

> **Use when:** profiling shows high allocation counts in your render path, or your list has 50+ visible items.

**The Problem:** Creating lipgloss styles on every frame causes thousands of allocations
per render cycle. For a list of 100 items, that's 1600+ allocations per frame.

**The Solution:** Pre-allocate styles at startup, reuse them during rendering.

```go
// Pre-computed at startup, not per-frame
type DelegateStyles struct {
    NormalTitle      lipgloss.Style
    NormalDesc       lipgloss.Style
    SelectedTitle    lipgloss.Style
    SelectedDesc     lipgloss.Style
    StatusOpen       lipgloss.Style
    StatusBlocked    lipgloss.Style
    StatusClosed     lipgloss.Style
    PriorityP0       lipgloss.Style
    PriorityP1       lipgloss.Style
    PriorityP2       lipgloss.Style
    PriorityDefault  lipgloss.Style
    AgeFresh         lipgloss.Style
    AgeAging         lipgloss.Style
    AgeStale         lipgloss.Style
    SparklineStyle   lipgloss.Style
    MutedText        lipgloss.Style
}

func NewDelegateStyles(theme Theme) DelegateStyles {
    return DelegateStyles{
        NormalTitle:   lipgloss.NewStyle().Foreground(theme.Primary),
        SelectedTitle: lipgloss.NewStyle().Foreground(theme.Highlight).Bold(true),
        StatusOpen:    lipgloss.NewStyle().Foreground(theme.Open),
        StatusBlocked: lipgloss.NewStyle().Foreground(theme.Blocked),
        // ... 16 pre-allocated styles
    }
}

// In the model: allocated once
type model struct {
    delegateStyles DelegateStyles  // Created in initialModel()
}

// In rendering: lookup by enum, zero allocation
func (m model) renderItem(issue Issue) string {
    style := m.delegateStyles.NormalTitle
    if issue.Selected {
        style = m.delegateStyles.SelectedTitle
    }
    return style.Render(issue.Title)
}
```

**Impact:** Reduces allocations from ~16 per visible item per frame to near-zero. On a list
of 50 visible items at 60fps, that's 48,000 fewer allocations per second.

---

## Object Pooling & Memory Efficiency

> **Use when:** your TUI handles 1000+ items and GC pauses are noticeable, or memory usage is climbing.

For large datasets (1000+ items), reduce GC pressure with `sync.Pool`.

```go
type IssuePool struct {
    pool sync.Pool
}

func NewIssuePool() *IssuePool {
    return &IssuePool{
        pool: sync.Pool{
            New: func() any {
                return &Issue{
                    Dependencies: make([]*Dependency, 0, 8),  // Pre-allocate slices
                    Comments:     make([]*Comment, 0, 4),
                    Labels:       make([]string, 0, 8),
                }
            },
        },
    }
}

func (p *IssuePool) Get() *Issue {
    return p.pool.Get().(*Issue)
}

func (p *IssuePool) Put(issue *Issue) {
    // Clear references to allow GC of pointed-to objects
    issue.Dependencies = issue.Dependencies[:0]
    issue.Comments = issue.Comments[:0]
    issue.Labels = issue.Labels[:0]
    issue.Title = ""
    issue.Description = ""
    p.pool.Put(issue)
}

// Batch return on snapshot replacement
func (p *IssuePool) ReturnSlice(issues []*Issue) {
    for _, issue := range issues {
        p.Put(issue)
    }
}
```

**Heuristic pre-allocation for JSONL parsing:**
```go
func estimateIssueCount(fileSize int64) int {
    avg := int(fileSize / 2048)  // ~2KB per issue average
    if avg < 64 { avg = 64 }
    if avg > 200000 { avg = 200000 }
    return avg
}

issues := make([]Issue, 0, estimateIssueCount(stat.Size()))
```

---

## Viewport Virtualization

> **Use when:** your list has 100+ items and you're rendering all of them even though only 20-40 are visible.

Only render the rows currently visible in the terminal window.

```go
func (m model) renderList(width int) string {
    visibleRows := m.height - 4  // -4 for header + footer

    start := m.scrollOffset
    end := start + visibleRows
    if end > len(m.items) {
        end = len(m.items)
    }

    var buf strings.Builder
    buf.WriteString(m.renderHeader(width))

    for i := start; i < end; i++ {
        isSelected := i == m.cursor
        buf.WriteString(m.renderRow(m.items[i], isSelected, width))
        buf.WriteRune('\n')
    }

    buf.WriteString(m.renderFooter(width))
    return buf.String()
}

// Scroll offset management
func (m *model) ensureCursorVisible() {
    visibleRows := m.height - 4
    if m.cursor < m.scrollOffset {
        m.scrollOffset = m.cursor
    }
    if m.cursor >= m.scrollOffset+visibleRows {
        m.scrollOffset = m.cursor - visibleRows + 1
    }
}
```

---

## Persistent UI State

> **Use when:** your TUI has user-configurable state (tree expand/collapse, view preferences, sort mode) that should survive restart.

Save and restore UI state across sessions.

```go
// Tree expand/collapse state persisted to JSON
type TreeState struct {
    Version  int             `json:"version"`
    Expanded map[string]bool `json:"expanded"`
}

func (m *model) saveTreeState() {
    state := TreeState{
        Version:  1,
        Expanded: m.treeExpanded,
    }
    data, _ := json.MarshalIndent(state, "", "  ")
    os.WriteFile(".beads/tree-state.json", data, 0644)
}

func (m *model) loadTreeState() {
    data, err := os.ReadFile(".beads/tree-state.json")
    if err != nil {
        return  // Graceful: use defaults if file missing
    }

    var state TreeState
    if err := json.Unmarshal(data, &state); err != nil {
        return  // Graceful: corrupted file = use defaults
    }

    if state.Version != 1 {
        return  // Future migration path
    }

    m.treeExpanded = state.Expanded
}

// Default: expanded for depth < 2, collapsed otherwise
func (m *model) isExpanded(id string, depth int) bool {
    if v, ok := m.treeExpanded[id]; ok {
        return v  // Explicit user choice
    }
    return depth < 2  // Default
}
```

---

## Smart Editor Dispatch

> **Use when:** your TUI has an "open in editor" feature and you need to handle vim (suspends TUI) vs VS Code (background) differently.

Suspend the TUI for terminal editors, launch GUI editors in the background.

```go
func classifyEditor(editor string) EditorKind {
    base := filepath.Base(strings.Fields(editor)[0])
    switch base {
    case "vim", "nvim", "vi", "nano", "micro", "helix", "emacs":
        return EditorTerminal
    case "code", "subl", "atom", "gedit", "kate":
        return EditorGUI
    case "sh", "bash", "zsh", "python":
        return EditorForbidden  // Don't run interpreters
    default:
        return EditorGUI  // Safe default: launch in background
    }
}

func (m model) openInEditor(issue Issue) tea.Cmd {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"
    }

    kind := classifyEditor(editor)

    // Write issue to temp file as YAML frontmatter + markdown body
    tmpFile := writeIssueTempFile(issue)

    switch kind {
    case EditorTerminal:
        // Suspend TUI, run editor in foreground
        c := exec.Command(editor, tmpFile)
        return tea.ExecProcess(c, func(err error) tea.Msg {
            return editorExitMsg{path: tmpFile, err: err}
        })

    case EditorGUI:
        // Launch in background, don't suspend TUI
        exec.Command(editor, tmpFile).Start()
        return nil

    case EditorForbidden:
        return func() tea.Msg {
            return errMsg{err: fmt.Errorf("refusing to run %s as editor", editor)}
        }
    }
    return nil
}

type editorExitMsg struct {
    path string
    err  error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case editorExitMsg:
        if msg.err != nil {
            m.err = msg.err
            return m, nil
        }
        // Parse changes from the temp file
        changes := parseIssueFrontmatter(msg.path)
        return m, applyChanges(changes)
}
```

---

## Focus Restoration

> **Use when:** pressing `?` for help or opening a modal loses the user's position, and Escape doesn't return them to where they were.

Save focus state before showing overlays, restore exactly on dismiss.

```go
type model struct {
    focus           focus
    focusBeforeHelp focus
}

func (m *model) showHelp() {
    m.focusBeforeHelp = m.focus
    m.showHelp = true
}

func (m *model) restoreFocusFromHelp() {
    m.showHelp = false
    // Intelligently restore to the exact previous state
    switch m.focusBeforeHelp {
    case focusDetail, focusList:
        m.focus = m.focusBeforeHelp
    case focusGraph:
        m.focus = focusGraph
    case focusBoard:
        m.focus = focusBoard
    case focusTree:
        m.focus = focusTree
    default:
        m.focus = focusList  // Safe fallback
    }
}
```

---

## Stale Message Detection

> **Use when:** you have async operations (Phase 2, file reload) and old results sometimes overwrite newer data.

Prevent race conditions from old async operations completing after data has reloaded.

```go
type Phase2ReadyMsg struct {
    DataHash string  // Hash of the data that was analyzed
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case Phase2ReadyMsg:
        // Only apply if the data hasn't changed since analysis started
        if msg.DataHash != m.snapshot.DataHash {
            // Data reloaded during analysis — discard stale results
            return m, nil
        }
        m.applyPhase2Results()
        return m, nil
}
```

---

## Debounced Search

> **Use when:** search/filter computation is expensive (semantic search, FTS5, or re-sorting 1000+ items) and typing feels laggy.

Prevent expensive search operations from firing on every keystroke.

```go
type semanticDebounceTickMsg struct {
    term string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.KeyMsg:
        if m.searchActive {
            m.pendingSearchTerm = m.searchInput.Value()
            // Start 150ms debounce timer
            return m, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
                return semanticDebounceTickMsg{term: m.pendingSearchTerm}
            })
        }

    case semanticDebounceTickMsg:
        // Only execute if the term hasn't changed since the timer started
        if msg.term == m.pendingSearchTerm && msg.term != m.lastSearchedTerm {
            m.lastSearchedTerm = msg.term
            return m, m.executeSearch(msg.term)
        }
}
```

---

## Multi-Tier Help System

> **Use when:** you want users to be able to learn your TUI without leaving it — from a quick cheat sheet to a full tutorial.

Three complementary help mechanisms for different user needs.

```go
// Tier 1: Quick Reference (?) — compact overlay, context-specific
func (m model) renderQuickHelp() string {
    keys := getKeysForFocus(m.focus)
    return renderKeyTable(keys, 60, 20)  // 60 chars wide, 20 lines
}

// Tier 2: Full Tutorial (`) — multi-page walkthrough with rich content
type TutorialPage struct {
    Title    string
    Content  []TutorialComponent
    Context  []focus  // Only show when these views are active
}

type TutorialComponent interface {
    Render(width int) string
}

type Section struct{ text string }
type KeyTable struct{ keys []KeyDesc }
type Tip struct{ text string }
type Warning struct{ text string }
type CodeBlock struct{ code, lang string }

// Tier 3: Shortcuts Sidebar (;) — persistent, always-visible
func (m model) renderShortcutsSidebar() string {
    // Fixed 34-char width on right edge
    shortcuts := getContextShortcuts(m.focus)
    sidebar := lipgloss.NewStyle().
        Width(34).
        Height(m.height).
        Border(lipgloss.NormalBorder(), false, false, false, true).
        BorderForeground(m.theme.Border).
        Render(renderShortcutList(shortcuts))
    return sidebar
}

// Sidebar scrollable with Ctrl+J/K
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+j":
            if m.showSidebar { m.sidebarScroll++ }
        case "ctrl+k":
            if m.showSidebar && m.sidebarScroll > 0 { m.sidebarScroll-- }
        }
}
```

---

## Kanban Board with Swimlane Modes

> **Use when:** you're building a board/column view and need to group items by status, priority, or type with instant switching.

Pre-compute all groupings for instant mode switching.

```go
type SwimLaneMode int
const (
    SwimByStatus   SwimLaneMode = iota  // Open | In Progress | Blocked | Closed
    SwimByPriority                       // P0 | P1 | P2 | P3+
    SwimByType                           // Bug | Feature | Task | Epic
)

// Pre-computed in BackgroundWorker for O(1) switching
type BoardState struct {
    ByStatus   [4][]Issue
    ByPriority [4][]Issue
    ByType     [4][]Issue
}

func (m model) currentColumns() [4][]Issue {
    switch m.swimMode {
    case SwimByStatus:   return m.snapshot.BoardState.ByStatus
    case SwimByPriority: return m.snapshot.BoardState.ByPriority
    case SwimByType:     return m.snapshot.BoardState.ByType
    }
    return m.snapshot.BoardState.ByStatus
}

// Card border colors encode dependency status
func cardBorderColor(issue Issue) lipgloss.Color {
    if issue.IsBlocked()    { return "#ff5555" }  // Red: has unresolved deps
    if issue.BlocksOthers() { return "#f1fa8c" }  // Yellow: completing unblocks others
    if issue.IsReady()      { return "#50fa7b" }  // Green: ready to work
    return "#6272a4"                               // Default: normal
}

// Column header with aggregate statistics
func renderColumnHeader(title string, items []Issue) string {
    p0Count := countP0P1(items)
    blockedCount := countBlocked(items)
    header := fmt.Sprintf("%s (%d)", title, len(items))
    if p0Count > 0 { header += fmt.Sprintf(" 🔥%d", p0Count) }
    if blockedCount > 0 { header += fmt.Sprintf(" ⚠️%d", blockedCount) }
    return header
}

// Rich 4-line card format
func renderCard(issue Issue, width int) string {
    line1 := fmt.Sprintf("%s P%d %s %s", typeIcon(issue.Type), issue.Priority, issue.ID, renderAge(issue.CreatedAt))
    line2 := truncate(issue.Title, width-4)
    line3 := fmt.Sprintf("👤%s  ⛔%d  →%d  🏷️%d", issue.Assignee, issue.BlockerCount, issue.BlocksCount, len(issue.Labels))
    line4 := strings.Join(issue.Labels[:min(3, len(issue.Labels))], ", ")
    return strings.Join([]string{line1, line2, line3, line4}, "\n")
}

// Dynamic column visibility: hide empty columns in priority/type modes
func (m model) visibleColumns() []int {
    cols := m.currentColumns()
    var visible []int
    for i, col := range cols {
        if len(col) > 0 || m.swimMode == SwimByStatus || m.showEmptyColumns {
            visible = append(visible, i)
        }
    }
    return visible
}
```

---

## Dashboard Panels with Drill-Down

> **Use when:** you want a multi-panel dashboard view showing ranked metrics, scores, or stats with per-item drill-down.

Multi-panel insights dashboard with metric explanations and calculation proofs.

```go
type MetricPanel struct {
    Title       string
    MetricName  string
    Items       []MetricItem
    Explanation string
    Formula     string
}

type MetricItem struct {
    ID    string
    Value float64
    Label string
}

// 6-panel grid layout (2 rows × 3 columns)
func (m model) renderInsightsDashboard() string {
    panels := []MetricPanel{
        {Title: "🚧 Bottlenecks",  MetricName: "Betweenness",   Items: m.topBetweenness()},
        {Title: "🏛️ Keystones",    MetricName: "Impact Depth",   Items: m.topCriticalPath()},
        {Title: "🌐 Influencers",  MetricName: "Eigenvector",    Items: m.topEigenvector()},
        {Title: "🛰️ Hubs",         MetricName: "HITS Hub",       Items: m.topHubs()},
        {Title: "📚 Authorities",  MetricName: "HITS Authority", Items: m.topAuthorities()},
        {Title: "🔄 Cycles",       MetricName: "Circular Deps",  Items: m.cycles()},
    }

    panelWidth := (m.width - 6) / 3  // 3 columns with spacing

    row1 := lipgloss.JoinHorizontal(lipgloss.Top,
        renderPanel(panels[0], panelWidth),
        renderPanel(panels[1], panelWidth),
        renderPanel(panels[2], panelWidth),
    )
    row2 := lipgloss.JoinHorizontal(lipgloss.Top,
        renderPanel(panels[3], panelWidth),
        renderPanel(panels[4], panelWidth),
        renderPanel(panels[5], panelWidth),
    )

    return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
}

// Detail panel shows calculation proof when item selected
func renderCalculationProof(item MetricItem, deps DependencyInfo) string {
    return fmt.Sprintf(`─── CALCULATION PROOF ───
%s Score: %.3f

Beads depending on this (%d):
%s
This depends on (%d):
%s

%s`,
        item.MetricName, item.Value,
        len(deps.Dependents), renderDepList(deps.Dependents),
        len(deps.Blockers), renderDepList(deps.Blockers),
        item.Explanation,
    )
}
```

---

## Mouse Handling by Focus

> **Use when:** mouse wheel scrolling behaves erratically because events go to the wrong component.

Route mouse events to the correct component based on current focus.

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.MouseMsg:
        switch msg.Button {
        case tea.MouseButtonWheelUp:
            switch m.focus {
            case focusList:
                m.cursor = max(0, m.cursor-1)
                m.syncDetailPane()
            case focusDetail:
                m.viewport.LineUp(3)
            case focusBoard:
                m.boardScrollUp()
            case focusGraph:
                m.graphScrollUp()
            case focusInsights:
                m.insightsScrollUp()
            }

        case tea.MouseButtonWheelDown:
            // Mirror of above...
        }
}
```

---

## Vim Key Combo Tracking

> **Use when:** you want vim-style multi-key combos (`gg` for top, `G` for bottom) that require tracking state between keystrokes.

Track multi-key sequences like `gg` (go to top).

```go
type model struct {
    waitingForG bool
}

func (m model) handleBoardKeys(msg tea.KeyMsg) (model, tea.Cmd) {
    switch msg.String() {
    case "g":
        if m.waitingForG {
            // Second 'g' pressed: jump to top
            m.waitingForG = false
            m.boardCursor = 0
            return m, nil
        }
        // First 'g': wait for second
        m.waitingForG = true
        return m, nil
    case "G":
        m.waitingForG = false
        m.boardCursor = len(m.currentColumn()) - 1  // Jump to bottom
        return m, nil
    default:
        m.waitingForG = false  // Any other key cancels the combo
    }
    // ...
}
```

---

## Terminal Editor Suspension

> **Use when:** you need the simplest possible `tea.ExecProcess` pattern without
> editor classification. For full terminal-vs-GUI dispatch, see
> [Smart Editor Dispatch](#smart-editor-dispatch) which builds on this.

```go
func (m model) openEditor(filePath string) tea.Cmd {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"
    }

    c := exec.Command(editor, filePath)
    c.Stdin = os.Stdin
    c.Stdout = os.Stdout
    c.Stderr = os.Stderr

    // tea.ExecProcess suspends TUI, runs the command, then resumes
    return tea.ExecProcess(c, func(err error) tea.Msg {
        return editorExitMsg{err: err}
    })
}
```

---

## Production Pre-Flight Checklist

### Must-Have (Before Release)

```
□ Two-phase async: expensive work never blocks the UI
□ Immutable snapshots: no mutexes in the render path
□ Responsive layout: 3+ breakpoints (mobile/split/wide/ultra-wide)
□ Focus state machine: explicit enum, modal priority layering
□ Semantic theming: AdaptiveColor for light/dark, no hardcoded hex
□ Pre-computed styles: delegate styles allocated once, not per-frame
□ Viewport virtualization: only render visible rows
□ Handle tea.WindowSizeMsg everywhere
□ Graceful ctrl+c exit with cleanup
□ Works when piped (--no-tui / NO_TUI env var)
□ File watching with debounce (prevent rapid-fire rebuilds)
□ Stale message detection (data hash comparison)
□ Keyboard hints visible (help component or status bar)
□ Tested on 80×24 minimum terminal
□ Tested with TERM=dumb / NO_COLOR=1
□ Tested with both light AND dark backgrounds
```

### Polish (Makes Users Love It)

```
□ Custom braille/unicode spinner for background processing
□ Sparklines for data density in list views
□ Heatmap colors for metric visualization
□ Persistent state (expand/collapse, preferences) saved to JSON
□ Multi-tier help: quick reference + full tutorial + persistent sidebar
□ Vim key combos (gg, G, Ctrl+D/U)
□ Mouse wheel scrolling per-component
□ Focus restoration after overlay dismiss
□ Debounced search (150ms)
□ Smart editor dispatch (terminal vs GUI detection)
□ Context-sensitive shortcuts display
□ Idle-time GC management
□ Object pooling for large datasets
□ Age color coding (fresh/aging/stale)
□ Card border colors encoding dependency status
□ Calculation proofs in detail panels
□ Column statistics in board headers
□ Inline card expansion (d key)
□ Adjustable split ratio (< and > keys)
□ Sort mode indicators in status bar
```

---

## Size-Adaptive Algorithm Selection

> **Use when:** your analysis algorithm works fine for 100 items but times out at 5000. Auto-switch between exact and approximate modes.

Choose algorithm complexity based on dataset size.

```go
type AnalysisConfig struct {
    BetweennessMode string // "exact" or "approx"
    Timeout         time.Duration
    SkipCycles      bool
    SkipHITS        bool
}

func ConfigForSize(nodeCount, edgeCount int) AnalysisConfig {
    switch {
    case nodeCount < 100:
        return AnalysisConfig{
            BetweennessMode: "exact",
            Timeout:         2 * time.Second,
        }
    case nodeCount < 500:
        return AnalysisConfig{
            BetweennessMode: "exact",
            Timeout:         500 * time.Millisecond,
        }
    case nodeCount < 2000:
        return AnalysisConfig{
            BetweennessMode: "approx",  // Reservoir sampling
            Timeout:         300 * time.Millisecond,
            SkipHITS:        edgeCount > nodeCount*5,  // Skip for dense graphs
        }
    default:
        return AnalysisConfig{
            BetweennessMode: "approx",
            Timeout:         300 * time.Millisecond,
            SkipCycles:      true,
            SkipHITS:        true,
        }
    }
}
```

---

## Multi-Layer Caching

> **Use when:** you want repeat operations (triage, analysis) to return instantly when the underlying data hasn't changed.

```go
// Layer 1: In-memory cache (per-process, fast)
type MemCache struct {
    mu        sync.RWMutex
    dataHash  string
    stats     *GraphStats
    computedAt time.Time
    ttl       time.Duration  // Default: 5 minutes
}

func (c *MemCache) Get(currentHash string) (*GraphStats, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.dataHash == currentHash && time.Since(c.computedAt) < c.ttl {
        return c.stats, true
    }
    return nil, false
}

// Layer 2: Disk cache (persistent, cross-invocation)
// ~/.cache/bv/analysis_cache.json
// LRU eviction: max 10 entries, max 24hr age, max 10MB per entry
type DiskCache struct {
    dir     string
    maxAge  time.Duration
    maxSize int64
}

func (c *DiskCache) Get(hash string) (*GraphStats, bool) {
    path := filepath.Join(c.dir, hash+".json")
    stat, err := os.Stat(path)
    if err != nil || time.Since(stat.ModTime()) > c.maxAge {
        return nil, false
    }
    data, _ := os.ReadFile(path)
    var stats GraphStats
    json.Unmarshal(data, &stats)
    return &stats, true
}
```

---

## Composite FilterValue for Zero-Allocation Fuzzy Search

> **Use when:** you want Bubbles list fuzzy search to match across ID, title, status, assignee, labels — all at once, no mode switching.

Instead of searching multiple fields individually (requiring complex UI controls),
flatten every item into a single searchable string at load time.

```go
// Implement list.Item with a composite filter string
type issueItem struct {
    id, title, status, issueType, assignee string
    priority int
    labels   []string
}

// FilterValue is called by Bubbles list for fuzzy search
// Build ONCE at load time, searched every keystroke — must be cheap
func (i issueItem) FilterValue() string {
    // Combine all searchable dimensions into one string
    // Typing "steve bug" finds bugs assigned to Steve
    // Typing "open v1.0" finds open items in v1.0 release
    return strings.Join([]string{
        i.id,
        i.title,
        i.status,
        i.issueType,
        fmt.Sprintf("P%d", i.priority),
        i.assignee,
        strings.Join(i.labels, " "),
    }, " ")
}
```

**Why this works:** The Bubbles list does fuzzy subsequence matching against this single
string. Users can search across ANY dimension without mode switching. The composite string
is built once during load, so search is zero-allocation per keystroke.

---

## Deterministic Stable Sorting

> **Use when:** items with equal priority randomly reorder between renders, causing a disorienting "shuffling list."

Prevent the "shuffling list" problem where equal-priority items randomly reorder.

```go
// ALWAYS use a stable secondary sort to ensure deterministic ordering
func sortIssues(items []Issue, mode SortMode) {
    sort.SliceStable(items, func(i, j int) bool {
        a, b := items[i], items[j]

        switch mode {
        case SortByPriority:
            if a.Priority != b.Priority {
                return a.Priority < b.Priority
            }
            // Tie-breaker: creation date descending (newest first)
            if !a.CreatedAt.Equal(b.CreatedAt) {
                return a.CreatedAt.After(b.CreatedAt)
            }
            // Final tie-breaker: ID (always deterministic)
            return a.ID < b.ID

        case SortByCreatedAsc:
            if !a.CreatedAt.Equal(b.CreatedAt) {
                return a.CreatedAt.Before(b.CreatedAt)
            }
            return a.ID < b.ID

        case SortByUpdated:
            if !a.UpdatedAt.Equal(b.UpdatedAt) {
                return a.UpdatedAt.After(b.UpdatedAt)
            }
            return a.ID < b.ID
        }

        // Default: priority asc → created desc → ID asc
        if a.Priority != b.Priority { return a.Priority < b.Priority }
        if !a.CreatedAt.Equal(b.CreatedAt) { return a.CreatedAt.After(b.CreatedAt) }
        return a.ID < b.ID
    })
}

// Sort mode cycling with status bar indicator
type SortMode int
const (
    SortDefault SortMode = iota
    SortByCreatedAsc
    SortByCreatedDesc
    SortByPriority
    SortByUpdated
)

var sortModeLabels = map[SortMode]string{
    SortDefault:      "Default",
    SortByCreatedAsc: "Created ↑",
    SortByCreatedDesc:"Created ↓",
    SortByPriority:   "Priority",
    SortByUpdated:    "Updated",
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.KeyMsg:
        if msg.String() == "s" {
            m.sortMode = (m.sortMode + 1) % 5
            sortIssues(m.items, m.sortMode)
        }
}
```

---

## tea.Batch Command Accumulation

> **Use when:** your Update() needs to return commands from multiple component updates without losing any.

The idiomatic pattern for collecting multiple commands from component updates.

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    var cmd tea.Cmd

    // Handle global messages
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        m.list.SetSize(m.width, m.height-4)
        m.viewport.Width = m.width
        m.viewport.Height = m.height - 6
    }

    // Always update the spinner if loading
    if m.loading {
        m.spinner, cmd = m.spinner.Update(msg)
        cmds = append(cmds, cmd)
    }

    // Delegate to focused component
    switch m.focus {
    case focusList:
        m.list, cmd = m.list.Update(msg)
        cmds = append(cmds, cmd)
    case focusDetail:
        m.viewport, cmd = m.viewport.Update(msg)
        cmds = append(cmds, cmd)
    }

    // Return ALL commands at once — never call tea.Batch multiple times
    return m, tea.Batch(cmds...)
}

// Custom tea.Cmd wrapping async work
func loadDataCmd(path string) tea.Cmd {
    return func() tea.Msg {
        data, err := os.ReadFile(path)
        if err != nil {
            return errMsg{err: err}
        }
        items := parse(data)
        return dataLoadedMsg{items: items}
    }
}
```

---

## Rich Multi-Line List Delegates

> **Use when:** the default 2-line Bubbles delegate (title + description) is too sparse — you need 3-4 line cards with icons, badges, and metadata.

Default Bubbles delegates show title + description (2 lines). Production TUIs need richer cards.

```go
type richDelegate struct {
    styles DelegateStyles
}

func (d richDelegate) Height() int  { return 4 }  // 4-line cards
func (d richDelegate) Spacing() int { return 1 }  // 1 line between cards
func (d richDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d richDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
    i, ok := item.(issueItem)
    if !ok { return }

    isSelected := index == m.Index()
    width := m.Width() - 4  // Account for list padding

    // Line 1: Type icon, Priority, ID, Age
    line1 := fmt.Sprintf("%s P%d %s %s",
        typeIcon(i.issueType), i.priority, i.id,
        humanAge(i.createdAt))

    // Line 2: Title (truncated to fit)
    line2 := truncate(i.title, width)

    // Line 3: Metadata badges
    line3 := fmt.Sprintf("👤%s  ⛔%d  →%d  🏷️%d",
        i.assignee, i.blockerCount, i.blocksCount, len(i.labels))

    // Line 4: Label names
    line4 := truncate(strings.Join(i.labels, ", "), width)

    // Apply selection styling
    style := d.styles.Normal
    if isSelected {
        style = d.styles.Selected
    }

    card := style.Width(width).Render(
        strings.Join([]string{line1, line2, line3, line4}, "\n"),
    )
    fmt.Fprint(w, card)
}

// Register the custom delegate
l := list.New(items, richDelegate{styles: myStyles}, width, height)
```

---

## Inline Expansion

> **Use when:** you want to show item details in-place (toggle with a key) without switching to a separate detail view.

Toggle expanded detail within a list without switching views.

```go
type model struct {
    expandedID string  // Empty = nothing expanded
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.KeyMsg:
        switch msg.String() {
        case "d":
            // Toggle expansion
            current := m.selectedItem().ID
            if m.expandedID == current {
                m.expandedID = ""  // Collapse
            } else {
                m.expandedID = current  // Expand
            }
        case "j", "k":
            // Auto-collapse on navigation for smooth browsing
            m.expandedID = ""
            // ... normal navigation
        }
}

func (m model) renderItem(item Item, width int) string {
    base := m.renderCompactCard(item, width)

    if item.ID != m.expandedID {
        return base
    }

    // Expanded: show full details inline
    detail := lipgloss.NewStyle().
        Border(lipgloss.NormalBorder(), false, false, false, true).
        BorderForeground(m.theme.Muted).
        PaddingLeft(2).
        Width(width - 4).
        Render(strings.Join([]string{
            item.Description,
            "",
            "Blocked by: " + strings.Join(item.BlockerIDs, ", "),
            "Blocks: " + strings.Join(item.DependentIDs, ", "),
        }, "\n"))

    return base + "\n" + detail
}
```

---

## Clipboard Integration

> **Use when:** you want `y` to copy an ID or `C` to copy formatted content, with visual confirmation.

```go
import "github.com/atotto/clipboard"

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.KeyMsg:
        switch msg.String() {
        case "y":
            // Copy selected item ID
            if item := m.selectedItem(); item != nil {
                clipboard.WriteAll(item.ID)
                m.toast = "Copied: " + item.ID
                m.toastTimer = 30
                return m, tick()
            }
        case "C":
            // Copy full item as formatted markdown
            if item := m.selectedItem(); item != nil {
                md := formatAsMarkdown(item)
                clipboard.WriteAll(md)
                m.toast = "Copied as Markdown"
                m.toastTimer = 30
                return m, tick()
            }
        }
}
```

---

## Environment Variable Preferences

> **Use when:** you want users to tune behavior (theme, debounce, layout) without a config file — just env vars.

Runtime tuning without config files — ideal for CLI tools.

```go
func loadPreferences() Preferences {
    p := Preferences{
        DebounceMs:   200,
        Theme:        "auto",
        SplitRatio:   0.4,
        SearchMode:   "fuzzy",
    }

    if v := os.Getenv("BV_DEBOUNCE_MS"); v != "" {
        if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
            p.DebounceMs = ms
        }
    }
    if v := os.Getenv("BV_THEME"); v != "" {
        p.Theme = v  // "dark", "light", or "auto"
    }
    if v := os.Getenv("BV_SPLIT_RATIO"); v != "" {
        if r, err := strconv.ParseFloat(v, 64); err == nil && r > 0.1 && r < 0.9 {
            p.SplitRatio = r
        }
    }
    if v := os.Getenv("NO_COLOR"); v != "" {
        p.NoColor = true
    }

    return p
}

// Standard env vars to respect:
// NO_COLOR=1        → disable all color output (spec: https://no-color.org/)
// TERM=dumb         → no cursor movement, no color
// CLICOLOR_FORCE=1  → force color even when not a TTY
```

---

## Graceful Degradation

> **Use when:** your TUI has optional features (clipboard, git, external tools) that may not be available in all environments.

When optional features or dependencies aren't available, hide them silently.

```go
type featureFlags struct {
    hasCass      bool   // Optional AI session integration
    hasClipboard bool   // Clipboard may not work over SSH
    hasGit       bool   // Git may not be in PATH
}

func detectFeatures() featureFlags {
    return featureFlags{
        hasCass:      commandExists("cass"),
        hasClipboard: clipboardAvailable(),
        hasGit:       commandExists("git"),
    }
}

func commandExists(name string) bool {
    _, err := exec.LookPath(name)
    return err == nil
}

func clipboardAvailable() bool {
    // Clipboard fails over SSH, in containers, on some Wayland setups
    err := clipboard.WriteAll("test")
    if err != nil { return false }
    _, err = clipboard.ReadAll()
    return err == nil
}

// In rendering: conditionally show features
func (m model) renderStatusBar() string {
    parts := []string{
        fmt.Sprintf("📋 %d items", len(m.items)),
    }

    if m.features.hasGit {
        parts = append(parts, fmt.Sprintf("🌿 %s", m.gitBranch))
    }

    if m.features.hasCass && m.cassStatus != "" {
        parts = append(parts, m.cassStatus)
    }

    return strings.Join(parts, "  •  ")
}

// In key handling: don't show shortcuts for unavailable features
func (m model) getAvailableKeys() []KeyBinding {
    keys := m.coreKeys()

    if m.features.hasClipboard {
        keys = append(keys, KeyBinding{Key: "y", Desc: "copy ID"})
        keys = append(keys, KeyBinding{Key: "C", Desc: "copy as markdown"})
    }

    if m.features.hasCass {
        keys = append(keys, KeyBinding{Key: "V", Desc: "view sessions"})
    }

    return keys
}
```

---

## Cached Markdown Rendering

> **Use when:** you render markdown with Glamour in a detail pane and the same content re-renders on every frame.

Glamour rendering is expensive. Cache it and only re-render when content changes.

```go
type markdownCache struct {
    renderer *glamour.TermRenderer
    cache    map[string]string  // content hash → rendered output
    width    int
}

func newMarkdownCache(theme string) *markdownCache {
    // Create renderer ONCE (avoids expensive regex recompilation)
    r, _ := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),  // Adapts to light/dark
        glamour.WithWordWrap(80),
    )
    return &markdownCache{
        renderer: r,
        cache:    make(map[string]string),
    }
}

func (mc *markdownCache) Render(content string, width int) string {
    // Invalidate cache if width changed
    if width != mc.width {
        mc.cache = make(map[string]string)
        mc.width = width
        mc.renderer, _ = glamour.NewTermRenderer(
            glamour.WithAutoStyle(),
            glamour.WithWordWrap(width),
        )
    }

    // Check cache
    key := content  // For large content, use a hash instead
    if rendered, ok := mc.cache[key]; ok {
        return rendered
    }

    // Render and cache
    rendered, err := mc.renderer.Render(content)
    if err != nil {
        return content  // Fallback: raw text
    }
    mc.cache[key] = rendered
    return rendered
}

// Usage in detail pane: only re-renders when selection changes
func (m model) renderDetail(width int) string {
    item := m.selectedItem()
    rendered := m.mdCache.Render(item.Description, width-4)
    m.viewport.SetContent(rendered)
    return m.viewport.View()
}
```

---

## Error Recovery in Background Goroutines

> **Use when:** you launch goroutines for async work and a panic in one of them kills the entire TUI.

Panics in goroutines crash the entire program. Always recover.

```go
func safeGo(name string, fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("panic in %s: %v\n%s", name, r, debug.Stack())
                // Don't re-panic — let the UI continue operating
            }
        }()
        fn()
    }()
}

// Usage
safeGo("phase2-analysis", func() {
    analyzer.ComputeExpensiveMetrics()
    program.Send(Phase2ReadyMsg{})
})

safeGo("file-watcher", func() {
    watchFile(dataPath, func() {
        program.Send(FileChangedMsg{})
    })
})

// For tea.Cmd that might panic:
func safeCmdFunc(fn func() tea.Msg) tea.Cmd {
    return func() tea.Msg {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("cmd panic: %v", r)
            }
        }()
        return fn()
    }
}
```

---

## Status Bar with Dynamic Segments

> **Use when:** you need a status bar with left-aligned and right-aligned segments that fills the terminal width.

Flexible status bar that fills the gap between left and right segments.

```go
func renderStatusBar(width int, segments ...StatusSegment) string {
    var left, right []string

    for _, seg := range segments {
        styled := lipgloss.NewStyle().
            Background(seg.bg).
            Foreground(seg.fg).
            Padding(0, 1).
            Render(seg.text)

        if seg.align == AlignLeft {
            left = append(left, styled)
        } else {
            right = append(right, styled)
        }
    }

    leftStr := strings.Join(left, "")
    rightStr := strings.Join(right, "")

    // Fill gap between left and right
    gap := width - lipgloss.Width(leftStr) - lipgloss.Width(rightStr)
    if gap < 0 { gap = 0 }

    return leftStr + strings.Repeat(" ", gap) + rightStr
}

type StatusSegment struct {
    text  string
    bg    lipgloss.Color
    fg    lipgloss.Color
    align Alignment
}

// Usage:
bar := renderStatusBar(m.width,
    StatusSegment{"NORMAL", "#FF5F87", "#FFF", AlignLeft},
    StatusSegment{filename, "#6124DF", "#FFF", AlignLeft},
    StatusSegment{fmt.Sprintf("[%s]", sortModeLabels[m.sortMode]), "#444", "#CCC", AlignRight},
    StatusSegment{fmt.Sprintf("%d/%d", m.cursor+1, len(m.items)), "#A550DF", "#FFF", AlignRight},
)
```

---

## Breadcrumb Navigation

> **Use when:** your TUI has nested views (board > swimlane > card) and users lose track of where they are.

Show the user's path through nested views.

```go
func (m model) renderBreadcrumb() string {
    parts := []string{"Home"}

    switch m.focus {
    case focusList:
        // Just "Home" — we're at the root
    case focusBoard:
        parts = append(parts, "Board")
        if m.swimMode != SwimByStatus {
            parts = append(parts, swimModeLabel(m.swimMode))
        }
    case focusInsights:
        parts = append(parts, "Insights")
        if m.selectedPanel >= 0 {
            parts = append(parts, m.panels[m.selectedPanel].Title)
        }
    case focusFlowMatrix:
        parts = append(parts, "Flow Matrix")
        if m.drilldownLabel != "" {
            parts = append(parts, m.drilldownLabel)
        }
    case focusHistory:
        parts = append(parts, "History")
        if m.historyMode == historyGitMode {
            parts = append(parts, "Git Mode")
        }
    }

    // Style: Home > Board > Priority  (dimmed separators)
    sep := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(" > ")
    var styled []string
    for i, p := range parts {
        if i == len(parts)-1 {
            styled = append(styled, lipgloss.NewStyle().Bold(true).Render(p))
        } else {
            styled = append(styled, lipgloss.NewStyle().Foreground(m.theme.Muted).Render(p))
        }
    }
    return strings.Join(styled, sep)
}
```

---

## strings.Builder in View()

> **Use when:** your View() function concatenates strings with `+=` in a loop — this is O(n²) and the #1 Go TUI perf bug.

Never use `+=` for string concatenation in render functions.

```go
// BAD: O(n²) string concatenation
func (m model) View() string {
    s := ""
    for _, item := range m.items {
        s += renderItem(item) + "\n"  // Each += copies entire string
    }
    return s
}

// GOOD: O(n) with pre-allocated builder
func (m model) View() string {
    var b strings.Builder
    b.Grow(len(m.items) * 80)  // Pre-allocate: ~80 chars per line

    for _, item := range m.items {
        b.WriteString(renderItem(item))
        b.WriteByte('\n')
    }
    return b.String()
}

// For complex views: use a render buffer pattern
type renderBuf struct {
    strings.Builder
}

func (rb *renderBuf) Line(s string) {
    rb.WriteString(s)
    rb.WriteByte('\n')
}

func (rb *renderBuf) BlankLine() {
    rb.WriteByte('\n')
}

func (rb *renderBuf) Divider(width int, char rune) {
    rb.WriteString(strings.Repeat(string(char), width))
    rb.WriteByte('\n')
}

func (m model) View() string {
    var buf renderBuf
    buf.Grow(4096)

    buf.Line(m.renderHeader())
    buf.Divider(m.width, '─')
    for i := m.scrollOffset; i < m.scrollOffset+m.visibleRows; i++ {
        buf.Line(m.renderRow(i))
    }
    buf.Line(m.renderStatusBar())

    return buf.String()
}
```

---

## Color-Coded Borders Encoding State

> **Use when:** you want item borders to visually encode state (blocked, ready, high-impact) without the user reading text.

Borders that convey meaning without reading text.

```go
func itemBorderStyle(item Item, theme Theme) lipgloss.Style {
    base := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(0, 1)

    switch {
    case item.IsBlocked():
        // Red: has unresolved dependencies — work on blockers first
        return base.BorderForeground(theme.Blocked)
    case item.BlocksOthers() && item.IsOpen():
        // Yellow/gold: completing this unblocks downstream work
        return base.BorderForeground(lipgloss.Color("#f1fa8c"))
    case item.IsReady():
        // Green: open with no blockers — pick this up!
        return base.BorderForeground(theme.Open)
    default:
        // Default: standard border
        return base.BorderForeground(theme.Border)
    }
}

// Search match overlay: highlight matching items in different colors
func searchMatchBorder(isCurrentMatch bool) lipgloss.Style {
    if isCurrentMatch {
        return lipgloss.NewStyle().
            Border(lipgloss.ThickBorder()).
            BorderForeground(lipgloss.Color("#bd93f9"))  // Purple: current match
    }
    return lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("#8be9fd"))  // Cyan: other matches
}
```

---

## Flattened Tree Navigation

> **Use when:** you have a hierarchical tree and need vim-style j/k navigation with h/l for expand/collapse.

Convert a tree structure to a flat visible-node list for vim-style j/k navigation.

```go
type TreeNode struct {
    ID       string
    Depth    int
    Children []*TreeNode
    // ... data
}

// Flatten visible nodes (respecting expand/collapse state)
func flattenVisible(roots []*TreeNode, expanded map[string]bool) []*TreeNode {
    var result []*TreeNode

    var walk func(node *TreeNode, depth int)
    walk = func(node *TreeNode, depth int) {
        node.Depth = depth
        result = append(result, node)

        // Only recurse into expanded nodes
        if expanded[node.ID] || (depth < 2 && !explicitly(expanded, node.ID, false)) {
            for _, child := range node.Children {
                walk(child, depth+1)
            }
        }
    }

    for _, root := range roots {
        walk(root, 0)
    }
    return result
}

// Render with tree connectors
func renderTreeRow(node *TreeNode, isLast bool, isSelected bool) string {
    var prefix string
    for i := 0; i < node.Depth; i++ {
        prefix += "│ "
    }

    connector := "├─"
    if isLast {
        connector = "└─"
    }

    expandIcon := "•"  // Leaf
    if len(node.Children) > 0 {
        if node.Expanded {
            expandIcon = "▾"  // Expanded
        } else {
            expandIcon = "▸"  // Collapsed
        }
    }

    line := prefix + connector + " " + expandIcon + " " + node.Title

    if isSelected {
        return selectedStyle.Render(line)
    }
    return line
}

// Navigation: h collapses or jumps to parent, l expands or enters first child
func (m model) handleTreeKey(msg tea.KeyMsg) (model, tea.Cmd) {
    node := m.visibleNodes[m.treeCursor]

    switch msg.String() {
    case "l", "right":
        if len(node.Children) > 0 && !m.treeExpanded[node.ID] {
            m.treeExpanded[node.ID] = true
            m.rebuildVisibleNodes()
        } else if len(node.Children) > 0 {
            // Already expanded: move to first child
            m.treeCursor++
        }
    case "h", "left":
        if m.treeExpanded[node.ID] {
            m.treeExpanded[node.ID] = false
            m.rebuildVisibleNodes()
        } else if node.Depth > 0 {
            // Already collapsed: jump to parent
            m.treeCursor = m.findParentIndex(m.treeCursor)
        }
    case "o":
        m.expandAll()
    case "O":
        m.collapseAll()
    }
    return m, nil
}
```

---

## Idle-Time GC Management

> **Use when:** you notice GC pauses during active interaction — schedule collection during idle periods instead.

Trigger garbage collection during idle periods to avoid GC pauses during interaction.

```go
type model struct {
    lastActivity time.Time
    gcScheduled  bool
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case tea.KeyMsg, tea.MouseMsg:
        m.lastActivity = time.Now()
        m.gcScheduled = false

    case idleTickMsg:
        if time.Since(m.lastActivity) > 5*time.Second && !m.gcScheduled {
            m.gcScheduled = true
            return m, func() tea.Msg {
                runtime.GC()
                return gcCompleteMsg{}
            }
        }
    }
    return m, nil
}

// Start idle ticker in Init()
func (m model) Init() tea.Cmd {
    return tea.Batch(
        loadData,
        tea.Every(3*time.Second, func(t time.Time) tea.Msg {
            return idleTickMsg{}
        }),
    )
}
```

---

## Data Hash Fingerprinting

> **Use when:** you need a deterministic cache key based on data content, or need to detect whether data has actually changed.

Deterministic hash for cache invalidation and stale message detection.

```go
func ComputeDataHash(issues []Issue) string {
    h := sha256.New()

    // Sort by ID for determinism
    sorted := make([]Issue, len(issues))
    copy(sorted, issues)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].ID < sorted[j].ID
    })

    for _, issue := range sorted {
        fmt.Fprintf(h, "%s|%s|%s|%s|%d|%s|%s",
            issue.ID, issue.Title, issue.Status,
            issue.Type, issue.Priority,
            issue.CreatedAt.Format(time.RFC3339Nano),
            issue.UpdatedAt.Format(time.RFC3339Nano),
        )
        // Include sorted labels and deps for completeness
        labels := make([]string, len(issue.Labels))
        copy(labels, issue.Labels)
        sort.Strings(labels)
        for _, l := range labels {
            fmt.Fprintf(h, "|L:%s", l)
        }
    }

    return hex.EncodeToString(h.Sum(nil))
}
```
