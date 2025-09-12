package attachment

import (
	"fmt"
	"io"
	"os"
)

type resolvedFile struct {
	path string
	mime string
	temp bool
}

// IMPORTANT: For remote downloads (temp=true), callers MUST call Cleanup(),
// ideally via `defer res.Cleanup()`, to avoid leaking temporary files. Local
// files set temp=false and Cleanup() is a no-op.
func (r *resolvedFile) AsURL() (string, bool)        { return "", false }
func (r *resolvedFile) AsFilePath() (string, bool)   { return r.path, true }
func (r *resolvedFile) Open() (io.ReadCloser, error) { return os.Open(r.path) }
func (r *resolvedFile) MIME() string                 { return r.mime }
func (r *resolvedFile) Cleanup() {
	if r.temp {
		_ = os.Remove(r.path)
	}
}

type resolvedURL struct{ url string }

func (r *resolvedURL) AsURL() (string, bool)      { return r.url, true }
func (r *resolvedURL) AsFilePath() (string, bool) { return "", false }

// Open is not supported for URL-only handles; remote streaming is not
// implemented in phase 1. Use a resolver to download before opening.
func (r *resolvedURL) Open() (io.ReadCloser, error) {
	return nil, fmt.Errorf("cannot open URL directly")
}
func (r *resolvedURL) MIME() string { return "" }
func (r *resolvedURL) Cleanup()     {}

func chooseURL(u string, us []string) string {
	if u != "" {
		return u
	}
	if len(us) > 0 {
		return us[0]
	}
	return ""
}

func choosePath(p string, ps []string) string {
	if p != "" {
		return p
	}
	if len(ps) > 0 {
		return ps[0]
	}
	return ""
}

func errMimeDenied(t Type, m string) error {
	return fmt.Errorf("mime not allowed for %s: %s", t, m)
}

// WithResolvedCleanup runs fn and guarantees Cleanup afterward.
func WithResolvedCleanup(res Resolved, fn func() error) error {
	if res == nil {
		return nil
	}
	defer res.Cleanup()
	return fn()
}
