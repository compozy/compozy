package ui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
	xansi "github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestCaptureSidebarSpacingScreenshot(t *testing.T) {
	outDir := os.Getenv("COMPOZY_SIDEBAR_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("set COMPOZY_SIDEBAR_SCREENSHOT_DIR to write sidebar spacing artifacts")
	}
	forceTrueColorForTest(t)
	m := newSidebarSpacingFixtureModel(t)
	assertSidebarStackHasNoGapOrAccentLeak(t, m.sidebarContent, m.sidebarViewport.Width(), len(m.jobs))

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create screenshot output directory: %v", err)
	}
	artifacts := map[string][]byte{
		"sidebar-spacing.ansi": []byte(m.renderSidebar() + "\n"),
		"sidebar-spacing.txt":  []byte(xansi.Strip(m.renderSidebar()) + "\n"),
		"sidebar-spacing.svg":  []byte(renderTerminalSVG("Compozy execution sidebar", m.renderSidebar())),
	}
	for name, content := range artifacts {
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
	}
}

func forceTrueColorForTest(t *testing.T) {
	t.Helper()
	previous := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() {
		lipgloss.Writer.Profile = previous
	})
}

func newSidebarSpacingFixtureModel(t *testing.T) *uiModel {
	t.Helper()
	m := newUIModel(6)
	m.handleWindowSize(tea.WindowSizeMsg{Width: 160, Height: 34})
	titles := []string{
		"Migrar edição de Audiências para modo inline no Sheet",
		"Calendário-mês de prazos com eventos consolidados",
		"WhatsApp inbox - rail de seleção, busca e filtros",
		"WhatsApp analytics - tracking/size dos funis",
		"Settings shell - coluna única 940px e navegação",
		"Settings tabelas/controls - membros e permissões",
	}
	for i, title := range titles {
		m.handleJobQueued(&jobQueuedMsg{
			Index:    i,
			CodeFile: fmt.Sprintf("task_%02d.md", i+1),
			SafeName: fmt.Sprintf("task_%02d-safe", i+1),
		})
		m.jobs[i].taskNumber = i + 1
		m.jobs[i].taskTitle = title
		m.jobs[i].taskType = "frontend"
	}
	fixtureNow := time.Date(2026, 6, 14, 21, 26, 26, 0, time.UTC)
	m.jobs[0].state = jobRunning
	m.now = fixtureNow
	m.jobs[0].startedAt = fixtureNow.Add(-11 * time.Second)
	m.selectedJob = 0
	m.sidebarDirty = true
	m.refreshSidebarContent()
	return m
}

func assertSidebarStackHasNoGapOrAccentLeak(t *testing.T, content string, width, items int) {
	t.Helper()
	rawLines := strings.Split(content, "\n")
	strippedLines := strings.Split(xansi.Strip(content), "\n")
	if got, want := len(strippedLines), sidebarRowLines+(items-1)*sidebarRowStride; got != want {
		t.Fatalf("expected shared-border sidebar content height %d, got %d: %q", want, got, xansi.Strip(content))
	}
	for i, line := range rawLines {
		if got := xansi.StringWidth(line); got != width {
			t.Fatalf("expected stacked sidebar line %d width %d, got %d: %q", i, width, got, xansi.Strip(line))
		}
	}
	assertNoAccentBorderGlyphs(t, rawLines)
	for i := 0; i < len(strippedLines)-1; i++ {
		current := strings.TrimSpace(strippedLines[i])
		next := strings.TrimSpace(strippedLines[i+1])
		if current == "" || next == "" {
			t.Fatalf("expected no blank gap between card rows at lines %d/%d: %q", i, i+1, xansi.Strip(content))
		}
		if strings.HasPrefix(current, "└") && strings.HasPrefix(next, "┌") {
			t.Fatalf(
				"expected shared border between cards, got duplicated borders at lines %d/%d: %q",
				i,
				i+1,
				xansi.Strip(content),
			)
		}
	}
	wantSeparator := sidebarCardSeparator(width, colorBorder)
	leakingSeparator := sidebarCardSeparator(width, colorAccent)
	for row := sidebarRowLines - 1; row < len(rawLines)-1; row += sidebarRowStride {
		if rawLines[row] == leakingSeparator {
			t.Fatalf("separator line %d leaks selected accent color: %q", row, rawLines[row])
		}
		if rawLines[row] != wantSeparator {
			t.Fatalf("separator line %d = %q, want muted separator %q", row, rawLines[row], wantSeparator)
		}
	}
}

