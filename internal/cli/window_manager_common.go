package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
)

const (
	windowManagerWorkspaceFlag    = "workspace"
	windowManagerAppFlag          = "app"
	windowManagerRevisionFlag     = "revision"
	windowManagerClientFlag       = "client"
	windowManagerOriginFlag       = "origin"
	windowManagerActorIDFlag      = "actor-id"
	windowManagerRectFlag         = "rect"
	windowManagerPayloadFlag      = "payload"
	windowManagerPayloadFile      = "payload-file"
	windowManagerDocumentFile     = "file"
	windowManagerCLIOrigin        = "cli"
	windowManagerCreateKey        = "create"
	windowManagerDeleteKey        = "delete"
	windowManagerListKey          = "list"
	windowManagerUpdateKey        = "update"
	windowManagerSwitchKey        = "switch"
	windowManagerPreviewKey       = "preview"
	windowManagerOpenKey          = "open"
	windowManagerNavigateKey      = "navigate"
	windowManagerFocusKey         = "focus"
	windowManagerMoveKey          = "move"
	windowManagerArrangeKey       = "arrange"
	windowManagerArrangementFlag  = "arrangement"
	windowManagerFrameFlag        = "frame"
	windowManagerGroupFlag        = "group"
	windowManagerResourceFlag     = "resource"
	windowManagerDiagnostics      = "diagnostics"
	windowManagerNameField        = "name"
	windowManagerPurposeField     = "purpose"
	windowManagerDesktopID        = "desktop_id"
	windowManagerValidField       = "valid"
	windowManagerNameHeader       = "NAME"
	windowManagerDiagnosticsLabel = "Diagnostics"
	windowManagerPathnameFlag     = "pathname"
	windowManagerSearchJSONFlag   = "search-json"
)

type windowManagerMutationFlags struct {
	workspace string
	revision  uint64
	clientID  string
	origin    string
	actorID   string
}

func requiredWindowManagerRoute(
	cmd *cobra.Command,
	pathname string,
	searchJSON string,
) (windowmanager.RouteIntent, error) {
	pathname, err := requiredWindowManagerFlag(pathname, windowManagerPathnameFlag)
	if err != nil {
		return windowmanager.RouteIntent{}, err
	}
	if !cmd.Flags().Changed(windowManagerSearchJSONFlag) {
		return windowmanager.RouteIntent{}, errors.New("cli: --search-json is required")
	}
	var search windowmanager.RouteSearch
	if err := decodeStrictWindowManagerJSON([]byte(searchJSON), &search); err != nil {
		return windowmanager.RouteIntent{}, newWindowManagerCLIValidationError(
			windowManagerCLIValidationInvalidValue,
			windowManagerSearchJSONFlag,
			fmt.Errorf("cli: decode --search-json: %w", err),
		)
	}
	route, err := windowmanager.CanonicalRouteIntent(
		windowmanager.RouteIntent{Pathname: pathname, Search: search},
	)
	if err != nil {
		return windowmanager.RouteIntent{}, newWindowManagerCLIValidationError(
			windowManagerCLIValidationInvalidValue,
			windowManagerPathnameFlag,
			fmt.Errorf("cli: invalid window route: %w", err),
		)
	}
	return route, nil
}

func addWindowManagerRouteFlags(cmd *cobra.Command, pathname *string, searchJSON *string) {
	cmd.Flags().StringVar(pathname, windowManagerPathnameFlag, "", "Absolute application pathname")
	cmd.Flags().StringVar(searchJSON, windowManagerSearchJSONFlag, "", "Route search state as a JSON object")
}

func (flags *windowManagerMutationFlags) add(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flags.workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().Uint64Var(&flags.revision, windowManagerRevisionFlag, 0, "Expected workspace revision")
	cmd.Flags().StringVar(&flags.clientID, windowManagerClientFlag, "", "Connected presentation client ID")
	cmd.Flags().StringVar(&flags.origin, windowManagerOriginFlag, windowManagerCLIOrigin, "Command origin label")
	cmd.Flags().StringVar(&flags.actorID, windowManagerActorIDFlag, "", "Optional actor identifier")
}

