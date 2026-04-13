package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type setupCommandState struct {
	agentNames []string
	skillNames []string
	global     bool
	copy       bool
	list       bool
	yes        bool
	all        bool

	loadCatalog           func(context.Context, setup.ResolverOptions) (setup.EffectiveCatalog, error)
	listAgents            func(setup.ResolverOptions) ([]setup.Agent, error)
	detectAgents          func(setup.ResolverOptions) ([]setup.Agent, error)
	previewSkills         setupSkillPreviewFunc
	previewReusableAgents func(setup.ResolverOptions, []setup.ReusableAgent) ([]setup.ReusableAgentPreviewItem, error)
	installSkills         setupSkillInstallFunc
	installReusableAgents setupReusableAgentInstallFunc
	isInteractive         func() bool
}

type setupSkillPreviewFunc func(
	setup.ResolverOptions,
	[]setup.Skill,
	[]string,
	bool,
	setup.InstallMode,
) ([]setup.PreviewItem, error)

type setupSkillInstallFunc func(
	setup.ResolverOptions,
	[]setup.Skill,
	[]string,
	bool,
	setup.InstallMode,
) ([]setup.SuccessItem, []setup.FailureItem, error)

type setupReusableAgentInstallFunc func(
	setup.ResolverOptions,
	[]setup.ReusableAgent,
) ([]setup.ReusableAgentSuccessItem, []setup.ReusableAgentFailureItem, error)

type setupInstallPlan struct {
	Config               setup.InstallConfig
	Skills               []setup.Skill
	ReusableAgents       []setup.ReusableAgent
	Previews             []setup.PreviewItem
	ReusableAgentPreview []setup.ReusableAgentPreviewItem
}

