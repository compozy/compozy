package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	appschema "github.com/compozy/compozy/desktop/schema"
	compozyconfig "github.com/compozy/compozy/internal/config"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const appStateSchemaVersion = 1

// AppStatusReport is the stable machine-readable desktop app status projection.
type AppStatusReport struct {
	SchemaVersion int             `json:"schema_version"`
	Installed     bool            `json:"installed"`
	AppVersion    string          `json:"app_version,omitempty"`
	Channel       string          `json:"channel,omitempty"`
	Running       bool            `json:"running"`
	PID           int             `json:"pid,omitempty"`
	State         string          `json:"state,omitempty"`
	Error         *AppErrorReport `json:"error,omitempty"`
	Runtime       AppRuntimeState `json:"runtime"`
	Update        AppUpdateState  `json:"update"`
}

// AppErrorReport is a typed, public-safe desktop diagnostic.
type AppErrorReport struct {
	Code        string `json:"code"`
	SafeMessage string `json:"safe_message"`
	LogPath     string `json:"log_path"`
}

// AppRuntimeState reports the last shell-observed runtime attachment state.
type AppRuntimeState struct {
	Attached bool   `json:"attached"`
	Owned    bool   `json:"owned"`
	Version  string `json:"version,omitempty"`
}

// AppUpdateState reports the last shell-observed app and runtime update state.
type AppUpdateState struct {
	AppState         string          `json:"app_state"`
	AppAvailable     string          `json:"app_available,omitempty"`
	RuntimeState     string          `json:"runtime_state"`
	RuntimeAvailable string          `json:"runtime_available,omitempty"`
	LastCheckedAt    string          `json:"last_checked_at,omitempty"`
	LastError        *AppErrorReport `json:"last_error,omitempty"`
}

type appStateRecord struct {
	SchemaVersion int             `json:"schema_version"`
	PID           int             `json:"pid"`
	StartedAt     time.Time       `json:"started_at"`
	AppVersion    string          `json:"app_version,omitempty"`
	Channel       string          `json:"channel,omitempty"`
	State         string          `json:"state"`
	Owned         bool            `json:"owned,omitempty"`
	Runtime       json.RawMessage `json:"runtime,omitempty"`
	Error         *AppErrorReport `json:"error,omitempty"`
	Update        *AppUpdateState `json:"update,omitempty"`
}

type appInstallation struct {
	Installed bool
	Version   string
}

var (
	compiledAppStateSchema     *jsonschema.Schema
	compiledAppStateSchemaErr  error
	compiledAppStateSchemaOnce sync.Once
)

func resolveAppStatus(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
) (AppStatusReport, error) {
	installation, err := deps.resolveAppInstallation(ctx, homePaths)
	if err != nil {
		return AppStatusReport{}, fmt.Errorf("app: resolve platform registration: %w", err)
	}
	report := AppStatusReport{
		SchemaVersion: appStateSchemaVersion,
		Installed:     installation.Installed,
		AppVersion:    installation.Version,
		Runtime:       AppRuntimeState{},
		Update: AppUpdateState{
			AppState:     appIdleState,
			RuntimeState: appIdleState,
		},
	}

	raw, err := os.ReadFile(filepath.Join(homePaths.HomeDir, "app.json"))
	if errors.Is(err, os.ErrNotExist) {
		return validateAppStatusReport(report)
	}
	if err != nil {
		return AppStatusReport{}, fmt.Errorf("app: read state record: %w", err)
	}
	var version struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &version); err != nil {
		return AppStatusReport{}, fmt.Errorf("app: decode state version: %w", err)
	}
	if version.SchemaVersion != appStateSchemaVersion {
		return AppStatusReport{}, newAppCommandError(
			appStateSchemaUnknownCode,
			fmt.Sprintf("unsupported app state schema version %d", version.SchemaVersion),
			nil,
		)
	}
	if err := validateAppStateJSON(raw); err != nil {
		return AppStatusReport{}, fmt.Errorf("app: validate state record: %w", err)
	}
	var record appStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return AppStatusReport{}, fmt.Errorf("app: decode state record: %w", err)
	}

	report.Channel = record.Channel
	report.PID = record.PID
	report.State = record.State
	report.Error = record.Error
	report.Runtime = AppRuntimeState{
		Attached: record.State == appProductState,
		Owned:    record.Owned,
	}
	if record.State == "skew" && len(record.Runtime) > 0 {
		if err := json.Unmarshal(record.Runtime, &report.Runtime.Version); err != nil {
			return AppStatusReport{}, fmt.Errorf("app: decode runtime version: %w", err)
		}
	}
	if record.Update != nil {
		report.Update = *record.Update
		if report.Update.AppState == "" {
			report.Update.AppState = appIdleState
		}
		if report.Update.RuntimeState == "" {
			report.Update.RuntimeState = appIdleState
		}
	}
	report.Running = record.PID > 0 && deps.processAlive(record.PID) &&
		deps.processMatchesStartTime(record.PID, record.StartedAt)
	if !report.Running {
		report.PID = 0
	}
	return validateAppStatusReport(report)
}

func validateAppStatusReport(report AppStatusReport) (AppStatusReport, error) {
	raw, err := json.Marshal(report)
	if err != nil {
		return AppStatusReport{}, fmt.Errorf("app: encode status report: %w", err)
	}
	if err := validateAppStateJSON(raw); err != nil {
		return AppStatusReport{}, fmt.Errorf("app: validate status report: %w", err)
	}
	return report, nil
}

func validateAppStateJSON(raw []byte) error {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	compiledAppStateSchemaOnce.Do(func() {
		var definition any
		if err := json.Unmarshal(appschema.Definition, &definition); err != nil {
			compiledAppStateSchemaErr = fmt.Errorf("decode canonical schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("app-state.schema.json", definition); err != nil {
			compiledAppStateSchemaErr = fmt.Errorf("register canonical schema: %w", err)
			return
		}
		compiledAppStateSchema, compiledAppStateSchemaErr = compiler.Compile("app-state.schema.json")
	})
	if compiledAppStateSchemaErr != nil {
		return compiledAppStateSchemaErr
	}
	return compiledAppStateSchema.Validate(value)
}
