package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
)

const (
	extensionWithDependenciesFlagName   = "with-dependencies"
	extensionAllowUnverifiedDepFlagName = "allow-unverified-dependency"
	extensionForceFlagName              = "force"
)

var errExtensionDependenciesDeclined = errors.New("cli: dependency install declined; nothing was installed")

// extensionDependencyFlags carries the experimental dependency consent of one install (D8). Gateway
// confirmations come from the repeatable --confirm-gateway-requirement flag: a bare digest confirms the
// requested extension and <name>=<digest> confirms one dependency.
type extensionDependencyFlags struct {
	withDependencies bool
	allowUnverified  []string
	confirmGateway   []string
}

func (f *extensionDependencyFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.withDependencies, extensionWithDependenciesFlagName, false,
		"Experimental. Install missing and enable disabled dependencies first, each through its own trust gate")
	cmd.Flags().StringArrayVar(&f.allowUnverified, extensionAllowUnverifiedDepFlagName, nil,
		"Experimental. Allow one unverified dependency by name (repeatable); --allow-unverified covers only "+
			"the requested extension")
	cmd.Flags().StringArrayVar(&f.confirmGateway, extensionConfirmGatewayFlagName, nil,
		"Confirm the candidate gateway requirement digest; use <name>=<digest> for a dependency (experimental, "+
			"repeatable)")
}

// apply sets the root gateway digest and per-dependency consent on every attempt of the plan.
func (f *extensionDependencyFlags) apply(cmd *cobra.Command, plan extensionInstallPlan, yes bool) error {
	root := ""
	consent := make(map[string]contract.ExtensionDependencyConsent)
	for _, raw := range f.confirmGateway {
		name, digest, scoped := strings.Cut(strings.TrimSpace(raw), "=")
		if !scoped {
			if root != "" {
				return fmt.Errorf("cli: --%s accepts one bare digest; use <name>=<digest> for a dependency",
					extensionConfirmGatewayFlagName)
			}
			root = name
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" || strings.TrimSpace(digest) == "" {
			return fmt.Errorf("cli: --%s dependency form is <name>=<digest>", extensionConfirmGatewayFlagName)
		}
		item := consent[name]
		item.ConfirmGatewayDigest = strings.TrimSpace(digest)
		consent[name] = item
	}
	for _, raw := range f.allowUnverified {
		name := strings.TrimSpace(raw)
		if name == "" {
			return fmt.Errorf("cli: --%s requires a dependency name", extensionAllowUnverifiedDepFlagName)
		}
		item := consent[name]
		item.AllowUnverified = true
		consent[name] = item
	}
	if len(f.allowUnverified) > 0 && !yes {
		mode, err := resolveOutputFormat(cmd)
		if err != nil {
			return err
		}
		if mode != OutputHuman {
			return fmt.Errorf("cli: --%s requires --yes for structured output", extensionAllowUnverifiedDepFlagName)
		}
	}
	for index := range plan.Attempts {
		plan.Attempts[index].ConfirmGatewayDigest = root
		plan.Attempts[index].WithDependencies = f.withDependencies
		if len(consent) > 0 {
			plan.Attempts[index].Dependencies = maps.Clone(consent)
		}
	}
	return nil
}

// pinExtensionDependencyDigests pins each acquired dependency to the digest the preview reviewed, like the
// requested extension's own expected_digest. Explicit consent stays untouched otherwise.
func pinExtensionDependencyDigests(
	ctx context.Context, deps commandDeps, plan extensionInstallPlan, preview *ExtensionInstallPreviewRecord,
) error {
	if preview == nil {
		client, err := requireExtensionDaemonClient(ctx, deps)
		if err != nil {
			return err
		}
		reviewed, err := runExtensionInstallAttempts(
			plan.Attempts,
			func(request InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				return client.PreviewExtensionInstall(ctx, request)
			},
		)
		if err != nil {
			return err
		}
		preview = &reviewed
	}
	dependencies := preview.Dependencies
	for index := range plan.Attempts {
		request := &plan.Attempts[index]
		for _, dependency := range dependencies {
			if dependency.DigestSHA256 == "" {
				continue
			}
			if request.Dependencies == nil {
				request.Dependencies = make(map[string]contract.ExtensionDependencyConsent)
			}
			item := request.Dependencies[dependency.Name]
			item.ExpectedDigest = cmp.Or(item.ExpectedDigest, dependency.DigestSHA256)
			request.Dependencies[dependency.Name] = item
		}
	}
	return nil
}

