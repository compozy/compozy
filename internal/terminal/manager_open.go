package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const processCleanupTimeout = 5 * time.Second

func (m *Service) Open(ctx context.Context, request OpenRequest) (Handle, error) {
	if ctx == nil {
		return nil, errors.New("terminal: open context is required")
	}
	cwd, workspaceID, err := m.resolveOpenWorkspace(ctx, request.WS, request.Cwd, request.Actor.ProfileID)
	if err != nil {
		return nil, err
	}
	request.WS, request.Cwd = workspaceID, cwd
	producer, err := m.beginWorkspaceProducer(workspaceID)
	if err != nil {
		return nil, err
	}
	defer producer.Release()
	return m.open(ctx, request, cwd, workspaceID)
}

func (m *Service) open(ctx context.Context, request OpenRequest, cwd, workspaceID string) (Handle, error) {
	if err := m.admit(ctx, workspaceID, request.Actor); err != nil {
		return nil, err
	}
	request.Title = SanitizeTitle(request.Title)
	var err error
	request.Capabilities, err = m.Capabilities(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if !request.Capabilities.Interactive {
		return nil, &Error{
			Code: ErrorCodeInteractiveUnavailable, Message: "interactive terminals are unavailable in this workspace",
			Platform: runtime.GOOS, Err: ErrInteractive,
		}
	}
	settings, err := m.settings(ctx, workspaceID, request.Actor.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("terminal: resolve settings: %w", err)
	}
	if err := validateSettings(settings); err != nil {
		return nil, err
	}
	request.WS = workspaceID
	releaseAdmission, err := m.reserveAdmission(ctx, request, settings)
	if err != nil {
		return nil, err
	}
	defer releaseAdmission()
	shell, err := resolveShell(request.Shell, settings.DefaultShell)
	if err != nil {
		return nil, err
	}
	id, err := newTerminalID(m.entropy)
	if err != nil {
		return nil, err
	}
	nonce, err := newMarkerNonce(m.entropy)
	if err != nil {
		return nil, err
	}
	cols, rows := normalizedDimensions(request.Cols, request.Rows)
	spec := ProcSpec{
		Argv: []string{shell}, Cwd: cwd, Cols: cols, Rows: rows,
		Mode: terminalpty.ModePTY, MarkerNonce: nonce,
		ShellIntegration: settings.ShellIntegration,
	}
	info := boundInfo(Info{
		ID: id, WS: workspaceID, ProfileID: request.Actor.ProfileID,
		Title: request.Title, Shell: shell, Cwd: cwd, Mode: ModePTY, State: terminalStateRunning,
		Capabilities: request.Capabilities, CreatedAt: m.now(),
	}, request.Actor)
	titlePinned := strings.TrimSpace(request.Title) != ""
	item, key, err := m.launchTerminal(ctx, terminalLaunch{
		spec: spec, info: info, origin: request.Actor, settings: settings, nonce: nonce, titlePinned: titlePinned,
		startLabel: fmt.Sprintf("shell %q", shell),
	})
	if err != nil {
		return nil, err
	}
	if err := m.startOpenedSession(ctx, item, request.Actor, settings.Recording); err != nil {
		m.removeInserted(key, item)
		item.cancel()
		return nil, cleanupRegisteredProcess(ctx, item.proc, item.processRecord, err)
	}
	return item, nil
}

func (m *Service) startOpenedSession(ctx context.Context, item *session, actor Actor, record bool) error {
	var autoRecording RecordingRef
	if record {
		var err error
		autoRecording, err = item.beginRecording(ctx)
		if err != nil {
			return fmt.Errorf("terminal: start automatic recording: %w", err)
		}
	}
	m.registerJournalTerminal(item)
	opened := item.Info()
	m.events.Notify(ctx, Event{
		Kind: EventKindOpened, WorkspaceID: opened.WS, ProfileID: opened.ProfileID,
		ProfileName: item.profileName, TerminalID: opened.ID, Actor: actor, Info: &opened,
		Detail: &EventDetail{Mode: opened.Mode, Cwd: opened.Cwd, Title: opened.Title}, At: m.now(),
	})
	if record {
		autoActor := Actor{Kind: ActorKindSystem, ID: "terminal-auto-recording", ProfileID: opened.ProfileID}
		item.emitRecordingEvent(ctx, EventKindRecordingStarted, autoActor, autoRecording, "", false)
	}
	item.start()
	return nil
}

func boundInfo(info Info, actor Actor) Info {
	if actor.Kind != ActorKindAgent {
		return info
	}
	info.BoundRun = &RunRef{
		SessionID: actor.SessionID, RunID: actor.RunID, Generation: actor.Generation,
	}
	return info
}

func (m *Service) eventProfileName(ctx context.Context, profileID string) string {
	if m.profileNames == nil {
		return ""
	}
	name, err := m.profileNames.ProfileName(ctx, profileID)
	if err != nil {
		m.logger.Warn("terminal: resolve event profile name", "profile_id", profileID, "error", err)
		return ""
	}
	return name
}

func (m *Service) reserveAdmission(ctx context.Context, request OpenRequest, settings Settings) (func(), error) {
	if err := requestContextError(ctx, "reserve admission"); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return nil, fmt.Errorf("%s: %w", errorMessageShuttingDown, ErrShuttingDown)
	}
	if _, sealed := m.sealedWorkspaces[request.WS]; sealed {
		m.mu.Unlock()
		return nil, fmt.Errorf("terminal workspace is sealed: %w", ErrServiceUnavailable)
	}
	workspaceCount := 0
	daemonCount := 0
	workspaceIDs := make([]string, 0)
	daemonIDs := make([]string, 0)
	for key, item := range m.terminals {
		if item.exited() {
			continue
		}
		daemonCount++
		daemonIDs = append(daemonIDs, string(key.id))
		if key.workspaceID == request.WS && key.profileID == request.Actor.ProfileID {
			workspaceCount++
			workspaceIDs = append(workspaceIDs, string(key.id))
		}
	}
	slices.Sort(workspaceIDs)
	slices.Sort(daemonIDs)
	scope := terminalScope{workspaceID: request.WS, profileID: request.Actor.ProfileID}
	workspaceCount += m.pendingByScope[scope]
	daemonCount += m.pendingDaemon
	if workspaceCount >= settings.MaxPerWorkspace {
		m.mu.Unlock()
		m.emitLimitRejected(ctx, request, "workspace", workspaceCount, settings.MaxPerWorkspace)
		return nil, limitError(workspaceCount, settings.MaxPerWorkspace, workspaceIDs)
	}
	if daemonCount >= settings.MaxPerDaemon {
		m.mu.Unlock()
		m.emitLimitRejected(ctx, request, "daemon", daemonCount, settings.MaxPerDaemon)
		return nil, limitError(daemonCount, settings.MaxPerDaemon, daemonIDs)
	}
	m.pendingByScope[scope]++
	m.pendingDaemon++
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			m.pendingByScope[scope]--
			if m.pendingByScope[scope] == 0 {
				delete(m.pendingByScope, scope)
			}
			m.pendingDaemon--
			m.mu.Unlock()
		})
	}, nil
}

