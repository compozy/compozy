package vt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	charmvt "github.com/charmbracelet/x/vt"
)

const DefaultMailboxBytes = 256 * 1024

var ErrClosed = errors.New("terminal vt actor closed")

type RingSnapshot func() ([]byte, uint64)

type Snapshot struct {
	Content string
	Seq     uint64
	Busy    bool
	Ended   bool
}

type commandKind uint8

const (
	commandScreen commandKind = iota + 1
	commandResize
)

type command struct {
	kind     commandKind
	cols     int
	rows     int
	snapshot chan Snapshot
	err      chan error
}

type writeMessage struct {
	data []byte
	end  uint64
}

type Actor struct {
	commands   chan command
	writes     chan writeMessage
	wake       chan struct{}
	done       chan struct{}
	capacity   int64
	pending    atomic.Int64
	dirty      atomic.Bool
	rebuilding atomic.Bool
	closing    atomic.Bool
	closed     atomic.Bool
	sequence   atomic.Uint64
	closeOnce  sync.Once
	closeErr   error
	finalMu    sync.RWMutex
	final      Snapshot
}

func New(cols, rows int, ring RingSnapshot) *Actor {
	actor := &Actor{
		commands: make(chan command, 64),
		writes:   make(chan writeMessage, 256),
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		capacity: DefaultMailboxBytes,
	}
	go actor.loop(cols, rows, ring)
	return actor
}

func (a *Actor) Write(input []byte) (int, error) {
	end := a.sequence.Add(uint64(len(input)))
	return a.WriteAt(input, end)
}

// WriteAt queues bytes with the ring's absolute end sequence.
func (a *Actor) WriteAt(input []byte, end uint64) (int, error) {
	if len(input) == 0 {
		return 0, nil
	}
	if a.closing.Load() || a.closed.Load() {
		return 0, ErrClosed
	}
	copyOfInput := append([]byte(nil), input...)
	if a.pending.Add(int64(len(copyOfInput))) > a.capacity {
		a.pending.Add(-int64(len(copyOfInput)))
		a.markDirty()
		return len(input), nil
	}
	select {
	case a.writes <- writeMessage{data: copyOfInput, end: end}:
		return len(input), nil
	default:
		a.pending.Add(-int64(len(copyOfInput)))
		a.markDirty()
		return len(input), nil
	}
}

func (a *Actor) Screen(ctx context.Context) (Snapshot, error) {
	if a.closed.Load() {
		return a.finalSnapshot(), nil
	}
	if a.dirty.Load() || a.rebuilding.Load() {
		return Snapshot{Busy: true}, nil
	}
	response := make(chan Snapshot, 1)
	if err := a.send(ctx, command{kind: commandScreen, snapshot: response}); err != nil {
		if errors.Is(err, ErrClosed) {
			return a.finalSnapshot(), nil
		}
		return Snapshot{}, err
	}
	select {
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	case <-a.done:
		return a.finalSnapshot(), nil
	case snapshot := <-response:
		return snapshot, nil
	}
}

func (a *Actor) Resize(ctx context.Context, cols, rows int) error {
	response := make(chan error, 1)
	if err := a.send(ctx, command{kind: commandResize, cols: cols, rows: rows, err: response}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.done:
		return ErrClosed
	case err := <-response:
		return err
	}
}

func (a *Actor) Close() error {
	a.closeOnce.Do(func() {
		a.closing.Store(true)
		select {
		case a.wake <- struct{}{}:
		default:
		}
	})
	<-a.done
	return a.closeErr
}

func (a *Actor) send(ctx context.Context, request command) error {
	if a.closed.Load() {
		return ErrClosed
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.done:
		return ErrClosed
	case a.commands <- request:
		if a.closed.Load() {
			return ErrClosed
		}
		return nil
	}
}

