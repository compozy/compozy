package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

const codexACPNPMPackageName = "@zed-industries/codex-acp"

var absoluteCodexACPPathPattern = regexp.MustCompile(`/[^\s"']*/codex-acp(?:\.js)?`)

type runtimeModelRequirement struct {
	RuntimeCommand     string
	RuntimeDisplayName string
	PackageName        string
	MinVersion         string
	UpgradeCommand     string
	UnavailableReason  string
}

var codexModelRequirements = map[string]runtimeModelRequirement{
	"gpt-5.5": {
		RuntimeCommand:     "codex-acp",
		RuntimeDisplayName: "codex-acp",
		PackageName:        codexACPNPMPackageName,
		MinVersion:         "0.12.0",
		UpgradeCommand:     "npm install -g @zed-industries/codex-acp@latest",
	},
}

func validateRuntimeModelCompatibility(spec Spec, modelName string, command []string) error {
	resolvedModel := strings.TrimSpace(modelName)
	req, ok := runtimeModelRequirementFor(spec.ID, resolvedModel)
	if !ok {
		return nil
	}
	if strings.TrimSpace(req.UnavailableReason) != "" {
		return fmt.Errorf(
			"%s is not available for %s: %s. %s",
			resolvedModel,
			spec.DisplayName,
			req.UnavailableReason,
			supportedModelMessage(req),
		)
	}
	if len(command) == 0 || command[0] != req.RuntimeCommand {
		return nil
	}

	version, ok := detectCodexACPVersion(command[0])
	if !ok {
		return nil
	}
	if compareSemver(version, req.MinVersion) >= 0 {
		return nil
	}
	return fmt.Errorf(
		"%s requires %s >= %s, but found %s. Update with: %s. %s",
		resolvedModel,
		req.RuntimeDisplayName,
		req.MinVersion,
		version,
		req.UpgradeCommand,
		supportedModelMessage(req),
	)
}

func codexModelCompatibilityHint(spec Spec, modelName string, err error) error {
	resolvedModel := strings.TrimSpace(modelName)
	req, ok := runtimeModelRequirementFor(spec.ID, resolvedModel)
	if err == nil || !ok {
		return err
	}
	if !strings.Contains(
		err.Error(),
		fmt.Sprintf("The model `%s` does not exist or you do not have access to it", resolvedModel),
	) {
		return err
	}
	return fmt.Errorf(
		"%w. This can happen when %s is not compatible with %s. "+
			"Install %s >= %s with: %s. %s",
		err,
		req.RuntimeDisplayName,
		resolvedModel,
		req.PackageName,
		req.MinVersion,
		req.UpgradeCommand,
		supportedModelMessage(req),
	)
}

func runtimeModelRequirementFor(ide string, modelName string) (runtimeModelRequirement, bool) {
	if ide != model.IDECodex {
		return runtimeModelRequirement{}, false
	}
	req, ok := codexModelRequirements[strings.TrimSpace(modelName)]
	return req, ok
}

func supportedModelMessage(req runtimeModelRequirement) string {
	displayName := strings.TrimSpace(req.RuntimeDisplayName)
	if displayName == "" {
		displayName = "the selected runtime"
	}
	return fmt.Sprintf("Choose a model supported by your installed %s.", displayName)
}

func detectCodexACPVersion(command string) (string, bool) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", false
	}
	return detectCodexACPVersionFromPath(path)
}

func detectCodexACPVersionFromPath(path string) (string, bool) {
	if version, ok := codexACPVersionNearPath(path); ok {
		return version, true
	}
	target, ok := codexACPWrapperTarget(path)
	if !ok {
		return "", false
	}
	return codexACPVersionNearPath(target)
}

func codexACPVersionNearPath(path string) (string, bool) {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolvedPath = path
	}
	dir := filepath.Dir(resolvedPath)
	for i := 0; i < 8; i++ {
		version, ok := readNPMVersion(filepath.Join(dir, "package.json"), codexACPNPMPackageName)
		if ok {
			return version, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func codexACPWrapperTarget(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > 16*1024 {
		return "", false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := absoluteCodexACPPathPattern.Find(content)
	if len(match) == 0 {
		return "", false
	}
	return string(match), true
}

func readNPMVersion(path string, packageName string) (string, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return "", false
	}
	if pkg.Name != packageName || strings.TrimSpace(pkg.Version) == "" {
		return "", false
	}
	return strings.TrimSpace(pkg.Version), true
}

func compareSemver(left, right string) int {
	leftParts := semverParts(left)
	rightParts := semverParts(right)
	for i := range leftParts {
		if leftParts[i] < rightParts[i] {
			return -1
		}
		if leftParts[i] > rightParts[i] {
			return 1
		}
	}
	return 0
}

func semverParts(version string) [3]int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	version = strings.SplitN(version, "-", 2)[0]
	rawParts := strings.Split(version, ".")
	var parts [3]int
	for i := 0; i < len(parts) && i < len(rawParts); i++ {
		parsed, err := strconv.Atoi(rawParts[i])
		if err != nil {
			continue
		}
		parts[i] = parsed
	}
	return parts
}
