package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/compozy/looper/internal/setup"
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

	listSkills    func() ([]setup.Skill, error)
	listAgents    func(setup.ResolverOptions) ([]setup.Agent, error)
	detectAgents  func(setup.ResolverOptions) ([]setup.Agent, error)
	preview       func(setup.InstallConfig) ([]setup.PreviewItem, error)
	install       func(setup.InstallConfig) (*setup.Result, error)
	isInteractive func() bool
}

func newSetupCommand() *cobra.Command {
	state := newSetupCommandState()
	cmd := &cobra.Command{
		Use:          "setup",
		Short:        "Install Looper bundled public skills for supported agents",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Install Looper's bundled public skills without relying on an external skills installer.

The command can run interactively or entirely from flags.`,
		Example: `  looper setup
  looper setup --list
  looper setup --agent codex --agent claude --skill create-prd --skill create-techspec --yes
  looper setup --all
  looper setup --agent cursor --global --copy --yes`,
		RunE: state.run,
	}

	cmd.Flags().StringSliceVarP(&state.agentNames, "agent", "a", nil, "Target agent/editor name (repeatable)")
	cmd.Flags().StringSliceVarP(&state.skillNames, "skill", "s", nil, "Bundled skill name to install (repeatable)")
	cmd.Flags().BoolVarP(&state.global, "global", "g", false, "Install to the user directory instead of the project")
	cmd.Flags().BoolVar(&state.copy, "copy", false, "Copy files instead of symlinking to agent directories")
	cmd.Flags().BoolVarP(&state.list, "list", "l", false, "List bundled public skills without installing")
	cmd.Flags().BoolVarP(&state.yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().
		BoolVar(&state.all, "all", false, "Install all bundled public skills to all supported agents without prompts")
	return cmd
}

func newSetupCommandState() *setupCommandState {
	return &setupCommandState{
		listSkills:    setup.ListBundledSkills,
		listAgents:    setup.SupportedAgents,
		detectAgents:  setup.DetectInstalledAgents,
		preview:       setup.PreviewBundledSkillInstall,
		install:       setup.InstallBundledSkills,
		isInteractive: isInteractiveTerminal,
	}
}

func (s *setupCommandState) run(cmd *cobra.Command, _ []string) error {
	skills, err := s.listSkills()
	if err != nil {
		return err
	}
	if s.list {
		printBundledSkills(cmd, skills)
		return nil
	}

	if err := s.prepareRunMode(); err != nil {
		return err
	}

	resolver := s.resolverOptions()
	supportedAgents, detectedAgents, err := s.loadAgents(resolver)
	if err != nil {
		return err
	}

	cfg, previews, err := s.buildInstallPlan(cmd, skills, resolver, supportedAgents, detectedAgents)
	if err != nil {
		return err
	}
	if err := s.confirmPlan(cmd, previews, cfg.Global, cfg.Mode); err != nil {
		return err
	}

	return s.executeInstall(cmd, cfg)
}

func (s *setupCommandState) prepareRunMode() error {
	if s.all {
		s.yes = true
	}
	if !s.yes && !s.isInteractive() {
		return errors.New("looper setup requires an interactive terminal unless --yes is provided")
	}
	return nil
}

func (s *setupCommandState) resolverOptions() setup.ResolverOptions {
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
	skills []setup.Skill,
	resolver setup.ResolverOptions,
	supportedAgents []setup.Agent,
	detectedAgents []setup.Agent,
) (setup.InstallConfig, []setup.PreviewItem, error) {
	selectedSkills, err := s.resolveSkillSelection(skills)
	if err != nil {
		return setup.InstallConfig{}, nil, err
	}

	selectedAgents, err := s.resolveAgentSelection(supportedAgents, detectedAgents)
	if err != nil {
		return setup.InstallConfig{}, nil, err
	}

	globalScope, err := s.resolveScope(cmd, selectedAgents)
	if err != nil {
		return setup.InstallConfig{}, nil, err
	}

	mode, err := s.resolveInstallMode(cmd, supportedAgents, selectedAgents, globalScope)
	if err != nil {
		return setup.InstallConfig{}, nil, err
	}

	cfg := setup.InstallConfig{
		ResolverOptions: resolver,
		SkillNames:      selectedSkills,
		AgentNames:      selectedAgents,
		Global:          globalScope,
		Mode:            mode,
	}
	previews, err := s.preview(cfg)
	if err != nil {
		return setup.InstallConfig{}, nil, err
	}
	return cfg, previews, nil
}

func (s *setupCommandState) confirmPlan(
	cmd *cobra.Command,
	previews []setup.PreviewItem,
	global bool,
	mode setup.InstallMode,
) error {
	if s.yes {
		return nil
	}

	printPreviewSummary(cmd, previews, global, mode)
	confirmed, err := confirmSetup()
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("setup canceled")
	}
	return nil
}

func (s *setupCommandState) executeInstall(cmd *cobra.Command, cfg setup.InstallConfig) error {
	result, err := s.install(cfg)
	if err != nil {
		return err
	}

	printInstallResult(cmd, result)
	if len(result.Failed) > 0 {
		return fmt.Errorf("setup completed with %d failure(s)", len(result.Failed))
	}
	return nil
}

func (s *setupCommandState) resolveSkillSelection(skills []setup.Skill) ([]string, error) {
	if len(s.skillNames) > 0 {
		return append([]string(nil), s.skillNames...), nil
	}
	if s.all || s.yes {
		return skillNames(skills), nil
	}

	selected := skillNames(skills)
	options := make([]huh.Option[string], 0, len(skills))
	for _, skill := range skills {
		label := fmt.Sprintf("%s: %s", skill.Name, skill.Description)
		options = append(options, huh.NewOption(label, skill.Name))
	}

	field := huh.NewMultiSelect[string]().
		Key("skills").
		Title("Bundled Skills").
		Description("Select the public Looper skills to install").
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
		return nil, fmt.Errorf("select bundled skills: %w", err)
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
		Description("Select the editors/agents where Looper should install skills").
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

func printBundledSkills(cmd *cobra.Command, skills []setup.Skill) {
	for _, skill := range skills {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", skill.Name, skill.Description)
	}
}

func printPreviewSummary(
	cmd *cobra.Command,
	previews []setup.PreviewItem,
	global bool,
	mode setup.InstallMode,
) {
	if len(previews) == 0 {
		return
	}

	cwd, homeDir := displayRoots()

	fmt.Fprintln(cmd.OutOrStdout(), "Installation Summary")
	fmt.Fprintf(cmd.OutOrStdout(), "  Scope: %s\n", scopeLabel(global))
	fmt.Fprintf(cmd.OutOrStdout(), "  Method: %s\n", mode)
	for i := range previews {
		preview := &previews[i]
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"  - %s -> %s (%s)",
			preview.Skill.Name,
			preview.Agent.DisplayName,
			shortenPath(preview.TargetPath, cwd, homeDir),
		)
		if mode == setup.InstallModeSymlink && !sameInstallPath(preview.CanonicalPath, preview.TargetPath) {
			fmt.Fprintf(cmd.OutOrStdout(), " via %s", shortenPath(preview.CanonicalPath, cwd, homeDir))
		}
		if preview.WillOverwrite {
			fmt.Fprint(cmd.OutOrStdout(), " [overwrite]")
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
}

func printInstallResult(cmd *cobra.Command, result *setup.Result) {
	if result == nil {
		return
	}

	cwd, homeDir := displayRoots()

	if len(result.Successful) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Installed")
		for i := range result.Successful {
			item := &result.Successful[i]
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"  - %s -> %s (%s)",
				item.Skill.Name,
				item.Agent.DisplayName,
				shortenPath(item.Path, cwd, homeDir),
			)
			if item.Mode == setup.InstallModeSymlink && item.SymlinkFailed {
				fmt.Fprint(cmd.OutOrStdout(), " [copied after symlink failure]")
			}
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}

	if len(result.Failed) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Failed")
		for i := range result.Failed {
			item := &result.Failed[i]
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"  - %s -> %s (%s): %s\n",
				item.Skill.Name,
				item.Agent.DisplayName,
				shortenPath(item.Path, cwd, homeDir),
				item.Error,
			)
		}
	}
}

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
	return huh.NewForm(huh.NewGroup(field)).WithTheme(huh.ThemeCharm()).Run()
}

func skillNames(skills []setup.Skill) []string {
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
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
		return "global"
	}
	return "project"
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
