package cli

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	core "github.com/compozy/compozy/internal/core"
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

func (s *commandState) preflightBundledSkills(cmd *cobra.Command, cfg core.Config) error {
	if !s.requiresBundledSkillPreflight() {
		return nil
	}

	agentName, bundledSkillNames, verifyBundledSkills, verifyResult, err := s.verifyBundledSkillState(cfg)
	if err != nil {
		return err
	}
	if verifyResult.HasMissing() {
		return buildMissingSkillError(cmd.CommandPath(), agentName, verifyResult)
	}
	if !verifyResult.HasDrift() {
		return nil
	}

	verifyCfg := setup.VerifyConfig{
		ResolverOptions: currentResolverOptions(),
		AgentName:       agentName,
		SkillNames:      bundledSkillNames,
	}
	return s.handleBundledSkillDrift(cmd, agentName, bundledSkillNames, verifyCfg, verifyResult, verifyBundledSkills)
}

func (s *commandState) verifyBundledSkillState(
	cfg core.Config,
) (string, []string, func(setup.VerifyConfig) (setup.VerifyResult, error), setup.VerifyResult, error) {
	agentName, err := setup.AgentNameForIDE(string(cfg.IDE))
	if err != nil {
		return "", nil, nil, setup.VerifyResult{}, err
	}

	listBundledSkills := s.listBundledSkills
	if listBundledSkills == nil {
		listBundledSkills = setup.ListBundledSkills
	}
	bundledSkills, err := listBundledSkills()
	if err != nil {
		return "", nil, nil, setup.VerifyResult{}, err
	}
	bundledSkillNames := skillNames(bundledSkills)

	verifyBundledSkills := s.verifyBundledSkills
	if verifyBundledSkills == nil {
		verifyBundledSkills = setup.VerifyBundledSkills
	}

	verifyResult, err := verifyBundledSkills(setup.VerifyConfig{
		ResolverOptions: currentResolverOptions(),
		AgentName:       agentName,
		SkillNames:      bundledSkillNames,
	})
	if err != nil {
		return "", nil, nil, setup.VerifyResult{}, err
	}
	return agentName, bundledSkillNames, verifyBundledSkills, verifyResult, nil
}

func (s *commandState) handleBundledSkillDrift(
	cmd *cobra.Command,
	agentName string,
	bundledSkillNames []string,
	verifyCfg setup.VerifyConfig,
	verifyResult setup.VerifyResult,
	verifyBundledSkills func(setup.VerifyConfig) (setup.VerifyResult, error),
) error {
	if !s.commandIsInteractive() || s.closeOnComplete {
		printBundledSkillDriftWarning(cmd, verifyResult, "continuing without updating the installed skills")
		return nil
	}

	confirmed, err := s.confirmBundledSkillRefresh(cmd, agentName, verifyResult)
	if err != nil {
		return err
	}
	if !confirmed {
		printBundledSkillDriftWarning(cmd, verifyResult, "continuing with the installed skills")
		return nil
	}

	if err := s.refreshBundledSkills(agentName, bundledSkillNames, verifyResult); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Updated bundled Compozy skills for %s (%s scope).\n",
		verifyResult.Agent.DisplayName,
		installScopeLabel(verifyResult.Scope),
	)

	return ensureBundledSkillsCurrent(verifyCfg, verifyBundledSkills)
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
	agentName string,
	verifyResult setup.VerifyResult,
) (bool, error) {
	confirmSkillRefresh := s.confirmSkillRefresh
	if confirmSkillRefresh == nil {
		confirmSkillRefresh = confirmSkillRefreshPrompt
	}

	return confirmSkillRefresh(cmd, skillRefreshPrompt{
		AgentDisplayName: verifyResult.Agent.DisplayName,
		AgentName:        agentName,
		CommandPath:      cmd.CommandPath(),
		Scope:            verifyResult.Scope,
		DriftedSkills:    verifyResult.DriftedSkillNames(),
	})
}

