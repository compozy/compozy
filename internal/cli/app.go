package cli

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

const appCommandKey = "app"

const (
	appIdleState    = "idle"
	appProductState = "product"
	appRetryMethod  = "retry"
)

func newAppCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{
		Use:   appCommandKey,
		Short: "Manage the CompozyOS desktop app",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	command.AddCommand(
		newAppOpenCommand(deps),
		newAppStatusCommand(deps),
		newAppUpdateCommand(deps),
		newAppControlCommand(deps, appRetryMethod, "Retry the current desktop app operation", appRetryMethod),
		newAppControlCommand(deps, "diagnose", "Show desktop app diagnostics", "diagnose"),
	)
	return command
}

func newAppStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationStatusKey,
		Short: "Show desktop app status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			homePaths, err := deps.resolveHome()
			if err != nil {
				return fmt.Errorf("app status: resolve home: %w", err)
			}
			report, err := resolveAppStatus(cmd.Context(), deps, homePaths)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, appStatusBundle(report))
		},
	}
}

func newAppOpenCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "open [path]",
		Short: "Launch or focus the desktop app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			schemeURL, productPath, err := appTargetURL(target)
			if err != nil {
				return err
			}
			homePaths, err := deps.resolveHome()
			if err != nil {
				return fmt.Errorf("app open: resolve home: %w", err)
			}
			installation, err := deps.resolveAppInstallation(cmd.Context(), homePaths)
			if err != nil {
				return fmt.Errorf("app open: resolve platform registration: %w", err)
			}
			if !installation.Installed {
				return newAppCommandError(
					appNotInstalledCode,
					"the CompozyOS desktop app is not installed",
					nil,
				)
			}
			running, err := appRecordRunning(homePaths, deps)
			if err != nil {
				return err
			}
			if running {
				result, callErr := deps.callAppControl(
					cmd.Context(),
					filepath.Join(homePaths.HomeDir, "app.sock"),
					"navigate",
					map[string]string{"path": productPath},
				)
				if callErr != nil {
					return callErr
				}
				return writeCommandOutput(cmd, appControlBundle("open", result))
			}
			if err := deps.openBrowser(cmd.Context(), schemeURL); err != nil {
				return newAppCommandError(
					appLaunchFailedCode,
					"the CompozyOS desktop app could not be launched",
					err,
				)
			}
			return writeCommandOutput(cmd, appControlBundle("open", map[string]any{
				"opened":            true,
				automationTargetKey: productPath,
			}))
		},
	}
}

func newAppUpdateCommand(deps commandDeps) *cobra.Command {
	var check bool
	var apply string
	command := &cobra.Command{
		Use:   updateUpdateKey,
		Short: "Check or apply desktop-managed updates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if check && strings.TrimSpace(apply) != "" {
				return errors.New("app update: --check and --apply are mutually exclusive")
			}
			method := "update.check"
			var params any
			if target := strings.TrimSpace(apply); target != "" {
				if target != appCommandKey && target != loopRuntimeKey {
					return errors.New("app update: --apply must be app or runtime")
				}
				method = "update.apply"
				params = map[string]string{automationTargetKey: target}
			}
			return runAppControlCommand(cmd, deps, method, params)
		},
	}
	command.Flags().BoolVar(&check, "check", false, "Check for app and runtime updates")
	command.Flags().StringVar(&apply, "apply", "", "Apply an update target: app or runtime")
	return command
}

func newAppControlCommand(deps commandDeps, use string, short string, method string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAppControlCommand(cmd, deps, method, nil)
		},
	}
}

func runAppControlCommand(cmd *cobra.Command, deps commandDeps, method string, params any) error {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return fmt.Errorf("app control: resolve home: %w", err)
	}
	installation, err := deps.resolveAppInstallation(cmd.Context(), homePaths)
	if err != nil {
		return fmt.Errorf("app control: resolve platform registration: %w", err)
	}
	if !installation.Installed {
		return newAppCommandError(appNotInstalledCode, "the CompozyOS desktop app is not installed", nil)
	}
	result, err := deps.callAppControl(
		cmd.Context(),
		filepath.Join(homePaths.HomeDir, "app.sock"),
		method,
		params,
	)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, appControlBundle(method, result))
}

func appTargetURL(raw string) (string, string, error) {
	if strings.TrimSpace(raw) == "" {
		return "compozyos://open/", "/", nil
	}
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "", "", newAppCommandError(invalidTargetPathCode, "the app target must be an absolute product path", nil)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Path == "" ||
		parsed.Fragment != "" || strings.Contains(parsed.Path, "\\") {
		return "", "", newAppCommandError(invalidTargetPathCode, "the app target is invalid", err)
	}
	for segment := range strings.SplitSeq(strings.TrimPrefix(parsed.EscapedPath(), "/"), "/") {
		decoded, decodeErr := url.PathUnescape(segment)
		if decodeErr != nil || decoded == "." || decoded == ".." ||
			strings.ContainsAny(decoded, "/\\") || strings.Contains(decoded, "://") {
			return "", "", newAppCommandError(invalidTargetPathCode, "the app target contains traversal", decodeErr)
		}
	}
	return "compozyos://open" + raw, raw, nil
}

func appRecordRunning(homePaths compozyconfig.HomePaths, deps commandDeps) (bool, error) {
	record, found, err := readAppStateRecord(homePaths)
	if err != nil || !found || record.PID <= 0 {
		return false, err
	}
	return deps.processAlive(record.PID) && deps.processMatchesStartTime(record.PID, record.StartedAt), nil
}

func appStatusBundle(report AppStatusReport) outputBundle {
	return outputBundle{
		jsonValue: report,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, report)
		},
		human: func() (string, error) {
			return renderHumanSection("CompozyOS desktop app", []keyValue{
				{Label: cliInstalledValue, Value: strconv.FormatBool(report.Installed)},
				{Label: versionValue, Value: stringOrDash(report.AppVersion)},
				{Label: "Running", Value: strconv.FormatBool(report.Running)},
				{Label: networkStateValue, Value: stringOrDash(report.State)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"app",
				[]string{marketplaceInstalledKey, "app_version", daemonRunningStatus, stateKey},
				[]string{
					strconv.FormatBool(report.Installed),
					report.AppVersion,
					strconv.FormatBool(report.Running),
					report.State,
				},
			), nil
		},
	}
}

func appControlBundle(action string, result any) outputBundle {
	payload := map[string]any{"action": action, "result": result}
	return outputBundle{
		jsonValue: payload,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, payload)
		},
		human: func() (string, error) {
			return "Desktop app action completed: " + action, nil
		},
		toon: func() (string, error) {
			return renderToonObject("app", []string{"action"}, []string{action}), nil
		},
	}
}
