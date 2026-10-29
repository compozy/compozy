package terminal

import "bytes"

// Primary Device Attributes (DA1). Line editors such as fish 4.1+ query the
// terminal at startup and wait for this answer before drawing a prompt. A
// browser viewer's emulator answers while it is attached for writing; with no
// writer (an agent- or CLI-opened PTY nobody watches yet) the daemon answers
// for it, as the VT100-with-AVO class the browser emulator reports, so the
// shell does not stall and print a timeout warning.
var primaryDeviceAttributesQueries = [][]byte{[]byte("\x1b[c"), []byte("\x1b[0c")}

const primaryDeviceAttributesReply = "\x1b[?1;2c"

// deviceQueryCarry keeps the tail a query split across reads needs.
const deviceQueryCarry = 3

func containsPrimaryDeviceAttributesQuery(output []byte) bool {
	for _, query := range primaryDeviceAttributesQueries {
		if bytes.Contains(output, query) {
			return true
		}
	}
	return false
}

// answerHeadlessDeviceQuery runs on the output reader goroutine only.
func (s *session) answerHeadlessDeviceQuery(output []byte) {
	if len(output) == 0 {
		return
	}
	// Only the seam between reads needs the previous tail; the read itself is scanned in place.
	seam := append(s.deviceQueryTail, output[:min(len(output), deviceQueryCarry)]...)
	found := containsPrimaryDeviceAttributesQuery(seam) || containsPrimaryDeviceAttributesQuery(output)
	s.deviceQueryTail = s.deviceQueryTail[:0]
	if !found {
		// A query already seen must not be found again in the next seam.
		s.deviceQueryTail = append(s.deviceQueryTail, output[max(0, len(output)-deviceQueryCarry):]...)
		return
	}
	if s.hasWriteSubscriber() {
		return
	}
	if err := writeAllInput([]byte(primaryDeviceAttributesReply), s.proc.Write); err != nil {
		s.manager.logger.Debug("terminal: answer device attributes query", "terminal_id", s.info.ID, "error", err)
	}
}

func (s *session) hasWriteSubscriber() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, subscriber := range s.subscribers {
		if subscriber.mode == terminalAccessWrite {
			return true
		}
	}
	return false
}
