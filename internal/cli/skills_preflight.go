package cli

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/huh/v2"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type skillRefreshPrompt struct {
	AgentDisplayName string
	AgentName        string
	CommandPath      string
	Scope            setup.InstallScope
	DriftedSkills    []string
}

type requiredSkillState struct {
	AgentName         string
	BundledSkillNames []string
	ExtensionPacks    []setup.SkillPackSource
	Bundled           setup.VerifyResult
	Extensions        setup.ExtensionVerifyResult
}

func (s *commandState) preflightBundledSkills(
	cmd *cobra.Command,
	cfg core.Config,
	extensionPacks []setup.SkillPackSource,
) error {
	if !s.requiresBundledSkillPreflight() {
		return nil
	}

	verifyState, err := s.verifyRequiredSkillState(cfg, extensionPacks)
	if err != nil {
		return err
	}
	if verifyState.HasBlockingMissing() {
		return buildMissingSkillError(cmd.CommandPath(), verifyState.AgentName, verifyState)
	}
	if !verifyState.HasRefreshableChanges() {
		return nil
	}

	return s.handleBundledSkillDrift(cmd, verifyState)
}

func (s *commandState) verifyRequiredSkillState(
	cfg core.Config,
	extensionPacks []setup.SkillPackSource,
) (requiredSkillState, error) {
	agentName, err := agent.SetupAgentName(string(cfg.IDE))
	if err != nil {
		return requiredSkillState{}, err
	}

	listBundledSkills := s.listBundledSkills
	if listBundledSkills == nil {
		listBundledSkills = setup.ListBundledSkills
	}
	bundledSkills, err := listBundledSkills()
	if err != nil {
		return requiredSkillState{}, err
	}
	bundledSkillNames := skillNames(bundledSkills)

	verifyBundledSkills := s.verifyBundledSkills
	if verifyBundledSkills == nil {
		verifyBundledSkills = setup.VerifyBundledSkills
	}

	bundledResult, err := verifyBundledSkills(setup.VerifyConfig{
		ResolverOptions: currentResolverOptions(),
		AgentName:       agentName,
		SkillNames:      bundledSkillNames,
	})
	if err != nil {
		return requiredSkillState{}, err
	}

	verifyExtensionSkills := s.verifyExtensionSkills
	if verifyExtensionSkills == nil {
		verifyExtensionSkills = setup.VerifyExtensionSkillPacks
	}
	extensionResult, err := verifyExtensionSkills(setup.ExtensionVerifyConfig{
		ResolverOptions: currentResolverOptions(),
		AgentName:       agentName,
		Packs:           extensionPacks,
		ScopeHint:       bundledResult.Scope,
	})
	if err != nil {
		return requiredSkillState{}, err
	}

	return requiredSkillState{
		AgentName:         agentName,
		BundledSkillNames: bundledSkillNames,
		ExtensionPacks:    append([]setup.SkillPackSource(nil), extensionPacks...),
		Bundled:           bundledResult,
		Extensions:        extensionResult,
	}, nil
}

func (s *commandState) handleBundledSkillDrift(
	cmd *cobra.Command,
	verifyState requiredSkillState,
) error {
<<<<<<< HEAD
	if !s.commandIsInteractive() || s.closeOnComplete {
		printBundledSkillDriftWarning(cmd, verifyResult, "continuing without updating the installed skills")
=======
	if !s.commandIsInteractive() {
		printBundledSkillDriftWarning(cmd, verifyState, "continuing without updating the installed skills")
>>>>>>> compozy/main
		return nil
	}

	confirmed, err := s.confirmBundledSkillRefresh(cmd, verifyState)
	if err != nil {
		return err
	}
	if !confirmed {
		printBundledSkillDriftWarning(cmd, verifyState, "continuing with the installed skills")
		return nil
	}

	if err := s.refreshBundledSkills(verifyState); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Updated required Compozy skills for %s (%s scope).\n",
		verifyState.AgentDisplayName(),
		installScopeLabel(verifyState.Scope()),
	)

	return ensureBundledSkillsCurrent(verifyState, s.verifyBundledSkills, s.verifyExtensionSkills)
}

func (s *commandState) commandIsInteractive() bool {
	isInteractive := s.isInteractive
	if isInteractive == nil {
		isInteractive = isInteractiveTerminal
	}
	return isInteractive()
}

func (s *commandState) confirmBundledSkillRefresh(
	cmd *cobra.Command,
	verifyState requiredSkillState,
) (bool, error) {
	confirmSkillRefresh := s.confirmSkillRefresh
	if confirmSkillRefresh == nil {
		confirmSkillRefresh = confirmSkillRefreshPrompt
	}

	return confirmSkillRefresh(cmd, skillRefreshPrompt{
		AgentDisplayName: verifyState.AgentDisplayName(),
		AgentName:        verifyState.AgentName,
		CommandPath:      cmd.CommandPath(),
		Scope:            verifyState.Scope(),
		DriftedSkills:    verifyState.RefreshSkillNames(),
	})
}

