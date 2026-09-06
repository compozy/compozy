package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/transcript"
)

// TranscriptSearch returns bounded literal matches from the session's projection.
func (m *Manager) TranscriptSearch(
	ctx context.Context,
	id string,
	query transcript.SearchQuery,
) (transcript.SearchResult, error) {
	var result transcript.SearchResult
	err := m.withTranscriptReader(ctx, strings.TrimSpace(id), func(reader transcript.Reader) error {
		navigation, ok := reader.(transcript.NavigationReader)
		if !ok {
			return fmt.Errorf("%w: navigation reads", errTranscriptProjectionUnavailable)
		}
		var readErr error
		result, readErr = navigation.TranscriptSearch(ctx, query)
		return readErr
	})
	if err != nil {
		return transcript.SearchResult{}, err
	}
	return result, nil
}

// TranscriptOutline returns every retained operator message and its reply preview.
func (m *Manager) TranscriptOutline(ctx context.Context, id string) (transcript.OutlineResult, error) {
	var result transcript.OutlineResult
	err := m.withTranscriptReader(ctx, strings.TrimSpace(id), func(reader transcript.Reader) error {
		navigation, ok := reader.(transcript.NavigationReader)
		if !ok {
			return fmt.Errorf("%w: navigation reads", errTranscriptProjectionUnavailable)
		}
		var readErr error
		result, readErr = navigation.TranscriptOutline(ctx)
		return readErr
	})
	if err != nil {
		return transcript.OutlineResult{}, err
	}
	return result, nil
}
