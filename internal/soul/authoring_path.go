package soul

import (
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/managedsidecar"
)

func validateManagedPath(workspaceRoot, targetPath, fileName string) *Diagnostic {
	_, diagnostic := resolveManagedPath(workspaceRoot, targetPath, fileName)
	return diagnostic
}

func resolveManagedPath(
	workspaceRoot string,
	targetPath string,
	fileName string,
) (managedsidecar.Path, *Diagnostic) {
	path, issue := managedsidecar.Resolve(workspaceRoot, targetPath)
	if issue == nil {
		issue = path.ValidateNoSymlinks()
	}
	if issue != nil {
		return managedsidecar.Path{}, managedPathDiagnostic(issue, fileName)
	}
	return path, nil
}

func managedPathDiagnostic(issue *managedsidecar.PathIssue, fileName string) *Diagnostic {
	sourcePath := firstNonEmpty(issue.SourcePath, fileName)
	message := fileName + " managed path is invalid"
	switch issue.Kind {
	case managedsidecar.IssueInvalidNUL:
		message = fileName + " path contains an invalid NUL byte"
	case managedsidecar.IssueResolveRoot:
		message = diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root: %v", issue.Err), 300)
	case managedsidecar.IssueResolveTarget:
		message = diagnostics.RedactAndBound(fmt.Sprintf("resolve %s path: %v", fileName, issue.Err), 300)
	case managedsidecar.IssueOutsideRoot:
		message = fileName + " path must stay inside the workspace root"
	case managedsidecar.IssueInspect:
		message = diagnostics.RedactAndBound(fmt.Sprintf("inspect %s path: %v", fileName, issue.Err), 300)
	case managedsidecar.IssueSymlink:
		message = fileName + " managed path must not contain symlinks"
	case managedsidecar.IssueSymlinkTargetOutside:
		message = fileName + " symlink target must stay inside the workspace root"
	}
	return &Diagnostic{
		Code:       authoringPathEscapeKey,
		Message:    message,
		SourcePath: sourcePath,
	}
}

func validAgentNameInput(name string) bool {
	return managedsidecar.ValidAgentName(name)
}
