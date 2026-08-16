package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
)

type appStateSnapshot struct {
	SchemaVersion int       `json:"schema_version"`
	PID           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
	AppVersion    string    `json:"app_version"`
}

// CheckAll computes both installed tracks and the live operation from one release lookup.
func (m *Manager) CheckAll(ctx context.Context, opts CheckOptions) (MultiState, *Release, error) {
	runtimeState, release, checkErr := m.Check(ctx, opts)
	if checkErr != nil && strings.TrimSpace(runtimeState.Message) == "" &&
		strings.TrimSpace(runtimeState.LastError) == "" {
		return MultiState{}, nil, checkErr
	}
	app, err := m.readAppTrack(release, runtimeState.InstallMethod)
	if err != nil {
		return MultiState{}, release, err
	}
	archivedApp, err := m.operationStore.LatestArchivedApp(ctx)
	if err != nil {
		return MultiState{}, release, err
	}
	applyArchivedAppFailure(app, archivedApp, opts.ForceRefresh)
	operation, err := m.operationStore.Read(ctx)
	if err != nil {
		return MultiState{}, release, err
	}
	return ProjectMultiState(runtimeState, app, operation), release, checkErr
}

func applyArchivedAppFailure(app *AppTrackState, archived *Operation, forceRefresh bool) {
	if forceRefresh || app == nil || archived == nil || archived.App == nil ||
		archived.App.Phase != PhaseFailed || archived.App.ConsecutiveFailures < 1 {
		return
	}
	comparison, err := compareVersions(app.CurrentVersion, archived.App.ToVersion)
	if err == nil && comparison >= 0 {
		return
	}
	app.Status = StatusFailed
	app.LatestVersion = archived.App.ToVersion
	app.AttemptID = archived.App.AttemptID
	app.LastError = strings.TrimSpace(archived.LastError)
	if app.LastError == "" {
		app.LastError = "The previous app install did not complete."
	}
	app.Message = "Automatic app update checks are paused after the previous install failed. Retry manually."
}

// Snapshot returns the cached live projection, tolerating a recorded upstream refresh error.
func (m *Manager) Snapshot(ctx context.Context) (MultiState, error) {
	state, _, err := m.CheckAll(ctx, CheckOptions{AllowCachedOnFailure: true})
	if err != nil && strings.TrimSpace(state.Runtime.Message) == "" &&
		strings.TrimSpace(state.Runtime.LastError) == "" {
		return MultiState{}, err
	}
	return state, nil
}

// OperationStore exposes the single journal authority to composition roots.
func (m *Manager) OperationStore() *OperationStore { return m.operationStore }

func (m *Manager) readAppTrack(release *Release, installMethod string) (*AppTrackState, error) {
	appStatePath := firstPath(
		m.homePaths.AppStateFile,
		filepath.Join(m.homePaths.HomeDir, compozyconfig.AppStateFileName),
	)
	data, err := os.ReadFile(appStatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if installMethod != string(InstallMethodDesktopApp) {
				return nil, nil
			}
			return composeAppTrack(m.currentVersion, false, release), nil
		}
		return nil, fmt.Errorf("update: read desktop app state: %w", err)
	}
	var snapshot appStateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("update: decode desktop app state: %w", err)
	}
	if snapshot.SchemaVersion != 2 {
		return nil, fmt.Errorf("update: unsupported desktop app state schema version %d", snapshot.SchemaVersion)
	}
	running := snapshot.PID > 0 && procutil.Alive(snapshot.PID) &&
		procutil.MatchesStartTime(snapshot.PID, snapshot.StartedAt)
	return composeAppTrack(snapshot.AppVersion, running, release), nil
}

func composeAppTrack(currentVersion string, running bool, release *Release) *AppTrackState {
	state := &AppTrackState{
		Status: StatusUpToDate, Running: running, CurrentVersion: strings.TrimSpace(currentVersion),
		Message: "CompozyOS app is up to date.",
	}
	if release == nil {
		return state
	}
	state.LatestVersion = strings.TrimSpace(release.Version)
	state.ReleaseURL = strings.TrimSpace(release.ReleaseURL)
	comparison, err := compareVersions(state.CurrentVersion, state.LatestVersion)
	if err != nil {
		state.Status = StatusUnsupported
		state.LastError = err.Error()
		state.Message = "The installed app version cannot be compared with the release."
		return state
	}
	if comparison < 0 {
		state.Status = StatusAvailable
		state.Message = "CompozyOS app " + state.LatestVersion + " is available."
	}
	return state
}
