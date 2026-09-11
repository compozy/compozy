package opendesign

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/tools"
)

// Suite: embedded extension product contract. Real parsers and the Loop compiler
// must load the shipped catalog; this is not a snapshot of prose or layout.
func TestEmbeddedResources(t *testing.T) {
	t.Parallel()
	// Invariant: the shipped executable matches the locally maintained lint source.
	// Owner: embedded extension; reuse this distribution-contract suite.
	t.Run("Should build the shipped linter from local source", func(t *testing.T) {
		t.Parallel()
		output, err := exec.CommandContext(t.Context(), "bun", "scripts/build-lint.ts", "--check").CombinedOutput()
		if err != nil {
			t.Fatalf("embedded linter drift: %v\n%s", err, output)
		}
	})
	t.Run("Should load the declared profile resources and compile the review loop", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		if err := os.CopyFS(directory, FS()); err != nil {
			t.Fatal(err)
		}
		manifest, err := extensionpkg.LoadManifest(directory)
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Profiles) != 1 || manifest.Profiles[0].Name != Name ||
			manifest.Profiles[0].Defaults.Agent != "open-design-designer" {
			t.Fatalf("profile defaults = %#v", manifest.Profiles)
		}
		// Explicit entrypoints expose only the three supported skills.
		if len(manifest.Resources.Skills) != 3 {
			t.Fatalf("skill declarations = %#v", manifest.Resources.Skills)
		}
		for _, resource := range manifest.Resources.Skills {
			if resource.Profile != Name || filepath.Base(resource.Path) != "SKILL.md" {
				t.Fatalf("skill placement = %#v", resource)
			}
			skill, err := skills.ParseSkillFile(filepath.Join(directory, resource.Path))
			if err != nil || skill == nil || skill.Meta.Name == "" {
				t.Fatalf("skill %s = %#v, %v", resource.Path, skill, err)
			}
		}
		for _, name := range []string{"open-design-designer", "open-design-critic"} {
			data, err := fs.ReadFile(FS(), "agents/"+name+"/AGENT.md")
			if err != nil {
				t.Fatal(err)
			}
			agent, err := config.ParseAgentDef(data)
			if err != nil || agent.Name != name {
				t.Fatalf("agent %s = %#v, %v", name, agent, err)
			}
		}
		descriptors, err := extensionpkg.ResolveManifestToolDescriptors(manifest)
		if err != nil || len(descriptors) != 1 {
			t.Fatalf("tool descriptors = %#v, %v", descriptors, err)
		}
		descriptor := descriptors[0].Tool
		if descriptor.ID.String() != LintToolID || !descriptor.ReadOnly || descriptor.Risk != tools.RiskRead {
			t.Fatalf("lint descriptor = %#v", descriptor)
		}
		described, err := Describe()
		if err != nil || len(described.Tools) != 1 {
			t.Fatalf("describe = %#v, %v", described, err)
		}
		for _, pair := range [][2]json.RawMessage{
			{descriptor.InputSchema, described.Tools[0].InputSchema},
			{descriptor.OutputSchema, described.Tools[0].OutputSchema},
		} {
			left, err := tools.SchemaDigest(pair[0])
			if err != nil {
				t.Fatal(err)
			}
			right, err := tools.SchemaDigest(pair[1])
			if err != nil || left != right {
				t.Fatalf("manifest/provider schema drift: %s != %s, %v", left, right, err)
			}
		}
		source := lintSchemas{snapshot: loop.ToolSchemaSnapshot{
			ToolID: LintToolID, InputSchema: descriptor.InputSchema, OutputSchema: descriptor.OutputSchema,
			InputSchemaDigest: descriptor.InputSchemaDigest, OutputSchemaDigest: descriptor.OutputSchemaDigest,
		}}
		data, err := fs.ReadFile(FS(), "loops/open-design-review/loop.yaml")
		if err != nil {
			t.Fatal(err)
		}
		_, definition, err := loop.ParseResource(data, loop.ResourceParseOptions{
			Source: loop.SourceMarketplace, Dir: "loops/open-design-review",
			FilePath: "loops/open-design-review/loop.yaml",
			Linter:   loop.NewLinter(loop.WithToolSchemaSource(source)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loop.NewCompiler(loop.WithCompilerToolSchemaSource(source)).Compile(definition); err != nil {
			t.Fatal(err)
		}
	})
}

type lintSchemas struct{ snapshot loop.ToolSchemaSnapshot }

var _ loop.ToolSchemaSource = lintSchemas{}

func (s lintSchemas) Snapshot(id string) (loop.ToolSchemaSnapshot, bool) {
	return s.snapshot, id == LintToolID
}
