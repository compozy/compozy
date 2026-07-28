package daemon

func (s *bootState) currentExtensionRuntime() extensionRuntime {
	if s == nil {
		return nil
	}
	s.extMu.RLock()
	defer s.extMu.RUnlock()
	return s.extensions
}

func (s *bootState) setExtensionRuntime(runtime extensionRuntime) {
	if s == nil {
		return
	}
	s.extMu.Lock()
	defer s.extMu.Unlock()
	s.extensions = runtime
}
