package version

import "fmt"

// UnstampedCommit is the placeholder commit value used when the binary is built
// without git commit information injected via -ldflags. It is the single source
// of truth for the "no commit recorded" sentinel.
const UnstampedCommit = "none"

var (
	Version                  = "dev"
	Commit                   = UnstampedCommit
	Date                     = "unknown"
	ExtensionProtocolVersion = "1"
)

func String() string {
	return fmt.Sprintf("%s (commit=%s date=%s)", Version, Commit, Date)
}