func (s *commandState) refreshBundledSkills(verifyState requiredSkillState) error {
	installBundledSkills := s.installBundledSkills
	if installBundledSkills == nil {
		installBundledSkills = setup.InstallBundledSkills
	}

	global := verifyState.Scope() == setup.InstallScopeGlobal
	mode := verifyState.Mode()

	installResult, err := installBundledSkills(setup.InstallConfig{
		ResolverOptions: currentResolverOptions(),
		SkillNames:      verifyState.BundledSkillNames,
		AgentNames:      []string{verifyState.AgentName},
		Global:          global,
		Mode:            mode,
	})
	if err != nil {
		return fmt.Errorf("refresh bundled skills: %w", err)
	}
	if len(installResult.Failed) > 0 {
		return fmt.Errorf("refresh bundled skills: setup completed with %d failure(s)", len(installResult.Failed))
	}

	if len(verifyState.ExtensionPacks) == 0 {
		return nil
	}

	installExtensionSkills := s.installExtensionSkills
	if installExtensionSkills == nil {
		installExtensionSkills = setup.InstallExtensionSkillPacks
	}
	extensionResult, err := installExtensionSkills(setup.ExtensionInstallConfig{
		ResolverOptions: currentResolverOptions(),
		Packs:           verifyState.ExtensionPacks,
		AgentNames:      []string{verifyState.AgentName},
		Global:          global,
		Mode:            mode,
	})
	if err != nil {
		return fmt.Errorf("refresh extension skills: %w", err)
	}
	if len(extensionResult.Failed) > 0 {
		return fmt.Errorf("refresh extension skills: setup completed with %d failure(s)", len(extensionResult.Failed))
	}
	return nil
}

func ensureBundledSkillsCurrent(
	verifyState requiredSkillState,
	verifyBundledSkills func(setup.VerifyConfig) (setup.VerifyResult, error),
	verifyExtensionSkills func(setup.ExtensionVerifyConfig) (setup.ExtensionVerifyResult, error),
) error {
	if verifyBundledSkills == nil {
		verifyBundledSkills = setup.VerifyBundledSkills
	}
	if verifyExtensionSkills == nil {
		verifyExtensionSkills = setup.VerifyExtensionSkillPacks
	}

	reverifiedBundled, err := verifyBundledSkills(setup.VerifyConfig{
		ResolverOptions: currentResolverOptions(),
		AgentName:       verifyState.AgentName,
		SkillNames:      verifyState.BundledSkillNames,
	})
	if err != nil {
		return fmt.Errorf("re-verify bundled skills: %w", err)
	}
	reverifiedExtensions, err := verifyExtensionSkills(setup.ExtensionVerifyConfig{
		ResolverOptions: currentResolverOptions(),
		AgentName:       verifyState.AgentName,
		Packs:           verifyState.ExtensionPacks,
		ScopeHint:       verifyState.Scope(),
	})
	if err != nil {
		return fmt.Errorf("re-verify extension skills: %w", err)
	}

	reverified := requiredSkillState{
		AgentName:         verifyState.AgentName,
		BundledSkillNames: verifyState.BundledSkillNames,
		ExtensionPacks:    verifyState.ExtensionPacks,
		Bundled:           reverifiedBundled,
		Extensions:        reverifiedExtensions,
	}
	if reverified.HasMissing() {
		return fmt.Errorf(
			"re-verify required skills for %s: missing skills remain: %s",
			reverified.AgentDisplayName(),
			strings.Join(reverified.MissingSkillNames(), ", "),
		)
	}
	if reverified.HasDrift() {
		return fmt.Errorf(
			"re-verify required skills for %s: drift remains: %s",
			reverified.AgentDisplayName(),
			strings.Join(reverified.DriftedSkillNames(), ", "),
		)
	}
	return nil
}

func (s *commandState) requiresBundledSkillPreflight() bool {
	return s.kind == commandKindStart || s.kind == commandKindFixReviews
}

