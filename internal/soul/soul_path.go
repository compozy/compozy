package soul

import (
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/managedsidecar"
)

func safeSourcePath(sourcePath string, workspaceRoot string) (string, *Diagnostic) {
	safePath, issue := managedsidecar.SafeSourcePath(sourcePath, workspaceRoot)
	if issue == nil {
		return safePath, nil
	}
	message := "SOUL.md path must stay inside the workspace root"
	switch issue.Kind {
	case managedsidecar.IssueInvalidNUL:
		safePath = FileName
		message = "SOUL.md path contains an invalid NUL byte"
	case managedsidecar.IssueResolveRoot:
		message = diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root: %v", issue.Err), 300)
	case managedsidecar.IssueResolveTarget:
		message = diagnostics.RedactAndBound(fmt.Sprintf("resolve SOUL.md path: %v", issue.Err), 300)
	case managedsidecar.IssueSymlinkTargetOutside:
		message = "SOUL.md symlink target must stay inside the workspace root"
	}
	return safePath, &Diagnostic{
		Code:       soulPathEscapeKey,
		Message:    message,
		SourcePath: firstNonEmpty(issue.SourcePath, safePath),
	}
}

func safePathWithoutRoot(path string) string {
	safe := managedsidecar.SafePathWithoutRoot(path)
	if safe == "" {
		return FileName
	}
	return safe
}

func soulPathForAgent(agentPath string) (string, error) {
	return managedsidecar.SidecarPath(agentPath, FileName)
}
