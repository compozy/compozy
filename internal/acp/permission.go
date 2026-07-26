package acp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	permissionRequestIDKey = "request_id"
)

var (
	// ErrPermissionDenied reports that the configured static policy rejected an operation.
	ErrPermissionDenied = errors.New("acp: permission denied")
	// ErrInvalidPath reports malformed tool paths rejected before filesystem access.
	ErrInvalidPath = errors.New("acp: invalid path")
	// ErrPathOutsideWorkspace reports that a requested path escapes the session root.
	ErrPathOutsideWorkspace = errors.New("acp: path outside session workspace")
	// ErrToolBlockedForNetworkTurn reports that daemon-side network turn policy rejected a tool operation.
	ErrToolBlockedForNetworkTurn = errors.New("acp: tool blocked for network-originated turn")
	// ErrPendingPermissionNotFound reports that no waiting permission request matched the approval request.
	ErrPendingPermissionNotFound = errors.New("acp: pending permission not found")
	// ErrPendingPermissionConflict reports that a fallback lookup by turn ID matched multiple pending requests.
	ErrPendingPermissionConflict = errors.New("acp: pending permission lookup is ambiguous")
	// ErrPermissionDecisionUnsupported reports a persisted decision that the provider did not offer.
	ErrPermissionDecisionUnsupported = errors.New("acp: permission decision unsupported")
)

type permissionPolicy struct {
	mode  aghconfig.PermissionMode
	root  string
	roots []string
}

// ApproveRequest resolves one pending permission request.
type ApproveRequest struct {
	RequestID string `json:"request_id,omitempty"`
	TurnID    string `json:"turn_id,omitempty"`
	Decision  string `json:"decision"`
}

// RequestPermissionRequest asks the session permission bridge for a tool-call decision.
type RequestPermissionRequest = acpsdk.RequestPermissionRequest

// RequestPermissionResponse reports the selected or canceled permission outcome.
type RequestPermissionResponse = acpsdk.RequestPermissionResponse

// Validate ensures the approval request can be matched to a pending permission.
func (r ApproveRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" && strings.TrimSpace(r.TurnID) == "" {
		return errors.New("acp: request_id or turn_id is required")
	}
	if _, err := parsePermissionDecision(r.Decision); err != nil {
		return err
	}
	return nil
}

type permissionEventRaw struct {
	RequestID string                  `json:"request_id"`
	Decision  string                  `json:"decision,omitempty"`
	ToolInput json.RawMessage         `json:"tool_input,omitempty"`
	Options   []permissionEventOption `json:"options,omitempty"`
	ToolCall  permissionToolCallRaw   `json:"tool_call"`
}

type permissionEventOption struct {
	Decision string `json:"decision,omitempty"`
	OptionID string `json:"option_id"`
	Kind     string `json:"kind"`
	Label    string `json:"label,omitempty"`
}

type permissionToolCallRaw struct {
	ID        string                    `json:"id"`
	Kind      string                    `json:"kind,omitempty"`
	Title     string                    `json:"title,omitempty"`
	Status    string                    `json:"status,omitempty"`
	Locations []acpsdk.ToolCallLocation `json:"locations,omitempty"`
}

func newPermissionPolicy(
	mode aghconfig.PermissionMode,
	root string,
	additionalRoots ...string,
) (permissionPolicy, error) {
	effectiveMode := mode
	if effectiveMode == "" {
		effectiveMode = aghconfig.PermissionModeApproveReads
	}
	if err := effectiveMode.Validate("permissions.mode"); err != nil {
		return permissionPolicy{}, err
	}

	primaryRoot, err := canonicalPermissionRoot(root, "permission root")
	if err != nil {
		return permissionPolicy{}, err
	}
	roots := []string{primaryRoot}
	seen := map[string]struct{}{primaryRoot: {}}
	for index, additionalRoot := range additionalRoots {
		if strings.TrimSpace(additionalRoot) == "" {
			continue
		}
		canonicalRoot, err := canonicalPermissionRoot(
			additionalRoot,
			fmt.Sprintf("additional permission root[%d]", index),
		)
		if err != nil {
			return permissionPolicy{}, err
		}
		if _, exists := seen[canonicalRoot]; exists {
			continue
		}
		seen[canonicalRoot] = struct{}{}
		roots = append(roots, canonicalRoot)
	}

	return permissionPolicy{
		mode:  effectiveMode,
		root:  primaryRoot,
		roots: roots,
	}, nil
}

func canonicalPermissionRoot(root string, label string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("acp: resolve %s: %w", label, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("acp: evaluate %s %q: %w", label, absRoot, err)
	}
	return filepath.Clean(resolvedRoot), nil
}

