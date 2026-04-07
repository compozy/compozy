package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"

	"github.com/compozy/compozy/pkg/compozy/events"
)

// Replay yields all events from fromSeq to EOF, tolerating a truncated final line.
func (r *Run) Replay(fromSeq uint64) iter.Seq2[events.Event, error] {
	return func(yield func(events.Event, error) bool) {
		if r == nil {
			yield(events.Event{}, errors.New("replay run: nil run"))
			return
		}

		file, err := os.Open(r.eventsPath)
		if err != nil {
			yield(events.Event{}, fmt.Errorf("open run events: %w", err))
			return
		}
		defer func() {
			_ = file.Close()
		}()

		partialFinalLine, err := fileHasPartialFinalLine(file)
		if err != nil {
			yield(events.Event{}, fmt.Errorf("inspect run events: %w", err))
			return
		}

		stopReplay := errors.New("stop replay")
		stopPendingDecode := errors.New("stop pending decode")
		var pendingDecodeErr error
		var pendingLine int

		readErr := forEachEventLine(file, func(rawLine []byte, lineNumber int) error {
			line := bytesTrimSpace(rawLine)
			if len(line) == 0 {
				return nil
			}

			ev, err := decodeEventLine(line, lineNumber)
			if err != nil {
				pendingDecodeErr = err
				pendingLine = lineNumber
				return nil
			}
			if pendingDecodeErr != nil {
				return stopPendingDecode
			}
			if ev.Seq < fromSeq {
				return nil
			}
			if !yield(ev, nil) {
				return stopReplay
			}
			return nil
		})
		switch {
		case errors.Is(readErr, stopReplay):
			return
		case errors.Is(readErr, stopPendingDecode):
			yield(events.Event{}, pendingDecodeErr)
			return
		case readErr != nil:
			yield(events.Event{}, fmt.Errorf("scan run events: %w", readErr))
			return
		}
		if pendingDecodeErr == nil {
			return
		}
		if partialFinalLine {
			yield(events.Event{}, fmt.Errorf("%w: line %d", ErrPartialEventLine, pendingLine))
			return
		}
		yield(events.Event{}, pendingDecodeErr)
	}
}

func decodeEventLine(line []byte, lineNumber int) (events.Event, error) {
	var ev events.Event
	if err := json.Unmarshal(line, &ev); err != nil {
		return events.Event{}, fmt.Errorf("decode run event line %d: %w", lineNumber, err)
	}
	if err := validateSchemaVersion(ev.SchemaVersion); err != nil {
		return events.Event{}, err
	}
	return ev, nil
}

func fileHasPartialFinalLine(file *os.File) (bool, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() == 0 {
		return false, nil
	}

	var tail [1]byte
	if _, err := file.ReadAt(tail[:], info.Size()-1); err != nil {
		return false, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	return tail[0] != '\n', nil
}
