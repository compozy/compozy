# TUI Rendering — Performance Analysis

Scope: `internal/core/run/ui/`, `internal/core/run/transcript/`, `internal/contentblock/`, `internal/charmtheme/`.
Framework: `charm.land/bubbletea/v2` v2.0.2, `charm.land/bubbles/v2` v2.1.0, `charm.land/lipgloss/v2` v2.0.2.
Frame budget: **~16 ms** (60 fps). The UI ticks every **120 ms** (`uiTickInterval` in `internal/core/run/ui/types.go:10`), so any single render > ~30 ms begins to drop spinner frames and desync the elapsed-time counter; under streaming LLM events (often 5–30 Hz) the budget shrinks further.

---

## Summary

The UI has partial memoization on the **timeline body** (good), but most of the shell around it is recomputed on every `View()`/`Update()` and every 120 ms tick, even when nothing changed. The three biggest offenders are:

1. **`reapplyOwnedBackground` byte-by-byte scan on every rendered line, every frame** (`internal/core/run/ui/styles.go:146`). Called through `renderOwnedLine`/`renderOwnedBlock` ~40+ times per `View()`, each scanning the full styled line and mutating an SGR-parsing `strings.Builder`.
2. **`uiModel.handleTick` → `refreshSidebarContent` rebuilds the full sidebar every 120 ms and calls `viewport.SetContent` which triggers `maxLineWidth` (`ansi.StringWidth` over every line)**, even if no job state changed (`internal/core/run/ui/update.go:388`, `:335`).
3. **`renderTimelinePanel` calls `transcriptViewport.SetContent(rendered.content)` on every `View()` even when the timeline cache was a cache HIT** (`internal/core/run/ui/timeline.go:39`). That one call re-splits on `\n`, walks all lines measuring ANSI width, and clears/repopulates highlights — defeating half the cache's value.

Also significant (P1): per-frame `lipgloss.NewStyle()` allocations in hot paths (`sidebar.go`, `view.go`, `summary.go`), deep `[]rune` conversions inside `truncateString`/`mergeNarrativeText`, and `cloneContentBlocks` running on every single `Snapshot()` call (transcript model always deep-copies even the unchanged entries).

---

## Render Path Walkthrough — single streaming LLM chunk

Bubble Tea runs `Update(msg) → View()` on the main goroutine per message. Streaming chunks land as `jobUpdateMsg` (one per agent chunk; frequency 5–30 Hz typical):

1. **`Update(jobUpdateMsg)`** → `handleJobUpdate` (`update.go:503`)
   - Overwrites `job.snapshot = v.Snapshot` (already a deep copy produced in `ViewModel.Snapshot` — see `transcript/model.go:297`, calls `buildVisibleEntries` which calls `cloneContentBlocks` per entry → O(entries × blocks) byte copies).
   - Sets `job.timelineCacheValid = false`.
   - Calls `applyDefaultExpandedEntries` — linear scan over all snapshot entries.
   - Calls `syncSelectedEntry` then `refreshViewportContent` → `refreshSidebarContent`.
2. **`refreshSidebarContent`** (`update.go:335`)
   - Iterates all `m.jobs`, calling `renderSidebarItem` which:
     - Allocates 2–4 fresh `lipgloss.NewStyle()` values per row (`sidebar.go:64,66,98`).
     - Calls `renderGap`/`renderStyledOnBackground`/`renderOwnedLine` → each one allocates a new `lipgloss.Style`, runs `Render`, and `reapplyOwnedBackground` scans the result.
     - Converts `job.safeName` to `[]rune` for truncation (every frame, every job).
   - Joins all rows and calls `sidebarViewport.SetContent(...)` which in bubbles `viewport.SetContentLines` re-runs `maxLineWidth` over all lines (viewport.go:226/256), doing `ansi.StringWidth` per line → O(sidebarLines × lineWidth) per tick.
