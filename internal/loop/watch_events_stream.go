package loop

import "strings"

// WatchEventsSessionStreamForSession returns the cursor stream key for one session.
func WatchEventsSessionStreamForSession(sessionID string) string {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return ""
	}
	return watchEventsSessionStream + watchEventsSessionSeparator + trimmed
}

// WatchEventsSessionIDFromStream extracts the session id from a dynamic session event stream key.
func WatchEventsSessionIDFromStream(stream string) (string, bool) {
	trimmed := strings.TrimSpace(stream)
	prefix := watchEventsSessionStream + watchEventsSessionSeparator
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	sessionID := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	return sessionID, sessionID != ""
}

// WatchEventsBaseStream returns the stable registry stream for a cursor stream key.
func WatchEventsBaseStream(stream string) string {
	if _, ok := WatchEventsSessionIDFromStream(stream); ok {
		return watchEventsSessionStream
	}
	return strings.TrimSpace(stream)
}