func (p permissionPolicy) authorize(op permissionOperation) error {
	if p.isAllowed(op) {
		return nil
	}
	return fmt.Errorf("%w: %s blocked by %s", ErrPermissionDenied, op, p.mode)
}

func (p permissionPolicy) isAllowed(op permissionOperation) bool {
	switch p.mode {
	case aghconfig.PermissionModeApproveAll:
		return true
	case aghconfig.PermissionModeApproveReads:
		return op == permissionReadTextFile
	case aghconfig.PermissionModeDenyAll:
		return false
	default:
		return false
	}
}

func (p permissionPolicy) permissionDecision(request acpsdk.RequestPermissionRequest) (permissionDecision, bool) {
	if _, err := p.resolvePathList(request.ToolCall.Locations); err != nil {
		return decisionRejectOnce, false
	}

	switch p.mode {
	case aghconfig.PermissionModeApproveAll:
		return decisionAllowOnce, false
	case aghconfig.PermissionModeApproveReads:
		if request.ToolCall.Kind != nil && *request.ToolCall.Kind == acpsdk.ToolKindRead {
			return decisionAllowOnce, false
		}
		return decisionPending, true
	case aghconfig.PermissionModeDenyAll:
		return decisionRejectOnce, false
	default:
		return decisionRejectOnce, false
	}
}

func (p permissionPolicy) resolvePath(requestPath string) (string, error) {
	target := strings.TrimSpace(requestPath)
	if target == "" {
		return "", fmt.Errorf("%w: request path is required", ErrInvalidPath)
	}
	if strings.ContainsRune(target, 0) {
		return "", fmt.Errorf("%w: request path contains NUL byte", ErrInvalidPath)
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(p.root, target)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("acp: resolve request path %q: %w", requestPath, err)
	}

	resolvedTarget, err := resolveExistingAwarePath(absTarget)
	if err != nil {
		return "", err
	}
	if !p.isWithinAllowedRoot(resolvedTarget) {
		return "", fmt.Errorf("%w: %s", ErrPathOutsideWorkspace, requestPath)
	}

	return resolvedTarget, nil
}

func (p permissionPolicy) isWithinAllowedRoot(target string) bool {
	roots := p.roots
	if len(roots) == 0 {
		roots = []string{p.root}
	}
	for _, root := range roots {
		if isWithinRoot(root, target) {
			return true
		}
	}
	return false
}

func (p permissionPolicy) resolvePathList(locations []acpsdk.ToolCallLocation) ([]string, error) {
	if len(locations) == 0 {
		return nil, nil
	}

	resolved := make([]string, 0, len(locations))
	for _, location := range locations {
		path, err := p.resolvePath(location.Path)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, path)
	}
	return resolved, nil
}

func resolveExistingAwarePath(target string) (string, error) {
	cleanTarget := filepath.Clean(target)
	if _, err := os.Stat(cleanTarget); err == nil {
		resolved, resolveErr := filepath.EvalSymlinks(cleanTarget)
		if resolveErr != nil {
			return "", fmt.Errorf("acp: evaluate request path %q: %w", cleanTarget, resolveErr)
		}
		return filepath.Clean(resolved), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("acp: stat request path %q: %w", cleanTarget, err)
	}

	parent := filepath.Dir(cleanTarget)
	existingParent, err := firstExistingAncestor(parent)
	if err != nil {
		return "", err
	}
	resolvedAncestor, err := filepath.EvalSymlinks(existingParent)
	if err != nil {
		return "", fmt.Errorf("acp: evaluate ancestor %q: %w", existingParent, err)
	}
	relativeParent, err := filepath.Rel(existingParent, parent)
	if err != nil {
		return "", fmt.Errorf("acp: resolve ancestor relationship for %q: %w", cleanTarget, err)
	}
	resolvedParent := filepath.Join(resolvedAncestor, relativeParent)
	return filepath.Join(resolvedParent, filepath.Base(cleanTarget)), nil
}

func firstExistingAncestor(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		if _, err := os.Stat(current); err == nil {
			return current, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("acp: stat ancestor %q: %w", current, err)
		}

		next := filepath.Dir(current)
		if next == current {
			return "", fmt.Errorf("acp: no existing ancestor for %q", path)
		}
		current = next
	}
}

func isWithinRoot(root, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)
	if cleanRoot == cleanTarget {
		return true
	}
	return strings.HasPrefix(cleanTarget, cleanRoot+string(os.PathSeparator))
}

func parsePermissionDecision(raw string) (permissionDecision, error) {
	switch permissionDecision(strings.TrimSpace(raw)) {
	case decisionAllowOnce, decisionAllowAlways, decisionRejectOnce, decisionRejectAlways:
		return permissionDecision(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("acp: invalid decision %q", raw)
	}
}
