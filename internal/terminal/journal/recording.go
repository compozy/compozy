package journal

import (
	"context"
	"errors"
	"fmt"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

// PersistRecording writes a redacted asciicast artifact and records its durable metadata.
func (s *Service) PersistRecording(
	ctx context.Context,
	workspaceID string,
	terminalID terminalpkg.ID,
	ref terminalpkg.RecordingRef,
	contents []byte,
) (terminalpkg.RecordingRef, error) {
	s.artifactMu.Lock()
	defer s.artifactMu.Unlock()
	path, digest, bytes, created, err := s.writeRecordingFile(workspaceID, ref.ID, contents)
	if err != nil {
		return terminalpkg.RecordingRef{}, err
	}
	ref.Path = path
	ref.Digest = digest
	ref.Bytes = bytes
	if err := s.LinkRecording(ctx, workspaceID, terminalID, ref); err != nil {
		return terminalpkg.RecordingRef{}, errors.Join(
			fmt.Errorf("terminal journal: persist recording metadata: %w", err),
			removeCreatedFile(path, created),
		)
	}
	s.ReleaseRecordingID(workspaceID, ref.ID)
	return ref, nil
}
