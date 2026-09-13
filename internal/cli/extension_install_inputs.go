package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/spf13/cobra"
)

type extensionInputFlags struct {
	runtimeName string
	values      []string
	file        string
	digest      string
}

func (f *extensionInputFlags) register(cmd *cobra.Command) {
	cmd.Flags().
		StringVar(&f.runtimeName, "runtime-name", "", "Request a runtime name for a single-server MCP extension")
	cmd.Flags().StringArrayVar(&f.values, "input", nil, "Set a manifest input as id=value (repeatable)")
	cmd.Flags().
		StringVar(&f.file, "input-file", "", "Read input values from a JSON object of {value} or {vault_ref} envelopes")
	cmd.Flags().StringVar(&f.digest, "expected-digest", "", "Pin the SHA-256 digest of the reviewed extension")
}

func prepareExtensionCLIInputs(
	ctx context.Context, deps commandDeps, plan extensionInstallPlan, flags extensionInputFlags,
) (extensionInstallPlan, *ExtensionInstallPreviewRecord, error) {
	if err := extensionpkg.ValidateExpectedDigest(flags.digest); err != nil {
		return extensionInstallPlan{}, nil, err
	}
	if name := strings.TrimSpace(flags.runtimeName); name != "" {
		if err := compozyconfig.ValidateMCPServerName(name); err != nil {
			return extensionInstallPlan{}, nil, err
		}
	}
	values, err := readExtensionInputFile(flags.file)
	if err != nil {
		return extensionInstallPlan{}, nil, err
	}
	for index, request := range plan.Attempts {
		request.RuntimeName = strings.TrimSpace(flags.runtimeName)
		request.ExpectedDigest = strings.ToLower(strings.TrimSpace(flags.digest))
		preview, types, err := extensionCLIInputMetadata(ctx, deps, request, len(flags.values) > 0)
		if err != nil {
			if index < len(plan.Attempts)-1 && extensionInstallFallbackAllowed(err) {
				continue
			}
			return extensionInstallPlan{}, nil, err
		}
		if preview != nil && request.ExpectedDigest == "" {
			request.ExpectedDigest = preview.DigestSHA256
		}
		if request.Source == contract.InstallExtensionSourceCurated && request.ExpectedDigest == "" {
			return extensionInstallPlan{}, nil, errors.New("cli: current extension listing has no acquisition digest")
		}
		request.Inputs, err = mergeExtensionInputFlags(values, flags.values, types)
		if err != nil {
			return extensionInstallPlan{}, nil, err
		}
		return extensionInstallPlan{Attempts: []InstallExtensionRequest{request}}, preview, nil
	}
	return extensionInstallPlan{}, nil, errors.New("cli: extension install plan has no attempts")
}

func extensionCLIInputMetadata(
	ctx context.Context, deps commandDeps, request InstallExtensionRequest, needsTypes bool,
) (*ExtensionInstallPreviewRecord, map[string]string, error) {
	if request.Source == contract.InstallExtensionSourceLocalPath {
		prepared, err := prepareExtensionInstall(request.Ref)
		if err != nil {
			return nil, nil, err
		}
		if err := extensionpkg.CheckExpectedDigest(request.ExpectedDigest, prepared.Checksum); err != nil {
			return nil, nil, err
		}
		types := make(map[string]string, len(prepared.Manifest.Inputs))
		for _, input := range prepared.Manifest.Inputs {
			types[input.ID] = input.Type
		}
		return &ExtensionInstallPreviewRecord{Name: prepared.Manifest.Name, DigestSHA256: prepared.Checksum}, types, nil
	}
	if request.Source != contract.InstallExtensionSourceCurated && !needsTypes {
		return nil, nil, nil
	}
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, nil, err
	}
	preview, err := client.PreviewExtensionInstall(ctx, request)
	if err != nil {
		return nil, nil, err
	}
	types := make(map[string]string, len(preview.Inputs))
	for _, input := range preview.Inputs {
		types[input.ID] = input.Type
	}
	return &preview, types, nil
}

func mergeExtensionInputFlags(
	values map[string]extensioninput.Value, flags []string, types map[string]string,
) (map[string]extensioninput.Value, error) {
	for _, flag := range flags {
		id, value, found := strings.Cut(flag, "=")
		id = strings.TrimSpace(id)
		if !found || id == "" {
			return nil, errors.New("cli: --input requires id=value")
		}
		inputType, known := types[id]
		if !known {
			return nil, &extensionpkg.InputValidationError{InputID: id, Reason: "unknown input id"}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("cli: encode input %q", id)
		}
		if inputType == "boolean" {
			raw = json.RawMessage(value)
		}
		normalized, err := extensionpkg.NormalizeInputJSON(inputType, raw)
		if err != nil {
			return nil, &extensionpkg.InputValidationError{
				InputID: id,
				Reason:  "invalid value for the declared type (maximum 8 KB)",
			}
		}
		if values == nil {
			values = make(map[string]extensioninput.Value)
		}
		values[id] = extensioninput.Value{Value: normalized}
	}
	return values, nil
}

func readExtensionInputFile(path string) (map[string]extensioninput.Value, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cli: open input file: %w", err)
	}
	defer file.Close()
	const maxInputFileBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(file, maxInputFileBytes+1))
	if err != nil || len(data) > maxInputFileBytes {
		return nil, errors.New("cli: input file could not be read within the 1 MB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var values map[string]extensioninput.Value
	if decoder.Decode(&values) != nil || values == nil {
		return nil, errors.New("cli: input file must be a JSON object of {value} or {vault_ref} envelopes")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("cli: input file must contain exactly one JSON object")
	}
	for id, value := range values {
		if strings.TrimSpace(id) == "" || (len(value.Value) > 0) == (value.VaultRef != nil) {
			return nil, &extensionpkg.InputValidationError{InputID: id, Reason: "supply exactly one value or vault_ref"}
		}
	}
	return values, nil
}
