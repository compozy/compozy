package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	watchpkg "github.com/compozy/compozy/internal/loop/watch"
)

// ListParkedLoopWaitEventSubscriptions rebuilds durable event-wait subscriptions after boot.
func (g *LoopRepo) ListParkedLoopWaitEventSubscriptions(
	ctx context.Context,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	return g.listParkedLoopWaitEventSubscriptions(ctx, "")
}

// ListParkedLoopWaitEventSubscriptionsForLoopRun refreshes one run in the observer index.
func (g *LoopRepo) ListParkedLoopWaitEventSubscriptionsForLoopRun(
	ctx context.Context,
	runID string,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	return g.listParkedLoopWaitEventSubscriptions(ctx, strings.TrimSpace(runID))
}

func (g *LoopRepo) listParkedLoopWaitEventSubscriptions(
	ctx context.Context,
	runID string,
) (returnSubscriptions []looppkg.ParkedWatchEventSubscription, returnErr error) {
	if err := g.checkReady(ctx, "list parked Loop event waits"); err != nil {
		return nil, err
	}
	query := `SELECT run.workspace_id, run.id, run.loop_name, run.inputs_json,
		run.definition_digest, snapshot.definition_json, wait.generation, wait.node_id,
		wait.item_index, wait.issued_epoch, wait.ahead_payload_json
		FROM loop_node_waits AS wait
		JOIN loop_runs AS run ON run.id = wait.loop_run_id
		JOIN loop_definition_snapshots AS snapshot ON snapshot.workspace_id = run.workspace_id
		 AND snapshot.definition_digest = run.definition_digest
		JOIN loop_generation_outputs AS output ON output.loop_run_id = wait.loop_run_id
		 AND output.generation = wait.generation AND output.node_id = wait.node_id
		 AND output.item_index = wait.item_index
		LEFT JOIN loop_node_controls AS control ON control.loop_run_id = wait.loop_run_id
		 AND control.node_id = wait.node_id
		WHERE wait.kind = 'event' AND wait.claim_state = 'waiting' AND output.status = 'waiting'
		 AND output.epoch = wait.issued_epoch
		 AND run.status NOT IN ('paused','done','no-op','blocked','failed','exhausted','stalled','canceled')
		 AND COALESCE(control.paused, 0) = 0 AND COALESCE(control.quarantined, 0) = 0
		 AND COALESCE(control.cancel_state, '') = ''`
	args := []any{}
	if runID != "" {
		query += " AND run.id = ?"
		args = append(args, runID)
	}
	query += " ORDER BY run.workspace_id, run.id, wait.generation, wait.node_id, wait.item_index"
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query parked Loop event waits: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("store: close parked Loop event waits: %w", closeErr))
		}
	}()
	subscriptions := make([]looppkg.ParkedWatchEventSubscription, 0)
	for rows.Next() {
		subscription, scanErr := scanParkedLoopWaitEventSubscription(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate parked Loop event waits: %w", err)
	}
	return subscriptions, nil
}

func scanParkedLoopWaitEventSubscription(row rowScanner) (looppkg.ParkedWatchEventSubscription, error) {
	var workspaceID, loopRunID, loopName, inputsRaw, digest, snapshotRaw, nodeID string
	var aheadPayload sql.NullString
	var generation, itemIndex int
	var issuedEpoch int64
	if err := row.Scan(&workspaceID, &loopRunID, &loopName, &inputsRaw, &digest, &snapshotRaw,
		&generation, &nodeID, &itemIndex, &issuedEpoch, &aheadPayload); err != nil {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf("store: scan parked Loop event wait: %w", err)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(json.RawMessage(snapshotRaw), digest)
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf(
			"store: hydrate parked Loop event wait definition: %w", err,
		)
	}
	node, found := loopWaitNodeByRuntimeID(resolved.Definition.Graph, "", nodeID)
	if !found {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf(
			"%w: parked Loop event wait node %q is absent", looppkg.ErrValidation, nodeID,
		)
	}
	var params dsl.WaitParams
	if err := node.Params.Decode(&params); err != nil || params.Event == nil {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf(
			"%w: parked Loop event wait node %q has invalid params", looppkg.ErrValidation, nodeID,
		)
	}
	inputs, err := decodeParkedWatchEventsInputs(inputsRaw)
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, err
	}
	lifecycle, err := looppkg.ResolveNodeLifecycleConfig(node, nil, resolved.EffectiveConfig.Lifecycle)
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf(
			"store: resolve parked Loop event wait lifecycle: %w", err,
		)
	}
	cursors, _, err := looppkg.DecodeWaitAheadRejectCursors(json.RawMessage(aheadPayload.String))
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf(
			"store: decode parked Loop event wait ahead fence: %w", err,
		)
	}
	return looppkg.ParkedWatchEventSubscription{
		WorkspaceID: workspaceID, LoopRunID: loopRunID, LoopName: loopName,
		NodeID: nodeID, Generation: generation, Inputs: inputs,
		Subscriptions: []watchpkg.EventSubscriptionRef{{
			Kind: strings.TrimSpace(params.Event.Kind), Filter: strings.TrimSpace(params.Event.Filter),
		}},
		Cursors: cursors, Contracts: resolved.WatchEventsContracts,
		Wait: &looppkg.ParkedNodeWaitEvent{
			ItemIndex: itemIndex, IssuedEpoch: issuedEpoch,
			AdmissionAttempts: lifecycle.WaitAdmissionAttempts,
		},
	}, nil
}

func loopWaitNodeByRuntimeID(graph dsl.Graph, prefix string, nodeID string) (dsl.Node, bool) {
	for _, node := range graph.Nodes {
		runtimeID := string(node.ID)
		if prefix != "" {
			runtimeID = prefix + "__" + runtimeID
		}
		if runtimeID == nodeID {
			return node, true
		}
		if node.Body != nil {
			if nested, found := loopWaitNodeByRuntimeID(*node.Body, runtimeID, nodeID); found {
				return nested, true
			}
		}
	}
	return dsl.Node{}, false
}
