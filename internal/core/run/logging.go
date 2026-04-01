package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/tidwall/pretty"
)

func buildCommandTaps(
	outF, errF *os.File,
	tailLines int,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	monitor *activityMonitor,
) (io.Writer, io.Writer) {
	outRing := newLineRing(tailLines)
	errRing := newLineRing(tailLines)
	if useUI {
		return buildUITaps(outF, errF, outRing, errRing, uiCh, index, ideType, aggregateUsage, aggregateMu, monitor)
	}
	return buildCLITaps(outF, errF, ideType, aggregateUsage, aggregateMu, monitor)
}

func buildUITaps(
	outF, errF *os.File,
	outRing, errRing *lineRing,
	uiCh chan uiMsg,
	index int,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	monitor *activityMonitor,
) (io.Writer, io.Writer) {
	uiTap := newUILogTap(index, false, outRing, errRing, uiCh, monitor)
	var outTap io.Writer
	if ideType == model.IDEClaude || ideType == model.IDECursor || ideType == model.IDEDroid {
		usageCallback := func(usage TokenUsage) {
			if uiCh != nil {
				uiCh <- tokenUsageUpdateMsg{Index: index, Usage: usage}
			}
			if aggregateUsage != nil && aggregateMu != nil {
				aggregateMu.Lock()
				aggregateUsage.Add(usage)
				aggregateMu.Unlock()
			}
		}
		outTap = io.MultiWriter(outF, newJSONFormatterWithCallbackAndMonitor(uiTap, usageCallback, monitor))
	} else {
		outTap = io.MultiWriter(outF, uiTap)
	}

	uiErrTap := io.Writer(newUILogTap(index, true, outRing, errRing, uiCh, monitor))
	if ideType == model.IDECodex {
		uiErrTap = newLineFilterWriter(uiErrTap, monitor, shouldSuppressCodexRolloutStderrLine)
	}
	errTap := io.MultiWriter(errF, uiErrTap)
	return outTap, errTap
}

func buildCLITaps(
	outF, errF *os.File,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	monitor *activityMonitor,
) (io.Writer, io.Writer) {
	if ideType == model.IDEClaude || ideType == model.IDECursor || ideType == model.IDEDroid {
		usageCallback := func(usage TokenUsage) {
			if aggregateUsage != nil && aggregateMu != nil {
				aggregateMu.Lock()
				aggregateUsage.Add(usage)
				aggregateMu.Unlock()
			}
		}
		return io.MultiWriter(
				outF,
				newJSONFormatterWithCallbackAndMonitor(os.Stdout, usageCallback, monitor),
			), io.MultiWriter(
				errF,
				os.Stderr,
			)
	}

	stderrWriter := io.Writer(os.Stderr)
	if ideType == model.IDECodex {
		stderrWriter = newLineFilterWriter(os.Stderr, monitor, shouldSuppressCodexRolloutStderrLine)
	}
	return io.MultiWriter(outF, os.Stdout), io.MultiWriter(errF, stderrWriter)
}

func shouldSuppressCodexRolloutStderrLine(line string) bool {
	return strings.Contains(line, "codex_core::rollout::list") &&
		strings.Contains(line, "state db missing rollout path for thread")
}

type lineFilterWriter struct {
	dst             io.Writer
	buf             []byte
	shouldDrop      func(string) bool
	activityMonitor *activityMonitor
}

func newLineFilterWriter(
	dst io.Writer,
	monitor *activityMonitor,
	shouldDrop func(string) bool,
) *lineFilterWriter {
	return &lineFilterWriter{
		dst:             dst,
		buf:             make([]byte, 0, 1024),
		shouldDrop:      shouldDrop,
		activityMonitor: monitor,
	}
}

func (w *lineFilterWriter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.activityMonitor != nil {
		w.activityMonitor.recordActivity()
	}
	cleaned := bytes.ReplaceAll(p, []byte{'\r'}, []byte{'\n'})
	w.buf = append(w.buf, cleaned...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		rawLine := bytes.TrimRight(w.buf[:i], "\r\n")
		if !w.shouldDrop(string(rawLine)) {
			if _, err := w.dst.Write(append(rawLine, '\n')); err != nil {
				return 0, err
			}
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

type lineRing struct {
	mu    sync.Mutex
	capN  int
	lines []string
}

func newLineRing(n int) *lineRing {
	if n <= 0 {
		n = 1
	}
	return &lineRing{capN: n, lines: make([]string, 0, n)}
}

func (r *lineRing) appendLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s == "" {
		return
	}
	r.lines = append(r.lines, s)
	if len(r.lines) > r.capN {
		r.lines = r.lines[len(r.lines)-r.capN:]
	}
}

func (r *lineRing) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

type activityMonitor struct {
	mu           sync.Mutex
	lastActivity time.Time
}

func newActivityMonitor() *activityMonitor {
	return &activityMonitor{lastActivity: time.Now()}
}

func (a *activityMonitor) recordActivity() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

func (a *activityMonitor) timeSinceLastActivity() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastActivity)
}

