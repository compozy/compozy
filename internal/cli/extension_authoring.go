package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
)

func newExtensionInitCommand() *cobra.Command {
	var template string
	var directory string
	command := &cobra.Command{
		Use:   "init <name>",
		Short: "Create an extension project from an embedded template",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := extensionpkg.ScaffoldExtension(extensionpkg.ScaffoldRequest{
				Name:      args[0],
				Template:  extensionpkg.ScaffoldTemplate(template),
				Directory: directory,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionScaffoldBundle(result))
		},
	}
	command.Flags().StringVarP(
		&template,
		"template",
		"t",
		string(extensionpkg.ScaffoldTemplateToolProviderTS),
		extensionScaffoldTemplateHelp(),
	)
	command.Flags().StringVarP(&directory, "dir", "d", "", "Target directory (default: ./<name>)")
	return command
}

func extensionScaffoldTemplateHelp() string {
	templates := extensionpkg.ScaffoldTemplates()
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		names = append(names, string(template))
	}
	return "Template: " + strings.Join(names, ", ")
}

func newExtensionBuildCommand(deps commandDeps) *cobra.Command {
	var outputDir string
	var buildCommand []string
	var timeout time.Duration
	command := &cobra.Command{
		Use:   "build [directory]",
		Short: "Build source into an immutable extension bundle",
		Args:  optionalDirectoryArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceDir, err := extensionAuthoringDirectory(deps, args)
			if err != nil {
				return err
			}
			result, err := extensionpkg.BuildBundle(cmd.Context(), extensionpkg.BuildRequest{
				SourceDir: sourceDir,
				OutputDir: outputDir,
				BuildCmd:  buildCommand,
				Timeout:   timeout,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBuildBundle(result))
		},
	}
	command.Flags().StringVar(&outputDir, "output-dir", "", "Generation root (default: <source>/dist)")
	command.Flags().StringSliceVar(
		&buildCommand,
		"build-command",
		nil,
		"Build command argv; repeat or use a comma-separated value",
	)
	command.Flags().DurationVar(&timeout, cliTimeoutKey, 60*time.Second, "Describe-mode timeout")
	return command
}

func newExtensionValidateCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [directory]",
		Short: "Validate an extension bundle without running its code",
		Args:  optionalDirectoryArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			directory, err := extensionAuthoringDirectory(deps, args)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				directory, err = resolveLocalValidationDirectory(directory)
				if err != nil {
					return err
				}
			}
			report, err := extensionpkg.ValidateBundleReport(directory)
			if err != nil {
				return err
			}
			if err := writeCommandOutput(cmd, extensionValidationBundle(report)); err != nil {
				return err
			}
			if validationHasErrors(report.Issues) {
				return errors.New("cli: extension bundle validation failed")
			}
			return nil
		},
	}
}

func resolveLocalValidationDirectory(sourceDir string) (string, error) {
	for _, name := range []string{"extension.toml", "extension.json"} {
		if info, err := os.Stat(filepath.Join(sourceDir, name)); err == nil && info.Mode().IsRegular() {
			return sourceDir, nil
		}
	}
	entries, err := os.ReadDir(filepath.Join(sourceDir, "dist"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sourceDir, nil
		}
		return "", fmt.Errorf("cli: read extension generations: %w", err)
	}
	generations := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "gen-") {
			generations = append(generations, filepath.Join(sourceDir, "dist", entry.Name()))
		}
	}
	slices.Sort(generations)
	switch len(generations) {
	case 0:
		return sourceDir, nil
	case 1:
		return generations[0], nil
	default:
		return "", errors.New("cli: multiple bundle generations found; pass the generation directory to validate")
	}
}

func optionalDirectoryArg(_ *cobra.Command, args []string) error {
	if len(args) > 1 {
		return errors.New("cli: accepts at most one directory")
	}
	if len(args) == 1 && strings.TrimSpace(args[0]) == "" {
		return errors.New("cli: directory must not be blank")
	}
	return nil
}

func extensionAuthoringDirectory(deps commandDeps, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	directory, err := deps.getwd()
	if err != nil {
		return "", fmt.Errorf("cli: resolve current directory: %w", err)
	}
	return directory, nil
}

func extensionScaffoldBundle(result *extensionpkg.ScaffoldResult) outputBundle {
	return singleExtensionAuthoringBundle(
		result,
		"Extension project created",
		[]keyValue{
			{Label: automationNameValue, Value: result.Name},
			{Label: "Template", Value: string(result.Template)},
			{Label: cliDirectoryValue, Value: result.Directory},
			{Label: "Files", Value: strconv.Itoa(len(result.Files))},
		},
		"extension_init",
		[]string{automationNameKey, "template", "directory", "files"},
		[]string{result.Name, string(result.Template), result.Directory, strconv.Itoa(len(result.Files))},
	)
}