// promptExtensionInstallDependencies answers extension_dependencies_required on an interactive terminal: one
// consolidated preview, one consent, then each artifact's own trust gate in order. It reports false when the
// refusal is not prompted (no TTY, a version_unmet node, or another error), so the caller keeps the error.
func promptExtensionInstallDependencies(
	cmd *cobra.Command, deps commandDeps, plan extensionInstallPlan, installErr error,
) (extensionInstallPlan, bool, error) {
	apiErr, ok := errors.AsType[*extensionOperationAPIError](installErr)
	if !ok || apiErr.payload.Code != diagnosticcontract.CodeExtensionDependenciesRequired ||
		apiErr.payload.RetryCommand != "" || len(apiErr.payload.Dependencies) == 0 {
		return plan, false, nil
	}
	interactive, err := extensionInputsInteractive(cmd, deps)
	if err != nil || !interactive {
		return plan, false, err
	}
	dependencies := apiErr.payload.Dependencies
	if err := writeExtensionDependencyPreview(cmd, dependencies); err != nil {
		return plan, false, err
	}
	prompter := newExtensionInputPrompter(cmd)
	if err := prompter.confirm(cmd.Context(), "Install these dependencies first"); err != nil {
		return plan, false, err
	}
	retry := extensionInstallPlan{Attempts: slices.Clone(plan.Attempts)}
	consent := make(map[string]contract.ExtensionDependencyConsent, len(dependencies))
	for _, dependency := range dependencies {
		item, err := prompter.dependencyGates(
			cmd.Context(),
			dependency,
			retry.Attempts[0].Dependencies[dependency.Name],
		)
		if err != nil {
			return plan, false, err
		}
		consent[dependency.Name] = item
	}
	for index := range retry.Attempts {
		merged := maps.Clone(retry.Attempts[index].Dependencies)
		if merged == nil {
			merged = make(map[string]contract.ExtensionDependencyConsent, len(consent))
		}
		maps.Copy(merged, consent)
		retry.Attempts[index].Dependencies = merged
		retry.Attempts[index].WithDependencies = true
	}
	return retry, true, nil
}

// dependencyGates runs one dependency artifact's own trust gates: unverified consent, then gateway digest.
func (p *extensionInputPrompter) dependencyGates(
	ctx context.Context, dependency contract.ExtensionDependencyPayload, item contract.ExtensionDependencyConsent,
) (contract.ExtensionDependencyConsent, error) {
	item.ExpectedDigest = cmp.Or(item.ExpectedDigest, dependency.DigestSHA256)
	name := safeLoopPromptText(dependency.Name)
	if dependency.ConsentRequired && !item.AllowUnverified {
		if err := p.confirm(ctx, name+" is not registry-verified. Install it anyway"); err != nil {
			return item, err
		}
		item.AllowUnverified = true
	}
	if digest := dependency.GatewayRequirementDigest; digest != "" && item.ConfirmGatewayDigest == "" {
		if err := p.confirm(ctx, "Confirm "+name+" gateway requirement "+digest); err != nil {
			return item, err
		}
		item.ConfirmGatewayDigest = digest
	}
	return item, nil
}

// confirm asks one y/N question; anything but yes declines the whole dependency install.
func (p *extensionInputPrompter) confirm(ctx context.Context, question string) error {
	value, _, err := p.promptOne(ctx, extensionPromptInput{
		ID: question, Prompt: question, Type: extensionInputTypeBoolean, Required: true,
	})
	if err != nil {
		return err
	}
	if string(value) != toolBoolTrue {
		return errExtensionDependenciesDeclined
	}
	return nil
}