type uiLogTap struct {
	idx             int
	isErr           bool
	out             *lineRing
	err             *lineRing
	ch              chan<- uiMsg
	buf             []byte
	activityMonitor *activityMonitor
}

func newUILogTap(
	idx int,
	isErr bool,
	outRing, errRing *lineRing,
	ch chan<- uiMsg,
	monitor *activityMonitor,
) *uiLogTap {
	return &uiLogTap{
		idx:             idx,
		isErr:           isErr,
		out:             outRing,
		err:             errRing,
		ch:              ch,
		buf:             make([]byte, 0, 1024),
		activityMonitor: monitor,
	}
}

func (t *uiLogTap) Write(p []byte) (int, error) {
	if len(p) > 0 && t.activityMonitor != nil {
		t.activityMonitor.recordActivity()
	}
	cleaned := bytes.ReplaceAll(p, []byte{'\r'}, []byte{'\n'})
	t.buf = append(t.buf, cleaned...)
	for {
		i := bytes.IndexByte(t.buf, '\n')
		if i < 0 {
			break
		}
		line := string(bytes.TrimRight(t.buf[:i], "\r\n"))
		if t.isErr {
			t.err.appendLine(line)
		} else {
			t.out.appendLine(line)
		}
		t.buf = t.buf[i+1:]
	}
	select {
	case t.ch <- jobLogUpdateMsg{Index: t.idx, Out: t.out.snapshot(), Err: t.err.snapshot()}:
	default:
	}
	return len(p), nil
}

type jsonFormatter struct {
	w               io.Writer
	buf             []byte
	usageCallback   func(TokenUsage)
	activityMonitor *activityMonitor
}

func newJSONFormatterWithCallbackAndMonitor(
	w io.Writer,
	callback func(TokenUsage),
	monitor *activityMonitor,
) *jsonFormatter {
	return &jsonFormatter{
		w:               w,
		buf:             make([]byte, 0, 4096),
		usageCallback:   callback,
		activityMonitor: monitor,
	}
}

func (f *jsonFormatter) Write(p []byte) (int, error) {
	if len(p) > 0 && f.activityMonitor != nil {
		f.activityMonitor.recordActivity()
	}
	f.buf = append(f.buf, p...)
	for {
		i := bytes.IndexByte(f.buf, '\n')
		if i < 0 {
			break
		}
		line := bytes.TrimRight(f.buf[:i], "\r\n")
		f.buf = f.buf[i+1:]
		formatted := f.formatLine(line)
		if _, err := f.w.Write(append(formatted, '\n')); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (f *jsonFormatter) formatLine(line []byte) []byte {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return line
	}
	if !json.Valid(line) {
		return line
	}

	var msg ClaudeMessage
	if err := json.Unmarshal(line, &msg); err == nil {
		if f.usageCallback != nil {
			f.tryParseUsage(&msg)
		}
		if formatted := f.formatClaudeMessage(&msg); formatted != nil {
			return formatted
		}
	}

	return pretty.Color(pretty.Pretty(line), nil)
}

func (f *jsonFormatter) formatClaudeMessage(msg *ClaudeMessage) []byte {
	switch msg.Type {
	case "user", "assistant":
		if len(msg.Message.Content) > 0 {
			var contentParts []string
			for _, content := range msg.Message.Content {
				if content.Type == "text" && content.Text != "" {
					contentParts = append(contentParts, content.Text)
				} else if content.Type == "tool_result" && content.Content != "" {
					contentParts = append(contentParts, content.Content)
				}
			}
			if len(contentParts) > 0 {
				return []byte(strings.Join(contentParts, "\n"))
			}
		}
	case "system":
		return []byte(fmt.Sprintf("[System: %s]", msg.Type))
	}
	return nil
}

func (f *jsonFormatter) tryParseUsage(msg *ClaudeMessage) {
	if msg.Type != "assistant" {
		return
	}
	usage := msg.Message.Usage
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	tokenUsage := TokenUsage{
		InputTokens:         usage.InputTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		OutputTokens:        usage.OutputTokens,
		Ephemeral5mTokens:   usage.CacheCreation.Ephemeral5mTokens,
		Ephemeral1hTokens:   usage.CacheCreation.Ephemeral1hTokens,
	}
	f.usageCallback(tokenUsage)
}