func buildMissingSkillError(commandPath, agentName string, result requiredSkillState) error {
	missing := strings.Join(result.BlockingMissingSkillNames(), ", ")

	switch result.Scope() {
	case setup.InstallScopeProject:
		return fmt.Errorf(
			"%s requires Compozy and enabled extension skills for %s. The project-local install is missing: %s. Run `compozy setup --agent %s` to update project skills, or `compozy setup --agent %s --global` to install globally",
			commandPath,
			result.AgentDisplayName(),
			missing,
			agentName,
			agentName,
		)
	case setup.InstallScopeGlobal:
		return fmt.Errorf(
			"%s requires Compozy and enabled extension skills for %s. The global install is missing: %s. Run `compozy setup --agent %s --global` to update global skills, or `compozy setup --agent %s` to install project-local skills",
			commandPath,
			result.AgentDisplayName(),
			missing,
			agentName,
			agentName,
		)
	default:
		return fmt.Errorf(
			"%s requires Compozy and enabled extension skills for %s. No compatible skills were found in project or global scope; missing skills: %s. Run `compozy setup --agent %s` to install project-local skills, or `compozy setup --agent %s --global` to install globally",
			commandPath,
			result.AgentDisplayName(),
			missing,
			agentName,
			agentName,
		)
	}
}

func printBundledSkillDriftWarning(cmd *cobra.Command, result requiredSkillState, suffix string) {
	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Warning: required Compozy skills for %s differ from the installed %s scope: %s; %s.\n",
		result.AgentDisplayName(),
		installScopeLabel(result.Scope()),
		strings.Join(result.RefreshSkillNames(), ", "),
		suffix,
	)
}

func confirmSkillRefreshPrompt(cmd *cobra.Command, prompt skillRefreshPrompt) (bool, error) {
	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Required Compozy skills for %s differ from the installed %s scope: %s.\n",
		prompt.AgentDisplayName,
		installScopeLabel(prompt.Scope),
		strings.Join(prompt.DriftedSkills, ", "),
	)

	confirmed := false
	field := huh.NewConfirm().
		Key("confirm").
		Title("Update required Compozy skills now?").
		Description(
			fmt.Sprintf(
				"Runs the equivalent of `compozy setup --agent %s%s` before %s continues.",
				prompt.AgentName,
				scopeInstallFlag(prompt.Scope),
				prompt.CommandPath,
			),
		).
		Value(&confirmed)
	if err := runPromptField(field); err != nil {
		return false, fmt.Errorf("confirm bundled skill refresh: %w", err)
	}
	return confirmed, nil
}

func installScopeLabel(scope setup.InstallScope) string {
	switch scope {
	case setup.InstallScopeGlobal:
		return string(setup.InstallScopeGlobal)
	case setup.InstallScopeProject:
		return string(setup.InstallScopeProject)
	default:
		return "unknown"
	}
}

func scopeInstallFlag(scope setup.InstallScope) string {
	if scope == setup.InstallScopeGlobal {
		return " --global"
	}
	return ""
}

func (s requiredSkillState) Scope() setup.InstallScope {
	if s.Bundled.Scope != "" && s.Bundled.Scope != setup.InstallScopeUnknown {
		return s.Bundled.Scope
	}
	if s.Extensions.Scope != "" && s.Extensions.Scope != setup.InstallScopeUnknown {
		return s.Extensions.Scope
	}
	return setup.InstallScopeUnknown
}

func (s requiredSkillState) Mode() setup.InstallMode {
	if len(s.Bundled.Skills) > 0 {
		return s.Bundled.Mode
	}
	if len(s.Extensions.Skills) > 0 {
		return s.Extensions.Mode
	}
	return setup.InstallModeCopy
}

func (s requiredSkillState) AgentDisplayName() string {
	if s.Bundled.Agent.DisplayName != "" {
		return s.Bundled.Agent.DisplayName
	}
	return s.Extensions.Agent.DisplayName
}

func (s requiredSkillState) MissingSkillNames() []string {
	names := append([]string(nil), s.Bundled.MissingSkillNames()...)
	names = append(names, s.Extensions.MissingSkillNames()...)
	return uniqueSortedStrings(names)
}

func (s requiredSkillState) DriftedSkillNames() []string {
	names := append([]string(nil), s.Bundled.DriftedSkillNames()...)
	names = append(names, s.Extensions.DriftedSkillNames()...)
	return uniqueSortedStrings(names)
}

func (s requiredSkillState) HasMissing() bool {
	return s.Bundled.HasMissing() || s.Extensions.HasMissing()
}

func (s requiredSkillState) HasDrift() bool {
	return s.Bundled.HasDrift() || s.Extensions.HasDrift()
}

func (s requiredSkillState) BlockingMissingSkillNames() []string {
	return uniqueSortedStrings(s.Bundled.MissingSkillNames())
}

func (s requiredSkillState) HasBlockingMissing() bool {
	return s.Bundled.HasMissing()
}

func (s requiredSkillState) RefreshSkillNames() []string {
	names := append([]string(nil), s.Bundled.DriftedSkillNames()...)
	names = append(names, s.Extensions.DriftedSkillNames()...)
	names = append(names, s.Extensions.MissingSkillNames()...)
	return uniqueSortedStrings(names)
}

func (s requiredSkillState) HasRefreshableChanges() bool {
	return len(s.RefreshSkillNames()) > 0
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