func (m *Service) insert(key terminalKey, item *session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return fmt.Errorf("%s: %w", errorMessageShuttingDown, ErrShuttingDown)
	}
	if _, sealed := m.sealedWorkspaces[key.workspaceID]; sealed {
		return fmt.Errorf("terminal workspace is sealed: %w", ErrServiceUnavailable)
	}
	if _, exists := m.terminals[key]; exists {
		return errors.New("terminal: generated duplicate id")
	}
	m.terminals[key] = item
	return nil
}

func (m *Service) removeInserted(key terminalKey, expected *session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.terminals[key] == expected {
		delete(m.terminals, key)
	}
}

func (m *Service) emitLimitRejected(
	ctx context.Context,
	request OpenRequest,
	limit string,
	current, maximum int,
) {
	m.events.Notify(ctx, Event{
		Kind: EventKindLimitRejected, WorkspaceID: request.WS, ProfileID: request.Actor.ProfileID,
		Actor: request.Actor, Detail: &EventDetail{Limit: limit, Current: current, Max: maximum}, At: m.now(),
	})
}

func limitError(current, maximum int, ids []string) error {
	return &Error{
		Code: ErrorCodeLimitReached,
		Message: fmt.Sprintf(
			"terminal limit reached (%d/%d); existing terminals: %s",
			current,
			maximum,
			strings.Join(ids, ", "),
		),
		Current: current,
		Max:     maximum,
		Err:     ErrLimitReached,
	}
}

func (m *Service) resolveOpenWorkspace(
	ctx context.Context,
	workspaceID string,
	cwd string,
	profileID string,
	additionalRoots ...string,
) (string, string, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return "", "", &Error{
			Code: ErrorCodeRequiresWorkspace, Message: "terminal operations require a workspace",
			Err: ErrRequiresWorkspace,
		}
	}
	if m.workspaces == nil {
		if strings.TrimSpace(cwd) == "" {
			cwd = "."
		}
		resolved, err := filepath.Abs(cwd)
		return resolved, workspaceID, err
	}
	resolved, err := m.resolveWorkspace(ctx, workspaceID, profileID)
	if err != nil {
		return "", "", fmt.Errorf(
			"terminal: resolve workspace %q: %w",
			workspaceID,
			err,
		)
	}
	canonicalID := strings.TrimSpace(resolved.ID)
	if canonicalID == "" {
		return "", "", errors.New(
			"terminal: resolved workspace registration is required",
		)
	}
	resolved.AdditionalDirs = append(resolved.AdditionalDirs, additionalRoots...)
	validCwd, err := resolveWorkspaceCwd(&resolved, cwd)
	if err != nil {
		return "", "", err
	}
	return validCwd, canonicalID, nil
}

