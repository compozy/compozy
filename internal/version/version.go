// Package version provides build metadata injected via ldflags.
package version

import "sync"

// Values set at build time via -ldflags.
var (
	Version    = "dev"
	Commit     = "unknown"
	BuildDate  = "unknown"
	mu         sync.RWMutex
	overrideMu sync.Mutex
)

// Info describes the current build metadata.
type Info struct {
	Version   string
	Commit    string
	BuildDate string
}

// Current returns the active build metadata snapshot.
func Current() Info {
	mu.RLock()
	defer mu.RUnlock()

	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}
}

// OverrideVersionForTesting swaps the reported version until the returned
// restore function is called. Tests must call the restore function.
func OverrideVersionForTesting(current string) func() {
	overrideMu.Lock()
	mu.Lock()
	original := Version
	Version = current
	mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			mu.Lock()
			Version = original
			mu.Unlock()
			overrideMu.Unlock()
		})
	}
}

// String returns a readable single-line build summary.
func (i Info) String() string {
	return i.Version + " (" + i.Commit + ", " + i.BuildDate + ")"
}