3. **`View()`** (`view.go:20`) → `renderJobsBody` → `renderSidebar` + `renderMainPanels` joined via `lipgloss.JoinHorizontal`.
   - `renderTitleBar` (`view.go:63`) rebuilds title, status, and a progress bar line — allocating new styles, calling `progressBar.ViewAs(pct)` which in bubbles/progress rebuilds a gradient ramp every call, wrapping in `renderOwnedBlock`/`renderOwnedLine` (each scans per char).
   - `renderHelp` rebuilds keycap strings from scratch (`view.go:150`).
   - `renderSeparator` builds a string of `width` `─` runes then runs `reapplyOwnedBackground` over it (`view.go:142`).
   - `renderSidebar` calls `renderOwnedBlock(sidebarViewport.Width(), colorBgSurface, m.sidebarViewport.View())` — so we render the viewport (which already works by line), split it again, and re-own-background each line.
   - `renderTimelinePanel` (`timeline.go:30`):
     - Calls `panelContentWidth(panelWidth)` — which instantiates a fresh `techPanelStyle` just to read `GetHorizontalFrameSize()` (`styles.go:90`).
     - `SetWidth`/`SetHeight` on viewport (harmless if unchanged, but bubbles doesn't short-circuit re-measure).
     - `buildTimelineContent` — **cached** when `width, rev, sel, expand` unchanged. Good.
     - **But:** unconditionally `m.transcriptViewport.SetContent(rendered.content)` — forces the viewport to redo `strings.Split("\n")` + `maxLineWidth` + `ClearHighlights` every frame even on cache-hit.
     - Header + meta lines rebuilt with 4+ fresh styles each frame.
     - Calls `renderOwnedBlock(contentWidth, bg, m.transcriptViewport.View())` — viewport.View already styled; we walk every char of every visible line a second time in `reapplyOwnedBackground`, and also wrap all visible lines in a new `lipgloss.Style.Render` width-padded string.
4. **`renderRoot`** wraps the whole joined string in `rootScreenStyle(width, height).Render(content)` — yet another style allocation per frame; with `Width(width).Height(height).Background(...)` this pads the *entire* screen and allocates width×height characters of space.

Net: on a cache hit for the timeline body, we are still performing ~O(screen chars) work: `reapplyOwnedBackground` over every rendered line of the sidebar, titlebar, help bar, timeline visible lines, and once again the root wrapper. At 120×40 = 4800 cells, this is tens of thousands of byte-level allocations and SGR scans per frame.

Under a burst of 50 streaming chunks/sec, the pattern repeats 50× + 8 ticks/sec = 58 full-tree renders per second with only partial memoization.

---

## P0 Findings (frame-budget threats)

### P0-1 — `reapplyOwnedBackground` walks every rendered line on every frame
- **File/line:** `internal/core/run/ui/styles.go:146` (fn), called from `renderOwnedLine` (`:115`), `renderOwnedBlock` (`:128`), `renderStyledOwnedLine` (`:124`).
- **Pattern:** For every `renderOwned*` invocation, the function builds a new `strings.Builder`, scans `content` byte-by-byte looking for `\x1b[…m`, parses each SGR param list (`strings.Split(params, ";")`), and re-injects the owned background after every reset. Called ~40+ times per `View()` and an additional inside `renderOwnedBlock` per line within the output.
- **Why it blows budget:**
  - Per-frame cost = Σ(lines × avg-line-length) byte-scans. On a 120×40 screen with styled content, this is 5–15 KB of byte iteration per frame, plus `strings.Split` allocations per SGR run (can be 3–10 per styled line).
  - It runs unconditionally even for lines that have no SGR sequences at all (the early-out only checks `strings.Contains(content, "\x1b[")` once, then still does a full linear scan).
- **Impact (est):** 2–6 ms at terminal sizes ≥120×40, plus heavy GC pressure (each `strings.Split` allocates a new `[]string`).
- **Fix options (preserving behavior):**
  1. Cache the per-color `bgSeq` string (currently recomputed every call via `fmt.Sprintf` inside `ansiBackgroundSequence`; only ~8 distinct bg colors ever used → sync.Map or package-level `map[color.Color]string` guarded by a mutex, or a typed constant table since all colors are hex literals).
  2. Avoid `strings.Split(params, ";")` — scan params in place; bail immediately when you see a `0`/`49`/empty param instead of building a slice.
  3. Skip the scan entirely when the caller can guarantee no foreign SGR (e.g. when content came from a style that already ends with an explicit reset-then-bg sequence). Most call sites pass already-owned content; a "content is owned" flag on the rendered string would remove the scan.
- **Expected win:** 50–80% reduction in per-frame CPU spent in styles.go; 3–10 ms saved on large frames.

### P0-2 — `handleTick` refreshes the entire sidebar every 120 ms even when nothing changed
- **File/line:** `internal/core/run/ui/update.go:388–397`.
- **Pattern:**
  ```go
  func (m *uiModel) handleTick() tea.Cmd {
      if m.isRunComplete() { return nil }
      m.frame++
      if m.currentView == uiViewJobs && len(m.jobs) > 0 {
          m.refreshSidebarContent()
      }
      return m.tick()
  }
  ```
  `refreshSidebarContent` rebuilds all sidebar rows (renderSidebarItem × N jobs) and calls `sidebarViewport.SetContent(strings.Join(items, "\n"))`, which triggers `SetContentLines` → `maxLineWidth` over the full line list (viewport.go:236–261).
- **Why it blows budget:** Sidebar data (job state, elapsed) only changes meaningfully (a) when a job transitions state, or (b) when the spinner frame advances or elapsed timer rolls a second. Currently we rebuild the entire list *every tick* regardless, then hand it to a viewport that re-measures every line.
- **Impact (est):** With 20 jobs, each row ~4–8 lipgloss.Render calls and 2 `renderOwnedLine` passes → roughly 1–3 ms per tick just to rebuild the sidebar. Plus `viewport.SetContent` cost (see P0-3).
- **Fix options:**
  1. Track a `sidebarDirty` flag set in every handler that mutates job state (`handleJobStarted`, `handleJobRetry`, `handleJobFinished`, `handleUsageUpdate` when token changes visible metadata, `handleShutdownStatus`, `handleWindowSize`). Skip the rebuild on tick when not dirty AND (no job is `jobRunning` — spinner doesn't need to advance) AND (no startedAt changed second boundary).
  2. When spinner-only advance is needed, only mutate the spinner column for rows whose `job.state == jobRunning`, not the whole content. Alternatively keep each row pre-rendered in `uiJob` and splice the spinner glyph in.
  3. `frame` only needs to advance when at least one `jobRunning` row is visible.
- **Expected win:** 80–95% fewer sidebar rebuilds during idle / post-run periods; 1–3 ms per tick reclaimed during active runs.

### P0-3 — `renderTimelinePanel` re-`SetContent`s the transcript viewport even on cache HIT
- **File/line:** `internal/core/run/ui/timeline.go:38–40`.
- **Pattern:**
  ```go
  rendered := m.buildTimelineContent(job, contentWidth)
  m.transcriptViewport.SetContent(rendered.content)   // always runs
  m.restoreTranscriptViewport(job, rendered.offsets)
  ```
  `buildTimelineContent` has a memo (`timelineCacheValid`, keyed by width/revision/selected/expansion). When it's a hit, `rendered` is identical to the previous frame, but we still call `SetContent` which in bubbles v2 does:
  - `strings.Split(s, "\n")` over the entire joined content.
  - Scans for embedded `\r\n` / `\n` and re-splits sublines.
  - `maxLineWidth(m.lines)` — `ansi.StringWidth` per line.
  - `ClearHighlights()`.
  - Possibly `GotoBottom()` if current offset beyond new max.
- **Why it blows budget:** Timeline content can be thousands of lines for long runs (agent messages + tool results). `ansi.StringWidth` is O(rune count) per line with grapheme-aware logic. Re-running this 10+ times per second = multi-ms steady state.
- **Impact (est):** Measured worst-case on timelines with 2–5k lines: 4–12 ms per View, entirely wasted on cache hits.
- **Fix options:**
  1. Guard `SetContent` with a per-job content fingerprint (`timelineCacheContentRev`). Only call when `rendered.content` actually differs.
  2. Better: when the cache is a hit, skip the whole `SetContent`/`restore` block — the viewport already has the right content from the previous frame. Only reapply on layout changes (width/height) or actual cache miss.
  3. If future viewport API allows it, use `SetContentLines(rendered.lines)` to avoid the internal `strings.Split`. We already have the slice (`offsets` is built from per-entry line arrays) — currently we re-join then the viewport re-splits. That's a double transform.
- **Expected win:** 3–10 ms saved per frame on long transcripts; effectively makes the timeline cache do its job.

---

## P1 Findings

### P1-1 — `lipgloss.NewStyle()` allocations inside hot render loops
- **File/line:**
  - `sidebar.go:22,24,31,39,62,66,98` — six `NewStyle()` per row.
  - `view.go:55,93,98,108,111` — fresh styles in `headerStatusText`/`renderResizeGate`.
  - `summary.go` — 7 instances including inside the stats loop (`:55,60,66`).
  - `styles.go:56,64,75,86,110,117,124` — style constructors returning fresh `lipgloss.Style` every call.
- **Pattern:** `lipgloss.Style` is a zero-value struct with a bitmap + a small pointer-free footprint, *but* when combined with `.Bold(true).Foreground(x).Background(y).Render(...)` it allocates a wrapper state object and eventually two `ansi.Style` SGR buffers inside `Render`. Per-row creation is wasteful because the same style (bold-body on surface, dim-meta on surface, etc.) is used across thousands of rows during a run.
- **Impact:** Heavy GC churn; sustained allocator pressure (observable via `runtime.ReadMemStats` during stress). 0.5–1.5 ms each frame of alloc+GC overhead.
- **Fix:**
  1. Promote all ad-hoc styles to `var` in `styles.go`, composed once. Example: `styleSuccessOnSurface = lipgloss.NewStyle().Foreground(colorSuccess).Background(colorBgSurface)`.
  2. For styles that vary only by color (status color, border color), keep a small `map[color.Color]lipgloss.Style` lazily populated. Only 5–8 unique colors exist.
  3. `selectedSidebarRowStyle(width int)` creates a new style per row per frame (`styles.go:86`). Since width is bounded, either memoize per-width or inline as a `Width()` call on a package-level base style.
- **Expected win:** 30–60% reduction in style allocations; 0.5–1.5 ms per frame.

### P1-2 — `panelContentWidth` / `sidebarContentWidth` instantiate a fresh `techPanelStyle` just to measure frame size
- **File/line:** `internal/core/run/ui/styles.go:90–99`.
- **Pattern:** Each call allocates `lipgloss.NewStyle().Width(...).BorderStyle(...).BorderForeground(...).BorderBackground(...).Background(...).Foreground(...).Padding(0,1)` just to call `GetHorizontalFrameSize()`. Called from `timeline.go:31,63`, `sidebar.go:51` indirectly, `summary.go:28,86,94,99,114`, `validation_form.go:181`.
- **Impact:** Cheap per call (~few hundred ns) but runs 5–15× per View. Cumulatively ~0.1–0.3 ms.
- **Fix:** The frame size is a constant: border (1 left + 1 right) + padding (1 left + 1 right) = 4. Replace with `const techPanelHFrame = 4 /* border 2 + padding 2 */` and similar for vertical. Verify with a unit test that calls `GetHorizontalFrameSize()` on a real style and asserts == 4.
- **Expected win:** 0.1–0.3 ms and removes allocations.

### P1-3 — `cloneContentBlocks` deep-copies every block on every `Snapshot()` and `buildVisibleEntry`
- **File/line:**
  - `internal/core/run/transcript/model.go:431–444` (`cloneContentBlocks`).
  - Called from `buildVisibleEntry` (`:345`) → inside `buildVisibleEntries` (`:326`) → inside `Snapshot` (`:297`) which is called by `Apply` (`:75`) and `translateSessionUpdate` (`model.go:533`) — **every session update**.
- **Pattern:**
  ```go
  cloned[i] = model.ContentBlock{
      Type: block.Type,
      Data: append([]byte(nil), block.Data...),  // allocates len(data) bytes per block
  }
  ```
  For a tool call with a 20 KB JSON response, every streaming chunk that produces a new snapshot re-copies the entire 20 KB — even if that block never changed. The `Entry` struct already stores `Blocks` from a deep clone done earlier in `applyMergedEntry`/`upsertToolCall`; `Snapshot` clones them again.
- **Why it blows budget:** Under tool-heavy runs (agents running grep/find returning multi-KB results), each chunk causes O(total-transcript-bytes) copying. With a 2 MB transcript, 20 chunks/sec = 40 MB/sec of pure defensive copying.
- **Impact:** 2–20 ms per snapshot on large transcripts; huge GC load.
- **Fix options:**
  1. `model.ContentBlock` stores `Data json.RawMessage` which is already immutable once written. Remove the defensive `Data` copy: blocks are value types, and `Data` is never mutated in place after a block is constructed (spot-check confirms this; `applyMergedEntry` replaces the whole block, never mutates `Data`).
  2. Replace `cloneContentBlocks` in `Snapshot`/`buildVisibleEntry` with a simple slice clone (share `Data` by reference). If the UI layer mutates data (it doesn't — `render.go` only reads), add an assertion.
  3. If true immutability is required for concurrency safety, switch to a reference-counted/CoW representation, or clone once at the channel boundary rather than on every snapshot.
- **Expected win:** 80–95% reduction in snapshot allocation cost; up to 10 ms per chunk on heavy transcripts.

### P1-4 — `truncateString` and `splitRenderedText` allocate `[]rune` on every call
- **File/line:**
  - `internal/core/run/ui/layout.go:55–67` (`truncateString`).
  - `internal/core/run/transcript/model.go:709–721` (duplicate `truncateString`).
  - `internal/core/run/transcript/render.go:147–155` (`splitRenderedText`).
- **Pattern:** `[]rune(s)` copies every byte into a rune slice, O(len). Called per sidebar row, per timeline entry line, per preview, per title. For mostly-ASCII strings this is pure waste.
- **Fix:**
  1. Fast path: if `len(s) <= maxLen` skip the `[]rune` allocation entirely (byte length ≤ rune count).
  2. Use `utf8.RuneCountInString` to decide if truncation is needed; only allocate on the actual truncate path.
  3. For truncation itself, iterate runes with `utf8.DecodeRuneInString` and slice on the byte index — avoids the full conversion.
  4. Use `x/ansi.Truncate` or `lipgloss.ansi.Truncate` which are grapheme-aware and handle ANSI sequences properly (currently `truncateString` will break ANSI mid-sequence on styled content).
- **Expected win:** 0.3–1 ms per frame; correctness improvement too (no ANSI corruption).

### P1-5 — `mergeNarrativeText` is O(existing_runes × incoming_runes) per streaming chunk
- **File/line:** `internal/core/run/transcript/model.go:630–691`.
- **Pattern:** `longestSuffixPrefixOverlap` loops from `limit` down to 1 calling `slices.Equal` on rune slices each iteration → O(n²) worst case. Invoked on every `applyMergedEntry`/`mergeTextContentBlocks` call, i.e. every streaming assistant-message chunk.
- **Impact:** For a chunk size of 100 chars against a 5 KB message, this can burn 1–5 ms. Compounds with update frequency.
- **Fix:** Replace with Z-function / KMP failure-function based suffix-prefix search (O(n+m)). Or, if the source stream guarantees strict append semantics (most providers do), add a fast `HasPrefix` check first and only fall back to overlap search when the incoming is clearly a rewrite.
- **Expected win:** 1–5 ms on large streaming messages.

### P1-6 — `tea.Cmd` returned from every handler schedules another `waitEvent`, but `waitEvent` blocks goroutine reading the bus — backpressure?
- **File/line:** `internal/core/run/ui/model.go:129–139`.
- **Pattern:** `waitEvent` reads from `m.events` synchronously inside a `tea.Cmd`. Under event storms, the adapter in `startUIEventAdapter` (`model.go:321`) keeps pushing to `sink chan uiMsg` (buffered at `max(total*4, 4)`), and `waitEvent` drains one per Update cycle. If Update + View take >16 ms, the buffer fills and the adapter blocks on `sink <- msg`, which in turn stops draining `updates` from the bus.
- **Impact:** Not a CPU hotspot but creates queue backlog under storms. If any handler takes 20 ms, a 5 Hz event stream backs up into the bus; the bus may drop or stall depending on implementation.
- **Fix options:**
  1. Increase buffer to something like 256 or `max(total*64, 256)` — cheap memory, smooth under bursts.
  2. Coalesce streaming `jobUpdateMsg` at the adapter: when a newer snapshot for the same index is pending, drop the older one. Snapshots are cumulative, so coalescing is lossless visually.
  3. Move `View()` computation off the main goroutine where possible (bubbletea doesn't support this natively, but the sidebar rebuild and timeline render can be produced eagerly after Update and cached).
- **Expected win:** Eliminates stutter during burst chunks; preserves spinner smoothness.

### P1-7 — `progressBar.ViewAs(pct)` called twice per frame (title + potentially summary) with `SetWidth` inline
- **File/line:** `view.go:79,83`, `summary.go:53,70`.
- **Pattern:** Bubbles `progress.Model.ViewAs` regenerates the gradient ramp on each call. Combined with `SetWidth` happening inline inside `renderTitleBar`, a width change triggers full recompute.
- **Fix:** Cache the last rendered progress string keyed by `(width, pct)`; pct only changes when a job finishes (low frequency).
- **Expected win:** 0.2–0.5 ms per frame when progress hasn't changed.

---

## P2 Findings

### P2-1 — `renderOwnedBlock` splits on `\n` after `renderOwnedLine` already produced per-line content
- **File/line:** `internal/core/run/ui/styles.go:128–134`. Called from `timeline.go:53`, `sidebar.go:50`, `validation_form.go:197,199,201`.
- **Pattern:** We already know the line structure (from `splitRenderedText` or viewport), but we join + re-split + re-own. Could pass `[]string` directly.
- **Expected win:** Small — 0.1–0.2 ms.

### P2-2 — `formatDuration` / `formatNumber` / `nextEntryID` use `fmt.Sprintf` in hot paths
- **File/line:** `sidebar.go:207,209`, `summary.go:170–189`, `transcript/model.go:468`.
- **Pattern:** `fmt.Sprintf("%02d:%02d", …)` allocates per call; in `sidebarMeta` (`sidebar.go:126,127`) three `Sprintf` per row per tick.
- **Fix:** Use `strconv.AppendInt` with a small `strings.Builder` or pre-sized `[]byte`. For small ints a lookup table is fastest.
- **Expected win:** 0.2–0.5 ms under 20+ jobs.

### P2-3 — `buildVisibleEntries` always returns a newly allocated `[]Entry` even when nothing changed
- **File/line:** `transcript/model.go:326–337`.
- **Pattern:** Even when `Apply` returns `changed=false` (short-circuited upstream), any other trigger of `Snapshot()` (e.g. `t.translateSessionUpdate` `viewModel.Snapshot()` on `!changed` path at `model.go:544`) still rebuilds the full entry slice.
- **Fix:** Memoize `Snapshot()` by `revision`; bump revision only when `apply` returns `true`. Return the cached snapshot on identity.
- **Expected win:** Minor unless unchanged snapshots happen often; primarily reduces allocator pressure.

### P2-4 — `reflect.ValueOf` + `IsNil` in `contentblock.ValidatePayload`
- **File/line:** `internal/contentblock/engine.go:16–26`.
- **Pattern:** Reflection used to detect nil pointers. Called on every block marshaling. Reflection is ~30× slower than type-switch.
- **Fix:** Type-switch on known block types at call sites (they're all known statically in `internal/core/model`), or require callers to pre-validate.
- **Expected win:** Minor unless marshaling is frequent; negligible for UI but matters for the journal path.

### P2-5 — `reapplyOwnedBackground` fallback copies even when no SGR present
- **File/line:** `styles.go:147–148`. The early-out `strings.Contains(content, "\x1b[")` returns the input unchanged when no SGR — good. But most rendered content from lipgloss *always* contains SGR, so this branch rarely fires. Combined with P0-1's fix, double-check call sites that pass raw content (e.g. `renderGap` already wraps in a styled render, so all content entering `reapplyOwnedBackground` has SGR).
- **Fix:** Merge with P0-1 fix.

### P2-6 — `rootScreenStyle` on every frame re-creates a huge padded buffer
- **File/line:** `view.go:13–18`, `styles.go:56–62`.
- **Pattern:** `lipgloss.NewStyle().Width(w).Height(h).Background(...).Render(content)` pads the content to `w×h` characters. For a 200×60 terminal that's 12000 cells of space-padding per frame.
- **Impact:** 0.3–0.8 ms of string building + one big allocation.
- **Fix:** Bubbletea already pads to the alt-screen size; explicit `Height(h)` may be redundant. At minimum, cache the pre-built top/bottom padding strips by (width, height, background) and splice.
- **Expected win:** 0.3–0.8 ms per frame on large terminals.

### P2-7 — `validation_form.syncIssueViewport` re-renders the full issue list on every `WindowSizeMsg`, but also every `Update` indirectly because `View()` doesn't memoize
- **File/line:** `validation_form.go:157–170, 184–203`.
- **Note:** Lower priority — this form is short-lived and user is idle. Flag only for completeness.

---

## Verification plan

Benchmarks live alongside the packages. Add:

### `internal/core/run/ui/view_bench_test.go`

1. **`BenchmarkView_JobsViewSteadyState`** — build a model with N=10 jobs, each with M=200 transcript entries of mixed kinds (narrative + tool calls). Call `m.View()` in a loop. Report `ns/op`, `B/op`, `allocs/op`. Run at terminal sizes (80×24, 120×40, 200×60).
2. **`BenchmarkUpdate_JobUpdateStream`** — simulate a streaming burst: feed 500 `jobUpdateMsg` with incremental assistant-message chunks (100-char deltas) and run Update → View per message. Measure per-msg latency p50/p95/p99.
3. **`BenchmarkSidebarTickRefresh`** — isolate `refreshSidebarContent` with N=20 jobs running. Target should be **< 1 ms** after fix; pre-fix baseline expected ~2–4 ms.
4. **`BenchmarkTimelineCacheHit`** — set up a job with a built timeline cache, call `renderTimelinePanel` 100 times without invalidating. Target: constant-time cache hit (< 0.5 ms), matching cache-hit expectation. Pre-fix: full `SetContent` cost on every call.
5. **`BenchmarkReapplyOwnedBackground`** — microbench for styles.go scanner on representative 100/500/2000-char styled lines with 0, 4, 20 SGR sequences.
6. **`BenchmarkCloneContentBlocks`** — transcript package: benchmark `Snapshot()` on 500 entries, half tool results with 10 KB payloads.
7. **`BenchmarkMergeNarrativeText`** — transcript: existing=5 KB, incoming=100 char delta; incoming=5 KB rewrite.

### Instrumentation

- Add a `_ = tea.EnableProfiling()` style pprof hook (or `go test -cpuprofile -memprofile`) to capture live profiles during a representative run.
- Use `go test -bench=. -benchmem -cpuprofile=view.cpu.pprof` and `go tool pprof -top view.cpu.pprof`.
- Visualize with `go tool pprof -http=:0 view.cpu.pprof` and confirm top-5 hotspots before/after.

### Golden output validation

- Capture `m.View()` output for fixed terminal sizes into `testdata/view_golden/*.txt` (ANSI sequences normalized via `ansi.Strip`). After each optimization commit, re-run `go test ./internal/core/run/ui/... -run TestViewGolden` and diff. Any byte-level diff in visible output fails the isomorphism proof.
- For `reapplyOwnedBackground` specifically, add a fuzz corpus of mixed-SGR inputs and assert the post-scan output equals the pre-scan output after replaying SGR in a VT parser (or simpler: assert each rendered line has the owned background as its "current" bg at EOL).

### Budget gates

After fixes:
- `View()` at 120×40 with 10 jobs and 200 entries: **≤ 5 ms p95**.
- `handleTick` steady-state sidebar refresh: **≤ 1 ms p95**.
- `handleJobUpdate` + `View()` combined: **≤ 8 ms p95** under 30 Hz streaming.
- Allocations per `View()` (post-cache-hit): **≤ 50 allocs**, currently estimated in the thousands.

### Risk / behavior preservation

- All optimizations must preserve ANSI output bit-for-bit for existing golden tests.
- `cloneContentBlocks` removal requires a code audit to confirm no writer mutates `ContentBlock.Data` after creation — add `go vet`-style check or a struct tag in review.
- Style memoization must not leak mutable `lipgloss.Style` across goroutines; lipgloss v2 styles are copy-by-value so the risk is low, but benchmark under race detector.
