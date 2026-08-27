package terminal

func (s *session) capturedOutput() ([]byte, bool, int64) {
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	return append([]byte(nil), s.capture...), s.captureTruncated, s.captureBytes
}

func (s *session) appendCaptureLocked(input []byte) {
	s.captureBytes += int64(len(input))
	if !s.captureTruncated && len(s.capture)+len(input) <= execCaptureLimit {
		s.capture = append(s.capture, input...)
		return
	}
	headSize := execCaptureLimit / 2
	tailSize := execCaptureLimit - headSize
	if !s.captureTruncated {
		combined := make([]byte, 0, len(s.capture)+len(input))
		combined = append(combined, s.capture...)
		combined = append(combined, input...)
		s.capture = append(s.capture[:0], combined[:headSize]...)
		s.capture = append(s.capture, combined[len(combined)-tailSize:]...)
		s.captureTruncated = true
		return
	}
	tail := make([]byte, 0, tailSize+len(input))
	tail = append(tail, s.capture[headSize:]...)
	tail = append(tail, input...)
	s.capture = s.capture[:headSize]
	if len(tail) > tailSize {
		tail = tail[len(tail)-tailSize:]
	}
	s.capture = append(s.capture, tail...)
}