func (s *commandState) refreshBundledSkills(
	agentName string,
	bundledSkillNames []string,
	verifyResult setup.VerifyResult,
) error {
	installBundledSkills := s.installBundledSkills
	if installBundledSkills == nil {
		installBundledSkills = setup.InstallBundledSkills
	}

	installResult, err := installBundledSkills(setup.InstallConfig{
		ResolverOptions: currentResolverOptions(),
		SkillNames:      bundledSkillNames,
		AgentNames:      []string{agentName},
		Global:          verifyResult.Scope == setup.InstallScopeGlobal,
		Mode:            verifyResult.Mode,
	})
	if err != nil {
		return fmt.Errorf("refresh bundled skills: %w", err)
	}
	if len(installResult.Failed) > 0 {
		return fmt.Errorf("refresh bundled skills: setup completed with %d failure(s)", len(installResult.Failed))
	}
	return nil
}

func ensureBundledSkillsCurrent(
	verifyCfg setup.VerifyConfig,
	verifyBundledSkills func(setup.VerifyConfig) (setup.VerifyResult, error),
) error {
	reverified, err := verifyBundledSkills(verifyCfg)
	if err != nil {
		return fmt.Errorf("re-verify bundled skills: %w", err)
	}
	if reverified.HasMissing() {
		return fmt.Errorf(
			"re-verify bundled skills for %s: missing skills remain: %s",
			reverified.Agent.DisplayName,
			strings.Join(reverified.MissingSkillNames(), ", "),
		)
	}
	if reverified.HasDrift() {
		return fmt.Errorf(
			"re-verify bundled skills for %s: drift remains: %s",
			reverified.Agent.DisplayName,
			strings.Join(reverified.DriftedSkillNames(), ", "),
		)
	}
	return nil
}

func (s *commandState) requiresBundledSkillPreflight() bool {
	return s.kind == commandKindStart || s.kind == commandKindFixReviews
}

func buildMissingSkillError(commandPath, agentName string, result setup.VerifyResult) error {
	missing := strings.Join(result.MissingSkillNames(), ", ")

	switch result.Scope {
	case setup.InstallScopeProject:
		return fmt.Errorf(
			"%s requires bundled Compozy skills for %s. The project-local install is missing: %s. Run `compozy setup --agent %s` to update project skills, or `compozy setup --agent %s --global` to install globally",
			commandPath,
			result.Agent.DisplayName,
			missing,
			agentName,
			agentName,
		)
	case setup.InstallScopeGlobal:
		return fmt.Errorf(
			"%s requires bundled Compozy skills for %s. The global install is missing: %s. Run `compozy setup --agent %s --global` to update global skills, or `compozy setup --agent %s` to install project-local skills",
			commandPath,
			result.Agent.DisplayName,
			missing,
			agentName,
			agentName,
		)
	default:
		return fmt.Errorf(
			"%s requires bundled Compozy skills for %s. No Compozy skills were found in project or global scope; missing skills: %s. Run `compozy setup --agent %s` to install project-local skills, or `compozy setup --agent %s --global` to install globally",
			commandPath,
			result.Agent.DisplayName,
			missing,
			agentName,
			agentName,
		)
	}
}

func printBundledSkillDriftWarning(cmd *cobra.Command, result setup.VerifyResult, suffix string) {
	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Warning: bundled Compozy skills for %s differ from the installed %s scope: %s; %s.\n",
		result.Agent.DisplayName,
		installScopeLabel(result.Scope),
		strings.Join(result.DriftedSkillNames(), ", "),
		suffix,
	)
}

func confirmSkillRefreshPrompt(cmd *cobra.Command, prompt skillRefreshPrompt) (bool, error) {
	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Bundled Compozy skills for %s differ from the installed %s scope: %s.\n",
		prompt.AgentDisplayName,
		installScopeLabel(prompt.Scope),
		strings.Join(prompt.DriftedSkills, ", "),
	)

	confirmed := false
	field := huh.NewConfirm().
		Key("confirm").
		Title("Update bundled Compozy skills now?").
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