func (m *Service) resolveWorkspace(
	ctx context.Context,
	workspaceID string,
	profileID string,
) (workspacepkg.ResolvedWorkspace, error) {
	profileResolver, supportsProfiles := m.workspaces.(ProfileWorkspaceResolver)
	if !supportsProfiles || m.profileNames == nil {
		return m.workspaces.Resolve(ctx, workspaceID)
	}
	profileName, err := m.profileNames.ProfileName(ctx, profileID)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf("terminal: resolve profile name: %w", err)
	}
	if profileName == "" || profileName == "default" {
		return m.workspaces.Resolve(ctx, workspaceID)
	}
	return profileResolver.ResolveForProfile(ctx, workspaceID, profileName)
}

func resolveWorkspaceCwd(workspace *workspacepkg.ResolvedWorkspace, requested string) (string, error) {
	if workspace == nil {
		return "", errors.New("terminal: resolved workspace is unavailable")
	}
	root := filepath.Clean(workspace.RootDir)
	displayPath := requested
	if requested == "" {
		requested = root
		displayPath = root
	} else if !filepath.IsAbs(requested) {
		requested = filepath.Join(root, requested)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(requested))
	if err != nil {
		return "", &Error{
			Code:    ErrorCodeInvalidCwd,
			Message: fmt.Sprintf("invalid terminal cwd %q: %v", displayPath, err),
			Path:    displayPath,
			Err:     ErrInvalidCwd,
		}
	}
	allowed := append([]string{root}, workspace.AdditionalDirs...)
	for _, candidate := range allowed {
		candidateResolved, resolveErr := filepath.EvalSymlinks(filepath.Clean(candidate))
		if resolveErr != nil {
			continue
		}
		if pathWithin(candidateResolved, resolved) {
			info, statErr := os.Stat(resolved)
			if statErr == nil && info.IsDir() {
				return resolved, nil
			}
		}
	}
	return "", &Error{
		Code:    ErrorCodeInvalidCwd,
		Message: fmt.Sprintf("invalid terminal cwd %q: outside workspace", displayPath),
		Path:    displayPath,
		Err:     ErrInvalidCwd,
	}
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolveShell(requested, configured string) (string, error) {
	candidates := append([]string{requested, configured}, platformShellCandidates()...)
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		resolved, err := exec.LookPath(candidate)
		if err == nil {
			return resolved, nil
		}
	}
	return "", &Error{
		Code:     ErrorCodeInteractiveUnavailable,
		Message:  "no terminal shell is available",
		Platform: runtime.GOOS,
		Err:      errors.Join(ErrInteractive, exec.ErrNotFound),
	}
}

func normalizedDimensions(cols, rows uint16) (uint16, uint16) {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	cols, rows, _ = terminalwire.ClampDimensions(cols, rows)
	return cols, rows
}

func cleanupUnregisteredProcess(ctx context.Context, proc Proc) error {
	if proc == nil {
		return nil
	}
	cleanupCtx, cancel := boundedCleanupContext(ctx, processCleanupTimeout)
	defer cancel()
	return cleanupProcess(cleanupCtx, proc)
}

func cleanupProcess(ctx context.Context, proc Proc) error {
	killErr := proc.Kill(terminalpty.SignalKILL)
	closeErr := proc.Close()
	_, waitErr := proc.Wait(ctx)
	return errors.Join(killErr, closeErr, waitErr)
}

func cleanupRegisteredProcess(
	ctx context.Context,
	proc Proc,
	record processCheckpoint,
	cause error,
) error {
	cleanupCtx, cancelCleanup := boundedCleanupContext(ctx, processCleanupTimeout)
	cleanupErr := cleanupProcess(cleanupCtx, proc)
	cancelCleanup()
	var completeErr error
	if record != nil {
		completeCtx, cancelComplete := boundedCleanupContext(ctx, processCleanupTimeout)
		completeErr = record.Complete(completeCtx, toolruntime.ProcessCompletion{
			Err: cause, Error: "terminal startup rollback",
		})
		cancelComplete()
	}
	return errors.Join(cause, cleanupErr, completeErr)
}

func boundedCleanupContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

func validateSettings(settings Settings) error {
	if settings.ScrollbackBytes <= 0 || settings.DetachedTTL <= 0 || settings.ExitRetention <= 0 ||
		settings.RecordingRetentionDays <= 0 ||
		settings.MaxPerWorkspace <= 0 || settings.MaxPerDaemon <= 0 || settings.MaxSubscribers <= 0 {
		return errors.New("terminal: settings must contain positive limits and retention durations")
	}
	return nil
}
