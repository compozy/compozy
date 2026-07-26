package daemon

import (
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
)

func applyLoopRunPinningForTest(t *testing.T, run *looppkg.Run, at time.Time) {
	t.Helper()
	definition, err := loopdsl.Parse([]byte(
		`{"apiVersion":"agh.loop/v1","kind":"Loop","meta":{"name":"` + run.LoopName + `","version":1},"contract":{"goal":"test","definition_of_done":"done","iteration_cap":1,"no_progress":{"window":1},"budget":{"tokens":1,"wall_clock_sec":1}},"graph":{"nodes":[{"id":"finish","class":"action","kind":"transform","params":{"map":{"ok":{"value":true}}}}],"edges":[]}}`,
	))
	if err != nil {
		t.Fatalf("Parse(test Loop pin) error = %v", err)
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compile(test Loop pin) error = %v", err)
	}
	applyResolvedLoopRunPinningForTest(t, run, at, resolved)
}

func applyResolvedLoopRunPinningForTest(
	t *testing.T,
	run *looppkg.Run,
	at time.Time,
	resolved *looppkg.ResolvedDefinition,
) {
	t.Helper()
	if resolved == nil {
		t.Fatal("resolved Loop definition is required")
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	run.StartedAt = at
	run.DefinitionVersion = resolved.DefinitionVersion
	run.DefinitionDigest = digest
	run.DefinitionSnapshot = append([]byte(nil), snapshot...)
	run.ActiveHumanCriteria = []byte(`[]`)
	if run.StartMetadata == nil {
		run.StartMetadata = map[string]any{}
	}
}
