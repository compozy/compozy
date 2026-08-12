package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	"github.com/spf13/cobra"
)

const (
	appDiagnoseMethod                = "diagnose"
	appExportDiagnosticsMethod       = "export_diagnostics"
	appDiagnosticConsentRequiredCode = "diagnostic_consent_required"
	appDiagnosticBundleOutputFlag    = "bundle-output"
	appDiagnosticSafeMessageMaxBytes = 512
)

type appDiagnoseOptions struct {
	bundle       bool
	yes          bool
	bundleOutput string
}

type appDiagnosticReport struct {
	SchemaVersion  int                         `json:"schema_version"`
	BootID         string                      `json:"boot_id"`
	BootPhase      string                      `json:"boot_phase"`
	AppVersion     string                      `json:"app_version"`
	RuntimeVersion *string                     `json:"runtime_version"`
	RuntimeOwned   *bool                       `json:"runtime_owned"`
	CurrentError   *appDiagnosticError         `json:"current_error"`
	PreviousCrash  *appDiagnosticPreviousCrash `json:"previous_crash"`
}

type appDiagnosticError struct {
	Code        string `json:"code"`
	SafeMessage string `json:"safe_message"`
}

type appDiagnosticPreviousCrash struct {
	BootID    string `json:"boot_id"`
	BootPhase string `json:"boot_phase"`
	StartedAt string `json:"started_at"`
}

type appDiagnosticBundleResult struct {
	Report appDiagnosticReport `json:"report"`
	Bundle appDiagnosticBundle `json:"bundle"`
}

func newAppDiagnoseCommand(deps commandDeps) *cobra.Command {
	var options appDiagnoseOptions
	command := &cobra.Command{
		Use:   "diagnose",
		Short: "Show safe desktop diagnostics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAppDiagnoseCommand(cmd, deps, options)
		},
	}
	command.Flags().BoolVar(&options.bundle, "bundle", false, "Create a local diagnostic bundle")
	command.Flags().BoolVar(&options.yes, yesFlagName, false, "Confirm diagnostic bundle creation")
	command.Flags().StringVar(
		&options.bundleOutput,
		appDiagnosticBundleOutputFlag,
		"",
		"File path for the diagnostic bundle",
	)
	return command
}

func runAppDiagnoseCommand(cmd *cobra.Command, deps commandDeps, options appDiagnoseOptions) error {
	if err := validateAppDiagnoseOptions(cmd, options); err != nil {
		return err
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return fmt.Errorf("app diagnose: resolve home: %w", err)
	}
	report, live, err := resolveAppDiagnosticReport(cmd.Context(), deps, homePaths)
	if err != nil {
		return err
	}
	if !options.bundle {
		return writeCommandOutput(cmd, appDiagnosticReportOutput(report))
	}

	outputPath, err := resolveAppDiagnosticBundlePath(homePaths, options.bundleOutput, deps.now())
	if err != nil {
		return err
	}
	bundle, err := createAppDiagnosticBundle(
		cmd.Context(),
		deps,
		homePaths,
		report,
		outputPath,
		live,
		strings.TrimSpace(options.bundleOutput) == "",
	)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, appDiagnosticBundleOutput(appDiagnosticBundleResult{
		Report: bundle.Manifest.Report,
		Bundle: bundle,
	}))
}

func validateAppDiagnoseOptions(cmd *cobra.Command, options appDiagnoseOptions) error {
	if !options.bundle {
		if options.yes {
			return errors.New("app diagnose: --yes requires --bundle")
		}
		if cmd.Flags().Changed(appDiagnosticBundleOutputFlag) {
			return errors.New("app diagnose: --bundle-output requires --bundle")
		}
		return nil
	}
	if !options.yes {
		return newAppCommandError(
			appDiagnosticConsentRequiredCode,
			"diagnostic bundle creation requires --yes",
			nil,
		)
	}
	if cmd.Flags().Changed(appDiagnosticBundleOutputFlag) && strings.TrimSpace(options.bundleOutput) == "" {
		return errors.New("app diagnose: --bundle-output must be a file path")
	}
	return nil
}