func writeExtensionDependencyPreview(cmd *cobra.Command, dependencies []contract.ExtensionDependencyPayload) error {
	var output strings.Builder
	output.WriteString("Dependencies (experimental):\n")
	for _, dependency := range dependencies {
		trust := "registry-verified"
		if dependency.ConsentRequired {
			trust = "unverified"
		}
		version := cmp.Or(dependency.Version, dependency.InstalledVersion, "?")
		fmt.Fprintf(&output, "  • %s %s from %s %s — %s, %s", safeLoopPromptText(dependency.Name),
			safeLoopPromptText(version), dependency.Source, safeLoopPromptText(dependency.Ref), dependency.State, trust)
		if dependency.GatewayRequirementDigest != "" {
			fmt.Fprintf(&output, ", gateway %s", dependency.GatewayRequirementDigest)
		}
		fmt.Fprintf(&output, ", required by %s\n", safeLoopPromptText(strings.Join(dependency.RequiredBy, ", ")))
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), output.String()); err != nil {
		return fmt.Errorf("cli: write extension dependency preview: %w", err)
	}
	return nil
}

// extensionDependencyErrorMessage renders the experimental dependency refusals with the next step.
func extensionDependencyErrorMessage(payload contract.ExtensionOperationErrorPayload) (string, bool) {
	switch {
	case payload.Code == diagnosticcontract.CodeExtensionDependenciesRequired:
		items := make([]string, 0, len(payload.Dependencies))
		for _, dependency := range payload.Dependencies {
			items = append(items, dependency.Name+" ("+dependency.State+")")
		}
		next := "Pass --with-dependencies to install them first (experimental)."
		if payload.RetryCommand != "" {
			next = "Update first: " + payload.RetryCommand
		}
		return fmt.Sprintf("extension dependencies unmet: %s. %s (%s)", strings.Join(items, ", "), next,
			payload.Code), true
	case payload.Code == diagnosticcontract.CodeExtensionDependencyInUse:
		items := make([]string, 0, len(payload.Dependents))
		for _, dependent := range payload.Dependents {
			items = append(items, fmt.Sprintf("%s (workspace %s, profile %s)", dependent.Name,
				cmp.Or(dependent.WorkspaceID, "global"), cmp.Or(dependent.Profile, "all")))
		}
		return fmt.Sprintf("extension is still required by: %s. Pass --force to proceed (experimental). (%s)",
			strings.Join(items, ", "), payload.Code), true
	case payload.Dependency != "":
		return fmt.Sprintf("%s (dependency %s%s)", apiErrorMessage(payload.Error, ""), payload.Dependency,
			extensionDependencyGateHint(payload)), true
	}
	return "", false
}

func extensionDependencyGateHint(payload contract.ExtensionOperationErrorPayload) string {
	switch payload.Code {
	case diagnosticcontract.CodeExtensionChecksumUnverified:
		return "; pass --" + extensionAllowUnverifiedDepFlagName + " " + payload.Dependency
	case diagnosticcontract.CodeExtensionGatewayConfirmRequired:
		return "; pass --" + extensionConfirmGatewayFlagName + " " + payload.Dependency + "=" + payload.CurrentDigest
	}
	return ""
}

// extensionForceQuery adds force=true to a removal when the command carried --force.
func extensionForceQuery(ctx context.Context, query url.Values) url.Values {
	if !extensionpkg.DependencyForced(ctx) {
		return query
	}
	values := cloneQueryValues(query)
	values.Set(extensionForceFlagName, toolBoolTrue)
	return values
}

// withExtensionForce carries --force to the remove and enablement requests (rule 14).
func withExtensionForce(cmd *cobra.Command, force bool) {
	cmd.SetContext(extensionpkg.WithDependencyForce(cmd.Context(), force))
}

func registerExtensionForceFlag(cmd *cobra.Command, force *bool) {
	cmd.Flags().BoolVar(force, extensionForceFlagName, false,
		"Experimental. Proceed while an installed dependent still needs the extension; the dependent then "+
			"reports unmet_dependencies. Nothing cascades")
}
