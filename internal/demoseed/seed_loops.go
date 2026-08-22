package demoseed

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store/globaldb"
)

//go:embed loops/*/loop.yaml
var loopDefinitions embed.FS

// loopPlacement assigns each authored Loop to the workspace that owns it and to
// the canvas positions the editor opens with.
type loopPlacement struct {
	name         string
	workspaceKey string
	positions    map[string][2]float64
}

func scenarioLoopPlacements() []loopPlacement {
	return []loopPlacement{
		{name: loopLaunchReadiness, workspaceKey: workspaceKeyLaunch, positions: map[string][2]float64{
			nodeMarketInput: {0, 0}, "assess": {260, 0}, "has_blockers": {520, 0}, "decision_gate": {780, 0},
		}},
		{name: loopLaunchReadiness, workspaceKey: workspaceKeyPlatform, positions: map[string][2]float64{
			nodeMarketInput: {0, 0}, "assess": {260, 0}, "has_blockers": {520, 0}, "decision_gate": {780, 0},
		}},
		{name: loopMarketRollout, workspaceKey: workspaceKeyLaunch, positions: map[string][2]float64{
			"markets_input": {0, 0}, "split_markets": {240, 0}, "review_fanout": {480, 0},
			"review_market": {720, 0}, "collect_reviews": {960, 0}, "promotion_gate": {1200, 0},
		}},
		{name: loopChargebackTriage, workspaceKey: workspaceKeyLaunch, positions: map[string][2]float64{
			"settlement_batch": {0, 0}, "score_batch": {260, 0}, "route_batch": {520, 0},
			"record_clean": {780, -80}, "record_escalated": {780, 80},
		}},
		{name: loopIncidentPostmort, workspaceKey: workspaceKeyLaunch, positions: map[string][2]float64{
			"incident_input": {0, 0}, "write_postmortem": {260, 0}, "confirm_publication": {520, 0},
		}},
		{name: loopReleaseTrain, workspaceKey: workspaceKeyPlatform, positions: map[string][2]float64{
			nodeMarketInput: {0, 0}, "readiness": {260, 0}, "settlement_audit": {520, 0},
		}},
		{name: loopDisputeSweep, workspaceKey: workspaceKeyPlatform, positions: map[string][2]float64{
			"dispute_files": {0, 0}, "bundle_fanout": {240, 0}, "build_bundle": {480, 0},
			"collect_bundles": {720, 0},
		}},
		{name: loopSettlementAudit, workspaceKey: workspaceKeyPlatform, positions: map[string][2]float64{
			"partner_input": {0, 0}, "audit": {260, 0}, "operator_signoff": {520, 0},
		}},
		{name: loopDocsFreshness, workspaceKey: workspaceKeyPlatform, positions: map[string][2]float64{
			"area_input": {0, 0}, "flag_stale": {260, 0},
		}},
	}
}

func scenarioLoopNames() []string {
	placements := scenarioLoopPlacements()
	names := make([]string, 0, len(placements))
	seen := make(map[string]struct{}, len(placements))
	for _, placement := range placements {
		if _, duplicate := seen[placement.name]; duplicate {
			continue
		}
		seen[placement.name] = struct{}{}
		names = append(names, placement.name)
	}
	return names
}

func loopDefinitionBytes(name string) ([]byte, error) {
	data, err := loopDefinitions.ReadFile(filepath.ToSlash(filepath.Join("loops", name, "loop.yaml")))
	if err != nil {
		return nil, fmt.Errorf("demo seed: read embedded Loop %q: %w", name, err)
	}
	return data, nil
}

func seedLoopDefinitions(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (int, error) {
	for _, placement := range scenarioLoopPlacements() {
		record, err := state.recordFor(placement.workspaceKey)
		if err != nil {
			return 0, err
		}
		data, err := loopDefinitionBytes(placement.name)
		if err != nil {
			return 0, err
		}
		loopsRoot := filepath.Join(record.RootDir, config.DirName, config.LoopsDirName)
		if _, _, err := looppkg.WriteDefinition(loopsRoot, data, looppkg.WriteDefinitionOptions{
			Source: looppkg.SourceUser, Overwrite: state.replace,
		}); err != nil {
			return 0, fmt.Errorf("demo seed: write Loop %q: %w", placement.name, err)
		}
		if err := writeLoopAnnotations(ctx, db, record.ID, placement); err != nil {
			return 0, err
		}
	}
	return len(scenarioLoopPlacements()), nil
}

func writeLoopAnnotations(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	placement loopPlacement,
) error {
	annotations := make([]looppkg.UIAnnotation, 0, len(placement.positions))
	for nodeID, position := range placement.positions {
		annotations = append(annotations, looppkg.UIAnnotation{
			NodeID: looppkg.NodeID(nodeID), X: position[0], Y: position[1],
		})
	}
	if err := db.ReplaceLoopUIAnnotations(
		ctx, looppkg.WorkspaceID(workspaceID), placement.name, annotations,
	); err != nil {
		return fmt.Errorf("demo seed: write Loop annotations for %q: %w", placement.name, err)
	}
	return nil
}

// compiledLoopSnapshot builds the pinned executed-definition artifact a run history import needs.
func compiledLoopSnapshot(name string) (json.RawMessage, string, int, error) {
	data, err := loopDefinitionBytes(name)
	if err != nil {
		return nil, "", 0, err
	}
	definition, err := dsl.Parse(data)
	if err != nil {
		return nil, "", 0, fmt.Errorf("demo seed: parse Loop %q: %w", name, err)
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		return nil, "", 0, fmt.Errorf("demo seed: compile Loop %q: %w", name, err)
	}
	effective, err := looppkg.ResolveEffectiveConfig(resolved, looppkg.LoopDefaults{}, nil, looppkg.LoopConfig{})
	if err != nil {
		return nil, "", 0, fmt.Errorf("demo seed: resolve config for Loop %q: %w", name, err)
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		return nil, "", 0, fmt.Errorf("demo seed: snapshot Loop %q: %w", name, err)
	}
	return snapshot, digest, resolved.DefinitionVersion, nil
}