func newSetupCommand(_ *kernel.Dispatcher) *cobra.Command {
	state := newSetupCommandState()
	cmd := &cobra.Command{
		Use:          "setup",
		Short:        "Install Compozy core assets plus setup assets shipped by enabled extensions",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Install Compozy's core public skills, any additional skills shipped by enabled
extensions, and global reusable agents under ~/.compozy/agents.

The command can run interactively or entirely from flags.`,
		Example: `  compozy setup
  compozy setup --list
  compozy setup --agent codex --agent claude --skill create-prd --skill create-techspec --yes
  compozy setup --all
  compozy setup --agent cursor --global --copy --yes`,
		RunE: state.run,
	}

	cmd.Flags().StringSliceVarP(&state.agentNames, "agent", "a", nil, "Target agent/editor name (repeatable)")
	cmd.Flags().StringSliceVarP(&state.skillNames, "skill", "s", nil, "Setup skill name to install (repeatable)")
	cmd.Flags().BoolVarP(&state.global, "global", "g", false, "Install to the user directory instead of the project")
	cmd.Flags().BoolVar(&state.copy, "copy", false, "Copy files instead of symlinking to agent directories")
	cmd.Flags().BoolVarP(&state.list, "list", "l", false, "List setup assets without installing")
	cmd.Flags().BoolVarP(&state.yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().
		BoolVar(&state.all, "all", false, "Install all available setup skills to all supported agents without prompts")
	return cmd
}

func newSetupCommandState() *setupCommandState {
	return &setupCommandState{
		loadCatalog:           loadEffectiveSetupCatalog,
		listAgents:            setup.SupportedAgents,
		detectAgents:          setup.DetectInstalledAgents,
		previewSkills:         setup.PreviewSelectedSkills,
		previewReusableAgents: setup.PreviewReusableAgentInstall,
		installSkills:         setup.InstallSelectedSkills,
		installReusableAgents: setup.InstallReusableAgents,
		isInteractive:         isInteractiveTerminal,
	}
}

func (s *setupCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	resolver := s.resolverOptions()
	catalog, err := s.loadCatalog(ctx, resolver)
	if err != nil {
		return err
	}
	if s.list {
		printSetupAssets(cmd, catalog.Skills, catalog.ReusableAgents, catalog.Conflicts)
		return nil
	}

	if !s.yes && s.isInteractive() {
		printWelcomeHeader(cmd)
	}

	if err := s.prepareRunMode(); err != nil {
		return err
	}

	supportedAgents, detectedAgents, err := s.loadAgents(resolver)
	if err != nil {
		return err
	}

	cfg, previews, reusableAgentPreviews, err := s.buildInstallPlan(
		cmd,
		catalog,
		resolver,
		supportedAgents,
		detectedAgents,
	)
	if err != nil {
		return err
	}
	printSetupWarnings(cmd, catalog.Conflicts)
	if err := s.confirmPlan(cmd, previews, reusableAgentPreviews, cfg.Global, cfg.Mode); err != nil {
		return err
	}

	return s.executeInstall(cmd, setupInstallPlan{
		Config:               cfg,
		Skills:               previewsToSkills(previews),
		ReusableAgents:       append([]setup.ReusableAgent(nil), catalog.ReusableAgents...),
		Previews:             previews,
		ReusableAgentPreview: reusableAgentPreviews,
	})
}

func (s *setupCommandState) prepareRunMode() error {
	if s.all {
		s.yes = true
	}
	if !s.yes && !s.isInteractive() {
		return errors.New("compozy setup requires an interactive terminal unless --yes is provided")
	}
	return nil
}

func (s *setupCommandState) resolverOptions() setup.ResolverOptions {
	return currentResolverOptions()
}

func currentResolverOptions() setup.ResolverOptions {
	return setup.ResolverOptions{
		CodeXHome:       strings.TrimSpace(os.Getenv("CODEX_HOME")),
		ClaudeConfigDir: strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")),
		XDGConfigHome:   strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")),
	}
}

func (s *setupCommandState) loadAgents(resolver setup.ResolverOptions) ([]setup.Agent, []setup.Agent, error) {
	supportedAgents, err := s.listAgents(resolver)
	if err != nil {
		return nil, nil, err
	}

	detectedAgents, err := s.detectAgents(resolver)
	if err != nil {
		return nil, nil, err
	}
	return supportedAgents, detectedAgents, nil
}

func (s *setupCommandState) buildInstallPlan(
	cmd *cobra.Command,
	catalog setup.EffectiveCatalog,
	resolver setup.ResolverOptions,
	supportedAgents []setup.Agent,
	detectedAgents []setup.Agent,
) (setup.InstallConfig, []setup.PreviewItem, []setup.ReusableAgentPreviewItem, error) {
	selectedSkillNames, err := s.resolveSkillSelection(catalog.Skills)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}
	selectedSkills, err := setup.SelectSkills(catalog.Skills, selectedSkillNames)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}

	selectedAgents, err := s.resolveAgentSelection(supportedAgents, detectedAgents)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}

	globalScope, err := s.resolveScope(cmd, selectedAgents)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}

	mode, err := s.resolveInstallMode(cmd, supportedAgents, selectedAgents, globalScope)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}

	cfg := setup.InstallConfig{
		ResolverOptions: resolver,
		SkillNames:      selectedSkillNames,
		AgentNames:      selectedAgents,
		Global:          globalScope,
		Mode:            mode,
	}
	previews, err := s.previewSkills(resolver, selectedSkills, selectedAgents, globalScope, mode)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}
	reusableAgentPreviews, err := s.previewReusableAgents(resolver, catalog.ReusableAgents)
	if err != nil {
		return setup.InstallConfig{}, nil, nil, err
	}
	return cfg, previews, reusableAgentPreviews, nil
}

func (s *setupCommandState) confirmPlan(
	cmd *cobra.Command,
	previews []setup.PreviewItem,
	reusableAgentPreviews []setup.ReusableAgentPreviewItem,
	global bool,
	mode setup.InstallMode,
) error {
	if s.yes {
		return nil
	}

	printPreviewSummary(cmd, previews, reusableAgentPreviews, global, mode)
	confirmed, err := confirmSetup()
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("setup canceled")
	}
	return nil
}

func (s *setupCommandState) executeInstall(cmd *cobra.Command, plan setupInstallPlan) error {
	result, err := s.installPlan(plan)
	printInstallResult(cmd, result)
	if err != nil {
		return err
	}
	failureCount := len(result.Failed) + len(result.ReusableAgentsFailed)
	if failureCount > 0 {
		return fmt.Errorf("setup completed with %d failure(s)", failureCount)
	}
	return nil
}

func (s *setupCommandState) installPlan(plan setupInstallPlan) (*setup.Result, error) {
	successful, failed, err := s.installSkills(
		plan.Config.ResolverOptions,
		plan.Skills,
		plan.Config.AgentNames,
		plan.Config.Global,
		plan.Config.Mode,
	)
	if err != nil {
		return nil, err
	}

	result := &setup.Result{
		Global:     plan.Config.Global,
		Mode:       plan.Config.Mode,
		Successful: successful,
		Failed:     failed,
	}

	successfulReusableAgents, failedReusableAgents, err := s.installReusableAgents(
		plan.Config.ResolverOptions,
		plan.ReusableAgents,
	)
	if err != nil {
		return result, fmt.Errorf("install reusable agents: %w", err)
	}
	result.ReusableAgentsSuccessful = successfulReusableAgents
	result.ReusableAgentsFailed = failedReusableAgents
	return result, nil
}

func (s *setupCommandState) resolveSkillSelection(skills []setup.Skill) ([]string, error) {
	if len(s.skillNames) > 0 {
		return append([]string(nil), s.skillNames...), nil
	}
	if s.all || s.yes {
		return skillNames(skills), nil
	}

	selected := skillNames(skills)

	maxNameLen := 0
	for i := range skills {
		if len(skills[i].Name) > maxNameLen {
			maxNameLen = len(skills[i].Name)
		}
	}

	options := make([]huh.Option[string], 0, len(skills))
	for i := range skills {
		label := fmt.Sprintf("%-*s  %s", maxNameLen, skills[i].Name, shortDescription(skills[i].Description))
		options = append(options, huh.NewOption(label, skills[i].Name))
	}

	field := huh.NewMultiSelect[string]().
		Key("skills").
		Title("Setup Skills").
		Description("Select the Compozy skills to install, including enabled extension assets").
		Options(options...).
		Value(&selected).
		Limit(len(skills)).
		Validate(func(values []string) error {
			if len(values) == 0 {
				return errors.New("select at least one skill")
			}
			return nil
		})
	if err := runPromptField(field); err != nil {
		return nil, fmt.Errorf("select setup skills: %w", err)
	}
	return selected, nil
}

func (s *setupCommandState) resolveAgentSelection(
	supported []setup.Agent,
	detected []setup.Agent,
) ([]string, error) {
	if len(s.agentNames) > 0 {
		return append([]string(nil), s.agentNames...), nil
	}
	if s.all {
		return agentNames(supported), nil
	}
	if s.yes {
		if len(detected) == 0 {
			return nil, errors.New("no agents detected; rerun with --agent or use interactive mode")
		}
		return agentNames(detected), nil
	}

	preselected := defaultAgentSelection(supported, detected)
	options := make([]huh.Option[string], 0, len(supported))
	for _, agent := range supported {
		scopeHint := agent.ProjectRootDir
		if agent.Universal {
			scopeHint = ".agents/skills"
		}
		label := fmt.Sprintf("%s [%s]", agent.DisplayName, scopeHint)
		options = append(options, huh.NewOption(label, agent.Name))
	}

	field := huh.NewMultiSelect[string]().
		Key("agents").
		Title("Target Agents").
		Description("Select the editors/agents where Compozy should install skills").
		Options(options...).
		Value(&preselected).
		Limit(len(supported)).
		Validate(func(values []string) error {
			if len(values) == 0 {
				return errors.New("select at least one agent")
			}
			return nil
		})
	if err := runPromptField(field); err != nil {
		return nil, fmt.Errorf("select target agents: %w", err)
	}
	return preselected, nil
}

func (s *setupCommandState) resolveScope(cmd *cobra.Command, agents []string) (bool, error) {
	if cmd.Flags().Changed("global") || s.yes {
		return s.global, nil
	}
	if len(agents) == 0 {
		return false, errors.New("resolve installation scope: no agents selected")
	}

	selection := "project"
	field := huh.NewSelect[string]().
		Key("scope").
		Title("Installation Scope").
		Description("Choose whether skills are shared per project or available globally").
		Options(
			huh.NewOption("Project (recommended)", "project"),
			huh.NewOption("Global", "global"),
		).
		Value(&selection)
	if err := runPromptField(field); err != nil {
		return false, fmt.Errorf("select installation scope: %w", err)
	}
	return selection == "global", nil
}

func (s *setupCommandState) resolveInstallMode(
	cmd *cobra.Command,
	supportedAgents []setup.Agent,
	selectedAgents []string,
	global bool,
) (setup.InstallMode, error) {
	if s.copy {
		return setup.InstallModeCopy, nil
	}

	roots := make(map[string]struct{}, len(selectedAgents))
	selected, err := setup.SelectAgents(supportedAgents, selectedAgents)
	if err != nil {
		return "", err
	}
	for _, agent := range selected {
		root := agent.ProjectRootDir
		if global {
			root = agent.GlobalRootDir
		}
		if agent.Universal {
			root = ".agents/skills"
		}
		roots[root] = struct{}{}
	}

	if len(roots) <= 1 {
		return setup.InstallModeCopy, nil
	}
	if s.yes || cmd.Flags().Changed("copy") {
		return setup.InstallModeSymlink, nil
	}

	selection := string(setup.InstallModeSymlink)
	field := huh.NewSelect[string]().
		Key("mode").
		Title("Installation Method").
		Description("Symlink keeps one canonical copy; copy duplicates files into each agent directory").
		Options(
			huh.NewOption("Symlink (recommended)", string(setup.InstallModeSymlink)),
			huh.NewOption("Copy", string(setup.InstallModeCopy)),
		).
		Value(&selection)
	if err := runPromptField(field); err != nil {
		return "", fmt.Errorf("select installation method: %w", err)
	}
	return setup.InstallMode(selection), nil
}

// --- Styled output functions ---

func printWelcomeHeader(cmd *cobra.Command) {
	styles := newCLIChromeStyles()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.title.Render("COMPOZY // SETUP"),
		styles.subtitle.Render("Install core and enabled extension assets plus global reusable agents"),
	)

	lipgloss.Fprintln(cmd.OutOrStdout(), styles.box.Render(content))
}

func printSetupAssets(
	cmd *cobra.Command,
	skills []setup.Skill,
	reusableAgents []setup.ReusableAgent,
	conflicts []setup.CatalogConflict,
) {
	if len(skills) == 0 && len(reusableAgents) == 0 {
		printSetupWarnings(cmd, conflicts)
		return
	}
	styles := newCLIChromeStyles()

	if len(skills) > 0 {
		maxNameLen := 0
		for i := range skills {
			if len(skills[i].Name) > maxNameLen {
				maxNameLen = len(skills[i].Name)
			}
		}

		lipgloss.Fprintln(cmd.OutOrStdout(), styles.sectionTitle.Render("Setup Skills"))

		for i := range skills {
			skill := &skills[i]
			name := styles.skill.Render(padRight(skill.Name, maxNameLen))
			source := styles.value.Render(
				"[" + setupAssetSourceLabel(skill.Origin, skill.ExtensionSource, skill.ExtensionName) + "]",
			)
			desc := styles.path.Render(skill.Description)
			lipgloss.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", name, source, desc)
		}
	}

	if len(reusableAgents) > 0 {
		if len(skills) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())
		}

		maxNameLen := 0
		for i := range reusableAgents {
			if len(reusableAgents[i].Name) > maxNameLen {
				maxNameLen = len(reusableAgents[i].Name)
			}
		}

		lipgloss.Fprintln(cmd.OutOrStdout(), styles.sectionTitle.Render("Global Reusable Agents"))
		for i := range reusableAgents {
			reusableAgent := &reusableAgents[i]
			name := styles.agent.Render(padRight(reusableAgent.Name, maxNameLen))
			source := styles.value.Render(
				"[" + setupAssetSourceLabel(
					reusableAgent.Origin,
					reusableAgent.ExtensionSource,
					reusableAgent.ExtensionName,
				) + "]",
			)
			desc := styles.path.Render(reusableAgent.Description)
			lipgloss.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", name, source, desc)
		}
	}

	printSetupWarnings(cmd, conflicts)
}

func printSetupWarnings(cmd *cobra.Command, conflicts []setup.CatalogConflict) {
	if len(conflicts) == 0 {
		return
	}

	styles := newCLIChromeStyles()
	w := cmd.OutOrStdout()
	fmt.Fprintln(w)
	lipgloss.Fprintln(w, styles.sectionTitle.Render("Warnings"))
	for i := range conflicts {
		lipgloss.Fprintf(w, "  %s  %s\n", styles.warn.Render("!"), formatSetupCatalogConflict(conflicts[i]))
	}
}

func printPreviewSummary(
	cmd *cobra.Command,
	previews []setup.PreviewItem,
	reusableAgentPreviews []setup.ReusableAgentPreviewItem,
	global bool,
	mode setup.InstallMode,
) {
	if len(previews) == 0 && len(reusableAgentPreviews) == 0 {
		return
	}
	styles := newCLIChromeStyles()

	cwd, homeDir := displayRoots()

	w := cmd.OutOrStdout()
	lipgloss.Fprintln(w, styles.sectionTitle.Render("Installation Summary"))
	fmt.Fprintln(w)

	lipgloss.Fprintf(w, "  %s  %s\n", styles.label.Render("Scope "), styles.value.Render(scopeLabel(global)))
	lipgloss.Fprintf(w, "  %s  %s\n", styles.label.Render("Method"), styles.value.Render(string(mode)))
	fmt.Fprintln(w)
	lipgloss.Fprintln(w, styles.separator.Render("  "+strings.Repeat("─", 50)))
	fmt.Fprintln(w)

	maxSkillLen := 0
	maxAgentLen := 0
	for i := range previews {
		if len(previews[i].Skill.Name) > maxSkillLen {
			maxSkillLen = len(previews[i].Skill.Name)
		}
		if len(previews[i].Agent.DisplayName) > maxAgentLen {
			maxAgentLen = len(previews[i].Agent.DisplayName)
		}
	}

	for i := range previews {
		preview := &previews[i]
		name := styles.skill.Render(padRight(preview.Skill.Name, maxSkillLen))
		arrow := styles.arrow.Render("->")
		agent := styles.agent.Render(padRight(preview.Agent.DisplayName, maxAgentLen))
		path := styles.path.Render(shortenPath(preview.TargetPath, cwd, homeDir))

		line := fmt.Sprintf("    %s  %s  %s  %s", name, arrow, agent, path)

		if mode == setup.InstallModeSymlink && !sameInstallPath(preview.CanonicalPath, preview.TargetPath) {
			via := styles.path.Render("via " + shortenPath(preview.CanonicalPath, cwd, homeDir))
			line += "  " + via
		}
		if preview.WillOverwrite {
			line += "  " + styles.warn.Render("[overwrite]")
		}
		lipgloss.Fprintln(w, line)
	}

	if len(reusableAgentPreviews) > 0 {
		fmt.Fprintln(w)
		lipgloss.Fprintln(w, styles.sectionTitle.Render("Global Reusable Agents"))
		fmt.Fprintln(w)

		maxReusableAgentLen := 0
		for i := range reusableAgentPreviews {
			if len(reusableAgentPreviews[i].ReusableAgent.Name) > maxReusableAgentLen {
				maxReusableAgentLen = len(reusableAgentPreviews[i].ReusableAgent.Name)
			}
		}

		for i := range reusableAgentPreviews {
			preview := &reusableAgentPreviews[i]
			name := styles.agent.Render(padRight(preview.ReusableAgent.Name, maxReusableAgentLen))
			path := styles.path.Render(shortenPath(preview.TargetPath, cwd, homeDir))

			line := fmt.Sprintf("    %s  %s", name, path)
			if preview.WillOverwrite {
				line += "  " + styles.warn.Render("[overwrite]")
			}
			lipgloss.Fprintln(w, line)
		}
	}
	fmt.Fprintln(w)
}

func printInstallResult(cmd *cobra.Command, result *setup.Result) {
	if result == nil {
		return
	}
	styles := newCLIChromeStyles()
	cwd, homeDir := displayRoots()
	w := cmd.OutOrStdout()
	maxSkillLen, maxAgentLen := computeColumnWidths(result.Successful, result.Failed)
	printSkillInstallSuccesses(w, styles, result.Successful, cwd, homeDir, maxSkillLen, maxAgentLen)
	if len(result.Successful) > 0 && len(result.Failed) > 0 {
		fmt.Fprintln(w)
		lipgloss.Fprintln(w, styles.separator.Render("  "+strings.Repeat("─", 50)))
	}
	printSkillInstallFailures(w, styles, result.Failed, cwd, homeDir, maxSkillLen, maxAgentLen)

	hasSkillResults := len(result.Successful) > 0 || len(result.Failed) > 0
	hasReusableAgentResults := len(result.ReusableAgentsSuccessful) > 0 || len(result.ReusableAgentsFailed) > 0
	if hasSkillResults && hasReusableAgentResults {
		fmt.Fprintln(w)
		lipgloss.Fprintln(w, styles.separator.Render("  "+strings.Repeat("─", 50)))
		fmt.Fprintln(w)
	}
	printReusableAgentInstallResults(w, styles, result, cwd, homeDir)
	fmt.Fprintln(w)
}

func printSkillInstallSuccesses(
	w io.Writer,
	styles cliChromeStyles,
	successful []setup.SuccessItem,
	cwd, homeDir string,
	maxSkillLen, maxAgentLen int,
) {
	if len(successful) == 0 {
		return
	}

	lipgloss.Fprintln(w, styles.successHeader.Render(
		fmt.Sprintf("  ✓ Installed (%d)", len(successful)),
	))
	fmt.Fprintln(w)

	for i := range successful {
		item := &successful[i]
		icon := styles.successIcon.Render("✓")
		name := styles.skill.Render(padRight(item.Skill.Name, maxSkillLen))
		arrow := styles.arrow.Render("->")
		agent := styles.agent.Render(padRight(item.Agent.DisplayName, maxAgentLen))
		path := styles.path.Render(shortenPath(item.Path, cwd, homeDir))

		line := fmt.Sprintf("    %s  %s  %s  %s  %s", icon, name, arrow, agent, path)
		if item.Mode == setup.InstallModeSymlink && item.SymlinkFailed {
			line += "  " + styles.warn.Render("[copied after symlink failure]")
		}
		lipgloss.Fprintln(w, line)
	}
}

func printSkillInstallFailures(
	w io.Writer,
	styles cliChromeStyles,
	failed []setup.FailureItem,
	cwd, homeDir string,
	maxSkillLen, maxAgentLen int,
) {
	if len(failed) == 0 {
		return
	}

	lipgloss.Fprintln(w, styles.failureHeader.Render(
		fmt.Sprintf("  ✗ Failed (%d)", len(failed)),
	))
	fmt.Fprintln(w)

	for i := range failed {
		item := &failed[i]
		icon := styles.failureIcon.Render("✗")
		name := styles.skill.Render(padRight(item.Skill.Name, maxSkillLen))
		arrow := styles.arrow.Render("->")
		agent := styles.agent.Render(padRight(item.Agent.DisplayName, maxAgentLen))
		path := styles.path.Render(shortenPath(item.Path, cwd, homeDir))

		lipgloss.Fprintf(w, "    %s  %s  %s  %s  %s\n", icon, name, arrow, agent, path)
		lipgloss.Fprintf(w, "       %s\n", styles.errorMessage.Render(item.Error))
	}
}

func printReusableAgentInstallResults(
	w io.Writer,
	styles cliChromeStyles,
	result *setup.Result,
	cwd, homeDir string,
) {
	if result == nil {
		return
	}
	if len(result.ReusableAgentsSuccessful) == 0 && len(result.ReusableAgentsFailed) == 0 {
		return
	}

	maxReusableAgentLen := reusableAgentColumnWidth(
		result.ReusableAgentsSuccessful,
		result.ReusableAgentsFailed,
	)

	if len(result.ReusableAgentsSuccessful) > 0 {
		lipgloss.Fprintln(w, styles.successHeader.Render(
			fmt.Sprintf("  ✓ Installed Global Reusable Agents (%d)", len(result.ReusableAgentsSuccessful)),
		))
		fmt.Fprintln(w)

		for i := range result.ReusableAgentsSuccessful {
			item := &result.ReusableAgentsSuccessful[i]
			icon := styles.successIcon.Render("✓")
			name := styles.agent.Render(padRight(item.ReusableAgent.Name, maxReusableAgentLen))
			path := styles.path.Render(shortenPath(item.Path, cwd, homeDir))
			lipgloss.Fprintf(w, "    %s  %s  %s\n", icon, name, path)
		}
	}

	if len(result.ReusableAgentsFailed) == 0 {
		return
	}
	if len(result.ReusableAgentsSuccessful) > 0 {
		fmt.Fprintln(w)
	}

	lipgloss.Fprintln(w, styles.failureHeader.Render(
		fmt.Sprintf("  ✗ Failed Global Reusable Agents (%d)", len(result.ReusableAgentsFailed)),
	))
	fmt.Fprintln(w)

	for i := range result.ReusableAgentsFailed {
		item := &result.ReusableAgentsFailed[i]
		icon := styles.failureIcon.Render("✗")
		name := styles.agent.Render(padRight(item.ReusableAgent.Name, maxReusableAgentLen))
		path := styles.path.Render(shortenPath(item.Path, cwd, homeDir))
		lipgloss.Fprintf(w, "    %s  %s  %s\n", icon, name, path)
		lipgloss.Fprintf(w, "       %s\n", styles.errorMessage.Render(item.Error))
	}
}

func reusableAgentColumnWidth(
	successful []setup.ReusableAgentSuccessItem,
	failed []setup.ReusableAgentFailureItem,
) int {
	maxReusableAgentLen := 0
	for i := range successful {
		if len(successful[i].ReusableAgent.Name) > maxReusableAgentLen {
			maxReusableAgentLen = len(successful[i].ReusableAgent.Name)
		}
	}
	for i := range failed {
		if len(failed[i].ReusableAgent.Name) > maxReusableAgentLen {
			maxReusableAgentLen = len(failed[i].ReusableAgent.Name)
		}
	}
	return maxReusableAgentLen
}

func computeColumnWidths(successful []setup.SuccessItem, failed []setup.FailureItem) (int, int) {
	maxSkill, maxAgent := 0, 0
	for i := range successful {
		if len(successful[i].Skill.Name) > maxSkill {
			maxSkill = len(successful[i].Skill.Name)
		}
		if len(successful[i].Agent.DisplayName) > maxAgent {
			maxAgent = len(successful[i].Agent.DisplayName)
		}
	}
	for i := range failed {
		if len(failed[i].Skill.Name) > maxSkill {
			maxSkill = len(failed[i].Skill.Name)
		}
		if len(failed[i].Agent.DisplayName) > maxAgent {
			maxAgent = len(failed[i].Agent.DisplayName)
		}
	}
	return maxSkill, maxAgent
}

func shortDescription(desc string) string {
	if idx := strings.Index(desc, ". "); idx >= 0 {
		desc = desc[:idx+1]
	}
	const maxLen = 80
	runes := []rune(desc)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return desc
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// --- Form and utility functions ---

func confirmSetup() (bool, error) {
	confirmed := false
	field := huh.NewConfirm().
		Key("confirm").
		Title("Proceed with installation?").
		Value(&confirmed)
	if err := runPromptField(field); err != nil {
		return false, fmt.Errorf("confirm installation: %w", err)
	}
	return confirmed, nil
}

func runPromptField(field huh.Field) error {
	return huh.NewForm(huh.NewGroup(field)).WithTheme(darkHuhTheme()).Run()
}

func skillNames(skills []setup.Skill) []string {
	names := make([]string, 0, len(skills))
	for i := range skills {
		names = append(names, skills[i].Name)
	}
	return names
}

func agentNames(agents []setup.Agent) []string {
	names := make([]string, 0, len(agents))
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	return names
}

func previewsToSkills(previews []setup.PreviewItem) []setup.Skill {
	if len(previews) == 0 {
		return nil
	}

	skills := make([]setup.Skill, 0, len(previews))
	seen := make(map[string]struct{}, len(previews))
	for i := range previews {
		preview := &previews[i]
		key := strings.Join([]string{
			preview.Skill.Name,
			string(preview.Skill.Origin),
			preview.Skill.ExtensionSource,
			preview.Skill.ExtensionName,
			preview.Skill.ManifestPath,
			preview.Skill.ResolvedPath,
		}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		skills = append(skills, preview.Skill)
	}
	return skills
}

func formatSetupCatalogConflict(conflict setup.CatalogConflict) string {
	subject := string(conflict.Kind)
	switch conflict.Resolution {
	case setup.CatalogConflictCoreWins:
		return fmt.Sprintf(
			`ignored extension %s %q from %s because the core %s wins`,
			subject,
			conflict.Name,
			setupAssetSourceLabel(
				conflict.Ignored.Origin,
				conflict.Ignored.ExtensionSource,
				conflict.Ignored.ExtensionName,
			),
			subject,
		)
	case setup.CatalogConflictExtensionPrecedence:
		return fmt.Sprintf(
			`ignored extension %s %q from %s because %s wins by precedence`,
			subject,
			conflict.Name,
			setupAssetSourceLabel(
				conflict.Ignored.Origin,
				conflict.Ignored.ExtensionSource,
				conflict.Ignored.ExtensionName,
			),
			setupAssetSourceLabel(
				conflict.Winner.Origin,
				conflict.Winner.ExtensionSource,
				conflict.Winner.ExtensionName,
			),
		)
	default:
		return fmt.Sprintf(`ignored conflicting %s %q`, subject, conflict.Name)
	}
}

func setupAssetSourceLabel(origin setup.AssetOrigin, extensionSource string, extensionName string) string {
	if origin == setup.AssetOriginBundled {
		return "core"
	}

	parts := make([]string, 0, 2)
	if trimmedSource := strings.TrimSpace(extensionSource); trimmedSource != "" {
		parts = append(parts, trimmedSource)
	}
	if trimmedName := strings.TrimSpace(extensionName); trimmedName != "" {
		parts = append(parts, trimmedName)
	}
	if len(parts) == 0 {
		return "extension"
	}
	return strings.Join(parts, ":")
}

func defaultAgentSelection(supported []setup.Agent, detected []setup.Agent) []string {
	if len(detected) > 0 {
		return agentNames(detected)
	}

	defaults := []string{"codex", "claude-code", "cursor", "droid"}
	selected := make([]string, 0, len(defaults))
	for _, name := range defaults {
		for _, agent := range supported {
			if agent.Name == name {
				selected = append(selected, name)
				break
			}
		}
	}
	if len(selected) > 0 {
		return selected
	}
	return nil
}

func scopeLabel(global bool) string {
	if global {
		return string(setup.InstallScopeGlobal)
	}
	return string(setup.InstallScopeProject)
}

func displayRoots() (string, string) {
	var cwd string
	if value, err := os.Getwd(); err == nil {
		cwd = value
	}

	var homeDir string
	if value, err := os.UserHomeDir(); err == nil {
		homeDir = value
	}
	return cwd, homeDir
}

func shortenPath(fullPath, cwd, homeDir string) string {
	if homeDir != "" && (fullPath == homeDir || strings.HasPrefix(fullPath, homeDir+string(os.PathSeparator))) {
		return "~" + strings.TrimPrefix(fullPath, homeDir)
	}
	if cwd != "" && (fullPath == cwd || strings.HasPrefix(fullPath, cwd+string(os.PathSeparator))) {
		return "." + strings.TrimPrefix(fullPath, cwd)
	}
	return filepath.Clean(fullPath)
}

func sameInstallPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func isInteractiveTerminal() bool {
	stdin, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdout, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return stdin.Mode()&os.ModeCharDevice != 0 && stdout.Mode()&os.ModeCharDevice != 0
}
