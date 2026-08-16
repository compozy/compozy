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
	compozyupdate "github.com/compozy/compozy/internal/update"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const appStateSchemaVersion = 2

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
	OperationID      string          `json:"operation_id,omitempty"`
	Phase            string          `json:"phase,omitempty"`
	Percent          *int            `json:"percent,omitempty"`
}

type appStateRecord struct {
	SchemaVersion    int             `json:"schema_version"`
	PID              int             `json:"pid"`
	StartedAt        time.Time       `json:"started_at"`
	AppVersion       string          `json:"app_version,omitempty"`
	Channel          string          `json:"channel,omitempty"`
	State            string          `json:"state"`
	Owned            bool            `json:"owned,omitempty"`
	Runtime          json.RawMessage `json:"runtime,omitempty"`
	Error            *AppErrorReport `json:"error,omitempty"`
	Update           *AppUpdateState `json:"update,omitempty"`
	DiagnosticReport json.RawMessage `json:"diagnostic_report"`
}

const appVersionKey = "app_version"

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

	record, found, err := readAppStateRecord(homePaths)
	if err != nil {
		return AppStatusReport{}, err
	}
	if !found {
		if err := overlayAppUpdateOperation(ctx, homePaths, &report); err != nil {
			return AppStatusReport{}, err
		}
		return validateAppStatusReport(report)
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
	if err := overlayAppUpdateOperation(ctx, homePaths, &report); err != nil {
		return AppStatusReport{}, err
	}
	return validateAppStatusReport(report)
}

func overlayAppUpdateOperation(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	report *AppStatusReport,
) error {
	store, err := compozyupdate.NewOperationStore(homePaths, nil)
	if err != nil {
		return err
	}
	operation, err := store.Read(ctx)
	if err != nil || operation == nil {
		return err
	}
	report.Update.OperationID = operation.ID
	percent := operation.Percent
	report.Update.Percent = &percent
	if phase, ok := compozyupdate.PhaseForUI(operation.ActiveTarget, activeOperationPhase(operation), operation.Percent); ok {
		report.Update.Phase = string(phase)
	}
	if operation.Runtime != nil {
		report.Update.RuntimeAvailable = operation.Runtime.ToVersion
		switch {
		case operation.Runtime.Phase == compozyupdate.PhaseFailed || operation.Runtime.Phase == compozyupdate.PhaseRolledBack:
			report.Update.RuntimeState = "failed"
		case operation.ActiveTarget == compozyupdate.TargetRuntime:
			if operation.Runtime.Phase == compozyupdate.PhasePending {
				report.Update.RuntimeState = "accepted"
			} else {
				report.Update.RuntimeState = "applying"
			}
		}
	}
	if operation.App != nil {
		report.Update.AppAvailable = operation.App.ToVersion
		switch {
		case operation.Waiting == compozyupdate.WaitingForApp || operation.App.Phase == compozyupdate.PhaseStaged:
			report.Update.AppState = "staged"
		case operation.App.Phase == compozyupdate.PhaseFailed:
			report.Update.AppState = "failed"
		case operation.ActiveTarget == compozyupdate.TargetApp:
			if operation.App.Phase == compozyupdate.PhasePending {
				report.Update.AppState = "accepted"
			} else {
				report.Update.AppState = "applying"
			}
		}
	}
	return nil
}

func activeOperationPhase(operation *compozyupdate.Operation) compozyupdate.OperationPhase {
	if operation == nil {
		return ""
	}
	if operation.ActiveTarget == compozyupdate.TargetRuntime && operation.Runtime != nil {
		return operation.Runtime.Phase
	}
	if operation.ActiveTarget == compozyupdate.TargetApp && operation.App != nil {
		return operation.App.Phase
	}
	return ""
}

func readAppStateRecord(homePaths compozyconfig.HomePaths) (appStateRecord, bool, error) {
	appStatePath := homePaths.AppStateFile
	if appStatePath == "" {
		appStatePath = filepath.Join(homePaths.HomeDir, compozyconfig.AppStateFileName)
	}
	raw, err := os.ReadFile(appStatePath)
	if errors.Is(err, os.ErrNotExist) {
		return appStateRecord{}, false, nil
	}
	if err != nil {
		return appStateRecord{}, false, fmt.Errorf("app: read state record: %w", err)
	}
	var version struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &version); err != nil {
		return appStateRecord{}, false, fmt.Errorf("app: decode state version: %w", err)
	}
	if version.SchemaVersion != appStateSchemaVersion {
		return appStateRecord{}, false, newAppCommandError(
			appStateSchemaUnknownCode,
			fmt.Sprintf("unsupported app state schema version %d", version.SchemaVersion),
			nil,
		)
	}
	if err := validateAppStateJSON(raw); err != nil {
		return appStateRecord{}, false, fmt.Errorf("app: validate state record: %w", err)
	}
	var record appStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return appStateRecord{}, false, fmt.Errorf("app: decode state record: %w", err)
	}
	return record, true, nil
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
