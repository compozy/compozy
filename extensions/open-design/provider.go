package opendesign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	compozysdk "github.com/compozy/compozy/sdk/go"
	"github.com/compozy/compozy/sdk/go/contracts"
)

const LintToolID = "ext__open_design__lint_artifact"

func newProvider(options ...compozysdk.Option) (*compozysdk.Extension, error) {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
		Name:    Name,
		Version: "0.1.0",
		Description: "Design interfaces, sites, slides, and visual documents as workspace HTML " +
			"with curated design guidance and OpenDesign-derived lints.",
		Profiles: []compozysdk.DescribeProfile{
			{Name: Name, Icon: "palette", Defaults: compozysdk.DescribeProfileDefaults{Agent: "open-design-designer"}},
		},
		Resources: compozysdk.DescribeResources{
			Skills: []compozysdk.DescribeResourcePath{
				{Path: "skills/open-design/SKILL.md", Profile: Name},
				{Path: "skills/open-design-review/SKILL.md", Profile: Name},
				{Path: "skills/agent-browser/SKILL.md", Profile: Name},
			},
			Agents: []compozysdk.DescribeResourcePath{{Path: "agents", Profile: Name}},
			Loops:  []compozysdk.DescribeResourcePath{{Path: "loops", Profile: Name}},
		},
		Subprocess: compozysdk.DescribeSubprocess{
			Command: "{{compozy_executable}}",
			Args:    []string{"__internal", "extension-provider", Name},
		},
	}, options...)
	if err := compozysdk.Tool(extension, "lint_artifact", compozysdk.ToolOptions{
		Profile: Name, ID: compozysdk.ToolID(LintToolID),
		Description: "Run the original OpenDesign heuristic design linter on HTML files under the " +
			"trusted workspace's docs/design directory. Returns original P0/P1/P2 findings, fixes, " +
			"file digests, and feedback. Requires Node.js. No files are changed.",
		InputSchema: json.RawMessage(lintInputSchema), OutputSchema: json.RawMessage(lintOutputSchema),
		ReadOnly: true, Risk: compozysdk.RiskRead,
	}, handleLint); err != nil {
		return nil, fmt.Errorf("open-design: register lint tool: %w", err)
	}
	return extension, nil
}

func Describe() (contracts.DescribePayload, error) {
	extension, err := newProvider()
	if err != nil {
		return contracts.DescribePayload{}, err
	}
	return extension.Describe()
}

func RunProvider(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	if ctx == nil || stdin == nil || stdout == nil || stderr == nil {
		return errors.New("open-design: context and stdio are required")
	}
	extension, err := newProvider(compozysdk.WithStdio(stdin, stdout), compozysdk.WithStderr(stderr))
	if err != nil {
		return err
	}
	if err := extension.Run(ctx); err != nil {
		return fmt.Errorf("open-design: run provider: %w", err)
	}
	return nil
}