func (a *Actor) markDirty() {
	a.dirty.Store(true)
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func (a *Actor) loop(cols, rows int, ring RingSnapshot) {
	emulator, drainDone := startEmulator(cols, rows)
	applied := uint64(0)
	ended := false
	if ring != nil {
		initial, sequence := ring()
		if len(initial) > 0 {
			if _, err := emulator.Write(initial); err != nil {
				ended = true
			}
		}
		applied = sequence
	}
	defer close(a.done)
	for {
		if a.closing.Load() {
			a.finishClose(emulator, drainDone, applied)
			return
		}
		if a.dirty.CompareAndSwap(true, false) && !ended {
			a.rebuilding.Store(true)
			var err error
			emulator, drainDone, applied, err = rebuildEmulator(
				emulator,
				drainDone,
				cols,
				rows,
				ring,
				a.closing.Load,
			)
			if err != nil {
				ended = !errors.Is(err, ErrClosed)
			}
			a.rebuilding.Store(false)
		}
		select {
		case <-a.wake:
			continue
		case message := <-a.writes:
			a.pending.Add(-int64(len(message.data)))
			if !ended {
				start := message.end - uint64(len(message.data))
				if message.end > applied {
					input := message.data
					if start < applied {
						input = input[applied-start:]
					}
					if _, err := emulator.Write(input); err != nil {
						ended = true
					}
					applied = message.end
				}
			}
		case request := <-a.commands:
			switch request.kind {
			case commandScreen:
				if a.dirty.Load() || a.rebuilding.Load() {
					request.snapshot <- Snapshot{Seq: applied, Busy: true, Ended: ended}
				} else {
					request.snapshot <- Snapshot{Content: screenText(emulator), Seq: applied, Ended: ended}
				}
			case commandResize:
				cols, rows = normalizeSize(request.cols, request.rows)
				if ended {
					request.err <- ErrClosed
					continue
				}
				emulator.Resize(cols, rows)
				request.err <- nil
			}
		}
	}
}

func (a *Actor) finishClose(emulator *charmvt.Emulator, drainDone <-chan error, applied uint64) {
	a.closed.Store(true)
	a.setFinal(Snapshot{Content: screenText(emulator), Seq: applied, Ended: true})
	a.closeErr = shutdownEmulator(emulator, drainDone)
}

func startEmulator(cols, rows int) (*charmvt.Emulator, <-chan error) {
	cols, rows = normalizeSize(cols, rows)
	emulator := charmvt.NewEmulator(cols, rows)
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, emulator)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
			err = nil
		}
		done <- err
	}()
	return emulator, done
}

func rebuildEmulator(
	current *charmvt.Emulator,
	drainDone <-chan error,
	cols, rows int,
	ring RingSnapshot,
	interrupted func() bool,
) (*charmvt.Emulator, <-chan error, uint64, error) {
	if err := shutdownEmulator(current, drainDone); err != nil {
		return current, drainDone, 0, err
	}
	next, nextDrain := startEmulator(cols, rows)
	if ring == nil {
		return next, nextDrain, 0, nil
	}
	data, sequence := ring()
	const chunkBytes = 4 * 1024
	for offset := 0; offset < len(data); offset += chunkBytes {
		if interrupted != nil && interrupted() {
			return next, nextDrain, sequence, ErrClosed
		}
		end := min(offset+chunkBytes, len(data))
		if _, err := next.Write(data[offset:end]); err != nil {
			shutdownErr := shutdownEmulator(next, nextDrain)
			return next, nextDrain, sequence, errors.Join(fmt.Errorf("terminal vt: rebuild: %w", err), shutdownErr)
		}
	}
	return next, nextDrain, sequence, nil
}

func (a *Actor) setFinal(snapshot Snapshot) {
	a.finalMu.Lock()
	a.final = snapshot
	a.finalMu.Unlock()
}

func (a *Actor) finalSnapshot() Snapshot {
	a.finalMu.RLock()
	defer a.finalMu.RUnlock()
	return a.final
}

func shutdownEmulator(emulator *charmvt.Emulator, drainDone <-chan error) error {
	if emulator == nil {
		return nil
	}
	closer, ok := emulator.InputPipe().(io.Closer)
	if !ok {
		return errors.New("terminal vt: input pipe is not closable")
	}
	inputErr := closer.Close()
	drainErr := <-drainDone
	closeErr := emulator.Close()
	return errors.Join(inputErr, drainErr, closeErr)
}

func screenText(emulator *charmvt.Emulator) string {
	if emulator == nil {
		return ""
	}
	lines := make([]string, 0, emulator.Height())
	for y := 0; y < emulator.Height(); y++ {
		var line strings.Builder
		for x := 0; x < emulator.Width(); {
			cell := emulator.CellAt(x, y)
			if cell == nil {
				line.WriteByte(' ')
				x++
				continue
			}
			line.WriteString(cell.String())
			width := cell.Width
			if width < 1 {
				width = 1
			}
			x += width
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func normalizeSize(cols, rows int) (int, int) {
	if cols < 20 {
		cols = 20
	}
	if cols > 2000 {
		cols = 2000
	}
	if rows < 5 {
		rows = 5
	}
	if rows > 1000 {
		rows = 1000
	}
	return cols, rows
}