func (flags windowManagerMutationFlags) request(
	cmd *cobra.Command,
	commandID contract.WindowManagerCommandID,
	payload any,
) (contract.WindowManagerCommandRequest, error) {
	workspace, err := requiredWindowManagerFlag(flags.workspace, windowManagerWorkspaceFlag)
	if err != nil {
		return contract.WindowManagerCommandRequest{}, err
	}
	revision, err := requiredWindowManagerRevision(cmd, flags.revision)
	if err != nil {
		return contract.WindowManagerCommandRequest{}, err
	}
	clientID, err := optionalWindowManagerID[windowmanager.ClientID](cmd, windowManagerClientFlag, flags.clientID)
	if err != nil {
		return contract.WindowManagerCommandRequest{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return contract.WindowManagerCommandRequest{}, fmt.Errorf("cli: encode %s payload: %w", commandID, err)
	}
	return contract.WindowManagerCommandRequest{
		WorkspaceID:      windowmanager.WorkspaceID(workspace),
		CommandID:        commandID,
		ExpectedRevision: revision,
		ClientID:         clientID,
		Actor: contract.WindowManagerActor{
			Kind: windowManagerCLIOrigin,
			ID:   strings.TrimSpace(flags.actorID),
		},
		Origin:  strings.TrimSpace(flags.origin),
		Payload: raw,
	}, nil
}

func executeWindowManagerCommand(
	cmd *cobra.Command,
	deps commandDeps,
	request contract.WindowManagerCommandRequest,
) (contract.WindowManagerResult, error) {
	client, err := windowManagerClientFromDeps(deps)
	if err != nil {
		return contract.WindowManagerResult{}, err
	}
	return client.ExecuteWindowManagerCommand(cmd.Context(), string(request.WorkspaceID), request)
}

func windowManagerClientFromDeps(deps commandDeps) (WindowManagerClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	windowManagerClient, ok := client.(WindowManagerClient)
	if !ok {
		return nil, errors.New("cli: window-manager client is unavailable")
	}
	return windowManagerClient, nil
}

func requiredWindowManagerFlag(value string, name string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", newWindowManagerCLIValidationError(
			windowManagerCLIValidationRequired,
			name,
			fmt.Errorf("cli: --%s is required", name),
		)
	}
	return trimmed, nil
}

func requiredWindowManagerRevision(
	cmd *cobra.Command,
	value uint64,
) (*contract.WindowManagerRevision, error) {
	if !cmd.Flags().Changed(windowManagerRevisionFlag) {
		return nil, newWindowManagerCLIValidationError(
			windowManagerCLIValidationRequired,
			windowManagerRevisionFlag,
			errors.New("cli: --revision is required"),
		)
	}
	if value > uint64(contract.WindowManagerMaxSafeRevision) {
		return nil, fmt.Errorf(
			"cli: --revision must not exceed %d",
			contract.WindowManagerMaxSafeRevision,
		)
	}
	revision := contract.WindowManagerRevision(value)
	return &revision, nil
}

func optionalWindowManagerRevision(
	cmd *cobra.Command,
	flagName string,
	value uint64,
) (*contract.WindowManagerRevision, error) {
	if !cmd.Flags().Changed(flagName) {
		return nil, nil
	}
	if value > uint64(contract.WindowManagerMaxSafeRevision) {
		return nil, fmt.Errorf("cli: --%s must not exceed %d", flagName, contract.WindowManagerMaxSafeRevision)
	}
	revision := contract.WindowManagerRevision(value)
	return &revision, nil
}

func optionalWindowManagerID[T ~string](
	cmd *cobra.Command,
	flagName string,
	value string,
) (*T, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if cmd.Flags().Changed(flagName) {
			return nil, newWindowManagerCLIValidationError(
				windowManagerCLIValidationInvalidValue,
				flagName,
				fmt.Errorf("cli: --%s must not be blank", flagName),
			)
		}
		return nil, nil
	}
	converted := T(trimmed)
	return &converted, nil
}

func requiredWindowManagerClient(flags windowManagerMutationFlags) error {
	if strings.TrimSpace(flags.clientID) == "" {
		return newWindowManagerCLIValidationError(
			windowManagerCLIValidationRequired,
			windowManagerClientFlag,
			errors.New("cli: --client is required for presentation commands"),
		)
	}
	return nil
}

