package extensionpkg

type extensionLogWriter struct {
	ring           *ExtensionLogRing
	generationHash string
}

func (w extensionLogWriter) Write(p []byte) (int, error) {
	if w.ring != nil {
		w.ring.append(string(p), w.generationHash)
	}
	return len(p), nil
}