func resolveAppDiagnosticReport(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
) (appDiagnosticReport, bool, error) {
	result, err := deps.callAppControl(
		ctx,
		filepath.Join(homePaths.HomeDir, "app.sock"),
		appDiagnoseMethod,
		map[string]any{},
	)
	if err == nil {
		report, decodeErr := decodeAppDiagnosticReport(result)
		if decodeErr != nil {
			return appDiagnosticReport{}, false, decodeErr
		}
		return report, true, nil
	}
	if !isAppNotRunningError(err) {
		return appDiagnosticReport{}, false, err
	}
	report, recordErr := readLocalAppDiagnosticReport(homePaths)
	if recordErr != nil {
		return appDiagnosticReport{}, false, recordErr
	}
	return report, false, nil
}

func isAppNotRunningError(err error) bool {
	var appErr *appCommandError
	return errors.As(err, &appErr) && appErr.code == appNotRunningCode
}

func readLocalAppDiagnosticReport(homePaths compozyconfig.HomePaths) (appDiagnosticReport, error) {
	record, found, err := readAppStateRecord(homePaths)
	if err != nil {
		return appDiagnosticReport{}, err
	}
	if !found {
		return appDiagnosticReport{}, newAppCommandError(
			appNotRunningCode,
			"the CompozyOS desktop app is not running and has no saved diagnostic report",
			nil,
		)
	}
	report, err := validateAppDiagnosticReport(record.DiagnosticReport)
	if err != nil {
		return appDiagnosticReport{}, fmt.Errorf("app diagnose: validate saved diagnostic report: %w", err)
	}
	return report, nil
}

func decodeAppDiagnosticReport(value any) (appDiagnosticReport, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return appDiagnosticReport{}, fmt.Errorf("app diagnose: encode app control report: %w", err)
	}
	return validateAppDiagnosticReport(raw)
}

func validateAppDiagnosticReport(raw []byte) (appDiagnosticReport, error) {
	if err := validateAppStateJSON(raw); err != nil {
		return appDiagnosticReport{}, fmt.Errorf("app diagnose: validate report schema: %w", err)
	}
	var report appDiagnosticReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return appDiagnosticReport{}, fmt.Errorf("app diagnose: decode report: %w", err)
	}
	if report.CurrentError != nil {
		report.CurrentError.SafeMessage = diagnosticspkg.RedactAndBound(
			report.CurrentError.SafeMessage,
			appDiagnosticSafeMessageMaxBytes,
		)
		if report.CurrentError.SafeMessage == "" {
			report.CurrentError.SafeMessage = "A desktop error occurred."
		}
	}
	sanitized, err := json.Marshal(report)
	if err != nil {
		return appDiagnosticReport{}, fmt.Errorf("app diagnose: encode sanitized report: %w", err)
	}
	if err := validateAppStateJSON(sanitized); err != nil {
		return appDiagnosticReport{}, fmt.Errorf("app diagnose: validate sanitized report schema: %w", err)
	}
	return report, nil
}

func appDiagnosticReportOutput(report appDiagnosticReport) outputBundle {
	return outputBundle{
		jsonValue: report,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, report)
		},
		human: func() (string, error) {
			rows := []keyValue{
				{Label: "Boot", Value: report.BootID},
				{Label: networkStateValue, Value: report.BootPhase},
				{Label: versionValue, Value: report.AppVersion},
			}
			if report.RuntimeVersion != nil {
				rows = append(rows, keyValue{Label: "Runtime version", Value: *report.RuntimeVersion})
			}
			if report.RuntimeOwned != nil {
				rows = append(rows, keyValue{Label: "Runtime owned", Value: strconv.FormatBool(*report.RuntimeOwned)})
			}
			if report.CurrentError != nil {
				rows = append(rows,
					keyValue{Label: "Error code", Value: report.CurrentError.Code},
					keyValue{Label: automationErrorValue, Value: report.CurrentError.SafeMessage},
				)
			}
			return renderHumanSection("Desktop diagnostics", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"app_diagnostics",
				[]string{"boot_id", "boot_phase", appVersionKey},
				[]string{report.BootID, report.BootPhase, report.AppVersion},
			), nil
		},
	}
}

func appDiagnosticBundleOutput(result appDiagnosticBundleResult) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, result)
		},
		human: func() (string, error) {
			return renderHumanSection("Desktop diagnostic bundle", []keyValue{
				{Label: automationPathValue, Value: result.Bundle.Path},
				{Label: "Size", Value: strconv.FormatInt(result.Bundle.Bytes, 10)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"app_diagnostic_bundle",
				[]string{automationPathValue, supportSizeBytesKey},
				[]string{result.Bundle.Path, strconv.FormatInt(result.Bundle.Bytes, 10)},
			), nil
		},
	}
}
