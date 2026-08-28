package terminal

import (
	"errors"
	"unicode/utf8"

	terminalvt "github.com/compozy/compozy/internal/terminal/vt"
)

func (s *session) markReaderEnded() {
	if len(s.vtCarry) > 0 {
		_, end := s.ring.Snapshot()
		if _, err := s.vt.WriteAt(s.vtCarry, end); err != nil && !errors.Is(err, terminalvt.ErrClosed) {
			s.manager.logger.Debug("terminal: flush emulator carry", "terminal_id", s.info.ID, "error", err)
		}
		s.vtCarry = nil
	}
	s.mu.Lock()
	s.readerEnded = true
	s.bumpRevisionLocked()
	s.mu.Unlock()
}

func splitCompleteUTF8(input []byte) ([]byte, []byte) {
	for index := 0; index < len(input); {
		if input[index] < utf8.RuneSelf {
			index++
			continue
		}
		_, size := utf8.DecodeRune(input[index:])
		if size == 1 && !utf8.FullRune(input[index:]) {
			return input[:index], input[index:]
		}
		index += size
	}
	return input, nil
}

func (s *session) bumpRevisionLocked() {
	s.revision++
	close(s.revisionReady)
	s.revisionReady = make(chan struct{})
}
