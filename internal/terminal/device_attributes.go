package terminal

// Primary Device Attributes (DA1). Line editors such as fish 4.1+ query the
// terminal at startup and wait for this answer before drawing a prompt. A
// browser viewer's emulator answers while it is attached for writing; with no
// writer (an agent- or CLI-opened PTY nobody watches yet) the daemon answers
// for it, as the VT100-with-AVO class the browser emulator reports, so the
// shell does not stall and print a timeout warning.
const primaryDeviceAttributesReply = "\x1b[?1;2c"

// deviceReplyQueue bounds pending replies; a shell asks once at startup.
const deviceReplyQueue = 4

// deviceQueryState is where the scanner stands inside `ESC [ [0] c`.
type deviceQueryState uint8

const (
	deviceQueryIdle deviceQueryState = iota
	deviceQueryEscape
	deviceQueryIntroducer
	deviceQueryZero
)

// deviceQueryScanner counts complete DA1 queries across reads, so a query
// split over any number of reads, or several in one read, each count once.
type deviceQueryScanner struct {
	state deviceQueryState
}

func (d *deviceQueryScanner) Scan(output []byte) int {
	found := 0
	for _, value := range output {
		switch {
		case value == 0x1b:
			d.state = deviceQueryEscape
		case d.state == deviceQueryEscape && value == '[':
			d.state = deviceQueryIntroducer
		case d.state == deviceQueryIntroducer && value == '0':
			d.state = deviceQueryZero
		case (d.state == deviceQueryIntroducer || d.state == deviceQueryZero) && value == 'c':
			found++
			d.state = deviceQueryIdle
		default:
			d.state = deviceQueryIdle
		}
	}
	return found
}

// answerHeadlessDeviceQuery runs on the output reader goroutine only. It never
// writes to the PTY itself: a blocked input write must not stop output draining.
func (s *session) answerHeadlessDeviceQuery(output []byte) {
	queries := s.deviceQueries.Scan(output)
	if queries == 0 || s.hasWriteSubscriber() {
		return
	}
	s.deviceReplyOnce.Do(s.startDeviceReplies)
	for range queries {
		select {
		case s.deviceReplies <- struct{}{}:
		default:
			s.manager.logger.Debug("terminal: device query reply queue full", "terminal_id", s.info.ID)
		}
	}
}

func (s *session) startDeviceReplies() {
	s.deviceReplies = make(chan struct{}, deviceReplyQueue)
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.deviceReplies:
				if err := writeAllInput([]byte(primaryDeviceAttributesReply), s.proc.Write); err != nil {
					s.manager.logger.Debug(
						"terminal: answer device attributes query", "terminal_id", s.info.ID, "error", err,
					)
				}
			}
		}
	}()
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