func readWindowManagerDocument(cmd *cobra.Command, file string) (contract.WindowManagerLayoutDocument, error) {
	data, err := readWindowManagerJSONFile(cmd, file, "layout document")
	if err != nil {
		return contract.WindowManagerLayoutDocument{}, err
	}
	var document contract.WindowManagerLayoutDocument
	if err := decodeStrictWindowManagerJSON(data, &document); err != nil {
		return contract.WindowManagerLayoutDocument{}, fmt.Errorf("cli: decode layout document: %w", err)
	}
	return document, nil
}

func readWindowManagerPayload(cmd *cobra.Command, inline string, file string) (json.RawMessage, error) {
	inlineChanged := cmd.Flags().Changed(windowManagerPayloadFlag)
	fileChanged := cmd.Flags().Changed(windowManagerPayloadFile)
	if inlineChanged == fileChanged {
		return nil, errors.New("cli: exactly one of --payload or --payload-file is required")
	}
	data := []byte(inline)
	if fileChanged {
		var err error
		data, err = readWindowManagerJSONFile(cmd, file, "command payload")
		if err != nil {
			return nil, err
		}
	}
	var object map[string]json.RawMessage
	if err := decodeStrictWindowManagerJSON(data, &object); err != nil {
		return nil, fmt.Errorf("cli: decode command payload: %w", err)
	}
	if object == nil {
		return nil, errors.New("cli: command payload must be a JSON object")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return nil, fmt.Errorf("cli: compact command payload: %w", err)
	}
	return json.RawMessage(compact.Bytes()), nil
}

func readWindowManagerJSONFile(cmd *cobra.Command, file string, label string) ([]byte, error) {
	path := strings.TrimSpace(file)
	if path == "" {
		return nil, fmt.Errorf("cli: --file is required for %s", label)
	}
	if path == "-" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("cli: read %s from stdin: %w", label, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cli: read %s file: %w", label, err)
	}
	return data, nil
}

func decodeStrictWindowManagerJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func optionalWindowManagerRect(cmd *cobra.Command, raw string) (*windowmanager.NormalizedRect, error) {
	return optionalWindowManagerNormalizedRect(cmd, windowManagerRectFlag, raw)
}

func optionalWindowManagerNormalizedRect(
	cmd *cobra.Command,
	flagName string,
	raw string,
) (*windowmanager.NormalizedRect, error) {
	if !cmd.Flags().Changed(flagName) {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("cli: --%s must be x,y,width,height in normalized coordinates", flagName)
	}
	values := make([]float64, 4)
	for index, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("cli: --%s coordinate %q is invalid", flagName, part)
		}
		values[index] = value
	}
	rect := windowmanager.NormalizedRect{X: values[0], Y: values[1], Width: values[2], Height: values[3]}
	if rect.X < 0 || rect.Y < 0 || rect.Width <= 0 || rect.Height <= 0 ||
		rect.X+rect.Width > 1 || rect.Y+rect.Height > 1 {
		return nil, newWindowManagerCLIValidationError(
			windowManagerCLIValidationOutOfBounds,
			flagName,
			fmt.Errorf("cli: --%s must be positive and contained in the normalized unit square", flagName),
		)
	}
	return &rect, nil
}

func requiredWindowManagerIDs(values []string, flagName string) ([]windowmanager.WindowID, error) {
	result := make([]windowmanager.WindowID, 0, len(values))
	seen := make(map[windowmanager.WindowID]struct{}, len(values))
	for _, value := range values {
		id := windowmanager.WindowID(strings.TrimSpace(value))
		if id == "" {
			return nil, fmt.Errorf("cli: --%s must not contain a blank ID", flagName)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("cli: --%s contains duplicate ID %q", flagName, id)
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("cli: at least one --%s is required", flagName)
	}
	return result, nil
}

func isWindowManagerCommandID(value contract.WindowManagerCommandID) bool {
	return slices.Contains(contract.WindowManagerCommandIDValues(), string(value))
}
