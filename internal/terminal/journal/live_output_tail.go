package journal

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/redact"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const maximumLiveOutputTails = 256

func prepareLiveOutputTail(segments []terminalpkg.OutputSegment) ([]terminalpkg.OutputSegment, error) {
	cleaned := make([]terminalpkg.OutputSegment, len(segments))
	for index, segment := range segments {
		cleaned[index] = segment
		if segment.Kind == terminalpkg.OutputSegmentBytes {
			cleaned[index].Text = redact.String(segment.Text)
		}
	}
	cleaned = terminalpkg.BoundedOutputTail(cleaned)
	if err := validateOutputTail(cleaned); err != nil {
		return nil, err
	}
	return cleaned, nil
}

func (s *Service) retainLiveOutputTail(
	workspaceID, terminalID, commandID string,
	segments []terminalpkg.OutputSegment,
) {
	key := liveOutputTailKey(workspaceID, commandID)
	s.liveTailMu.Lock()
	defer s.liveTailMu.Unlock()
	if _, exists := s.liveTails[key]; !exists {
		s.liveTailOrder = append(s.liveTailOrder, key)
	}
	s.liveTails[key] = slices.Clone(segments)
	s.liveTailTerminals[key] = strings.TrimSpace(terminalID)
	for len(s.liveTailOrder) > maximumLiveOutputTails {
		oldest := s.liveTailOrder[0]
		s.liveTailOrder = s.liveTailOrder[1:]
		delete(s.liveTails, oldest)
		delete(s.liveTailTerminals, oldest)
	}
}

func (s *Service) liveOutputTail(workspaceID, commandID string) []terminalpkg.OutputSegment {
	s.liveTailMu.Lock()
	defer s.liveTailMu.Unlock()
	return slices.Clone(s.liveTails[liveOutputTailKey(workspaceID, commandID)])
}

func (s *Service) removeWorkspaceLiveTails(workspaceID string) {
	prefix := strings.TrimSpace(workspaceID) + "\x00"
	s.liveTailMu.Lock()
	defer s.liveTailMu.Unlock()
	kept := s.liveTailOrder[:0]
	for _, key := range s.liveTailOrder {
		if strings.HasPrefix(key, prefix) {
			delete(s.liveTails, key)
			delete(s.liveTailTerminals, key)
			continue
		}
		kept = append(kept, key)
	}
	s.liveTailOrder = kept
}

func (s *Service) removeTerminalLiveTails(workspaceID string, terminalID terminalpkg.ID) {
	prefix := strings.TrimSpace(workspaceID) + "\x00"
	wantedTerminal := strings.TrimSpace(string(terminalID))
	s.liveTailMu.Lock()
	defer s.liveTailMu.Unlock()
	kept := s.liveTailOrder[:0]
	for _, key := range s.liveTailOrder {
		if strings.HasPrefix(key, prefix) && s.liveTailTerminals[key] == wantedTerminal {
			delete(s.liveTails, key)
			delete(s.liveTailTerminals, key)
			continue
		}
		kept = append(kept, key)
	}
	s.liveTailOrder = kept
}

func (s *Service) clearLiveTails() {
	s.liveTailMu.Lock()
	defer s.liveTailMu.Unlock()
	clear(s.liveTails)
	clear(s.liveTailTerminals)
	s.liveTailOrder = nil
}

func liveOutputTailKey(workspaceID, commandID string) string {
	return strings.TrimSpace(workspaceID) + "\x00" + strings.TrimSpace(commandID)
}