func extensionBuildBundle(result *extensionpkg.BuildResult) outputBundle {
	return singleExtensionAuthoringBundle(
		result,
		"Extension bundle built",
		[]keyValue{
			{Label: automationNameValue, Value: result.Manifest.Name},
			{Label: cliGenerationValue, Value: result.GenerationHash},
			{Label: cliDirectoryValue, Value: result.GenerationDir},
			{Label: "Manifest", Value: result.ManifestPath},
		},
		"extension_build",
		[]string{automationNameKey, "generation_hash", "generation_dir", "manifest_path"},
		[]string{
			result.Manifest.Name,
			result.GenerationHash,
			result.GenerationDir,
			result.ManifestPath,
		},
	)
}

func extensionValidationBundle(report *extensionpkg.ValidationReport) outputBundle {
	if strings.TrimSpace(report.Format) == string(extensionpkg.FormatAgentPlugin) {
		return outputBundle{
			jsonValue: report,
			jsonl: func(cmd *cobra.Command) error {
				return writeJSONLine(cmd, report)
			},
			human: func() (string, error) {
				return extensionPortableValidationHuman(report), nil
			},
			toon: func() (string, error) {
				return extensionPortableValidationToon(report)
			},
		}
	}
	status := configValidKey
	if validationHasErrors(report.Issues) {
		status = configInvalidKey
	}
	consent := make([]string, 0, len(report.ConsentAreas))
	for _, area := range report.ConsentAreas {
		consent = append(consent, area.Area+":"+area.Access)
	}
	rows := []keyValue{
		{Label: automationStatusValue, Value: status},
		{Label: cliIssuesValue, Value: strconv.Itoa(len(report.Issues))},
		{Label: "Consent", Value: strings.Join(consent, ", ")},
	}
	for _, issue := range report.Issues {
		location := issue.Path
		if issue.Line > 0 {
			location += ":" + strconv.Itoa(issue.Line)
			if issue.Column > 0 {
				location += ":" + strconv.Itoa(issue.Column)
			}
		}
		rows = append(rows, keyValue{
			Label: strings.ToUpper(string(issue.Severity)) + " " + location,
			Value: issue.Message,
		})
	}
	return singleExtensionAuthoringBundle(
		report,
		"Extension bundle validation",
		rows,
		"extension_validate",
		[]string{automationStatusKey, cliIssuesKey, "consent_areas"},
		[]string{status, strconv.Itoa(len(report.Issues)), strings.Join(consent, ",")},
	)
}

func extensionPortableValidationHuman(report *extensionpkg.ValidationReport) string {
	packageName := strings.TrimSpace(report.Name + " " + report.Version)
	rows := []keyValue{
		{Label: automationStatusValue, Value: stringOrDash(report.Status)},
		{Label: cliFormatValue, Value: extensionFormatLabel(report.Format)},
	}
	if packageName != "" {
		rows = append(rows, keyValue{Label: "Package", Value: packageName})
	}
	rows = append(rows, keyValue{Label: cliIssuesValue, Value: strconv.Itoa(len(report.Issues))})
	blocks := []string{renderHumanSection("Extension bundle validation", rows)}
	if len(report.WouldIngest) > 0 {
		rows := make([][]string, 0, len(report.WouldIngest))
		for _, item := range report.WouldIngest {
			rows = append(rows, []string{item.Kind, item.Name, item.Transport, item.Detail})
		}
		blocks = append(
			blocks,
			renderHumanTable("Would ingest", []string{cliKindValue, cliNameValue, "Transport", cliDetailValue}, rows),
		)
	}
	for _, issue := range report.Issues {
		scope := strings.TrimSpace(issue.Scope)
		if scope == "" {
			scope = strings.TrimSpace(issue.Path)
		}
		blocks = append(blocks, fmt.Sprintf(
			"%s %s:  %s",
			strings.ToUpper(string(issue.Severity)),
			scope,
			strings.TrimSpace(issue.Message),
		))
	}
	return renderHumanBlocks(blocks...)
}

func singleExtensionAuthoringBundle(
	value any,
	title string,
	rows []keyValue,
	toonName string,
	toonFields []string,
	toonValues []string,
) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, value)
		},
		human: func() (string, error) {
			return renderHumanSectionResult(title, rows)
		},
		toon: func() (string, error) {
			return renderToonObject(toonName, toonFields, toonValues), nil
		},
	}
}

func validationHasErrors(issues []extensionpkg.ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == extensionpkg.IssueSeverityError {
			return true
		}
	}
	return false
}