func assertNoAccentBorderGlyphs(t *testing.T, rawLines []string) {
	t.Helper()
	const (
		accentHex    = "#A3E635"
		borderGlyphs = "┌┐└┘├┤│─"
	)
	for row, line := range rawLines {
		for _, span := range ansiSVGSpans(line) {
			if span.fg == accentHex && strings.ContainsAny(span.text, borderGlyphs) {
				t.Fatalf("line %d leaks accent color into sidebar border glyphs: %q", row, line)
			}
		}
	}
}

func renderTerminalSVG(title, content string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	maxCols := 1
	for _, line := range lines {
		if width := xansi.StringWidth(line); width > maxCols {
			maxCols = width
		}
	}
	const (
		fontSize   = 16
		cellWidth  = 9.6
		lineHeight = 22
		marginX    = 18
		marginY    = 28
	)
	width := int(float64(maxCols)*cellWidth) + 2*marginX
	height := len(lines)*lineHeight + 2*marginY
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n",
		width,
		height,
		width,
		height,
	)
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<rect width=\"100%%\" height=\"100%%\" fill=\"#1f1d2e\"/>\n")
	fmt.Fprintf(
		&b,
		"<g font-family=\"SFMono-Regular, Menlo, Monaco, Consolas, monospace\" font-size=\"%d\" xml:space=\"preserve\">\n",
		fontSize,
	)
	for i, line := range lines {
		y := marginY + i*lineHeight
		fmt.Fprintf(&b, "<text x=\"%d\" y=\"%d\">", marginX, y)
		for _, span := range ansiSVGSpans(line) {
			weight := "400"
			if span.bold {
				weight = "700"
			}
			fmt.Fprintf(
				&b,
				"<tspan fill=\"%s\" font-weight=\"%s\">%s</tspan>",
				span.fg,
				weight,
				html.EscapeString(span.text),
			)
		}
		b.WriteString("</text>\n")
	}
	b.WriteString("</g>\n</svg>\n")
	return b.String()
}

type ansiSVGSpan struct {
	text string
	fg   string
	bold bool
}

func ansiSVGSpans(line string) []ansiSVGSpan {
	style := ansiSVGSpan{fg: "#E7E5E4"}
	spans := make([]ansiSVGSpan, 0, 8)
	var text strings.Builder
	flush := func() {
		if text.Len() == 0 {
			return
		}
		spans = append(spans, ansiSVGSpan{text: text.String(), fg: style.fg, bold: style.bold})
		text.Reset()
	}
	for i := 0; i < len(line); {
		if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
			end := strings.IndexByte(line[i+2:], 'm')
			if end >= 0 {
				flush()
				applySGR(&style, line[i+2:i+2+end])
				i += end + 3
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		if size == 0 {
			break
		}
		text.WriteString(line[i : i+size])
		i += size
	}
	flush()
	return spans
}

func applySGR(style *ansiSVGSpan, seq string) {
	if seq == "" {
		seq = "0"
	}
	parts := strings.Split(seq, ";")
	params := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			value = 0
		}
		params = append(params, value)
	}
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case 0:
			style.fg = "#E7E5E4"
			style.bold = false
		case 1:
			style.bold = true
		case 2:
			style.fg = "#78716C"
		case 22:
			style.bold = false
		case 39:
			style.fg = "#E7E5E4"
		case 38:
			if i+4 < len(params) && params[i+1] == 2 {
				style.fg = fmt.Sprintf(
					"#%02X%02X%02X",
					clampColor(params[i+2]),
					clampColor(params[i+3]),
					clampColor(params[i+4]),
				)
				i += 4
			}
		}
	}
}

func clampColor(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}
