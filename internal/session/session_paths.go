package session

// SessionDir reports the on-disk session directory path.
func (s *Session) SessionDir() string {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionDir
}

// MetaPath reports the on-disk metadata file path.
func (s *Session) MetaPath() string {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metaPath
}

// DBPath reports the on-disk per-session event database path.
func (s *Session) DBPath() string {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbPath
}
