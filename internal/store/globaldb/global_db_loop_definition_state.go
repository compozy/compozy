package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// DeleteLoopDefinitionState atomically removes mutable sidecars owned by one Loop definition.
func (g *LoopRepo) DeleteLoopDefinitionState(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
) (looppkg.DefinitionState, error) {
	if err := g.checkReady(ctx, "delete loop definition state"); err != nil {
		return looppkg.DefinitionState{}, err
	}
	workspaceID, name, err := normalizeLoopAnnotationKey(ws, loopName)
	if err != nil {
		return looppkg.DefinitionState{}, err
	}
	var state looppkg.DefinitionState
	err = g.withImmediateTransaction(ctx, "delete loop definition state", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		row, getErr := queries.GetLoopConfig(ctx, sqlcgen.GetLoopConfigParams{
			WorkspaceID: string(workspaceID),
			LoopName:    name,
		})
		switch {
		case getErr == nil:
			config, decodeErr := loopConfigFromGenerated(row)
			if decodeErr != nil {
				return decodeErr
			}
			state.Config = &config
		case errors.Is(getErr, sql.ErrNoRows):
		case getErr != nil:
			return fmt.Errorf("store: get loop config before definition state delete: %w", getErr)
		}
		rows, listErr := queries.ListLoopUIAnnotations(ctx, sqlcgen.ListLoopUIAnnotationsParams{
			WorkspaceID: string(workspaceID),
			LoopName:    name,
		})
		if listErr != nil {
			return fmt.Errorf("store: list loop annotations before definition state delete: %w", listErr)
		}
		state.Annotations = make([]looppkg.UIAnnotation, 0, len(rows))
		for _, row := range rows {
			state.Annotations = append(state.Annotations, looppkg.UIAnnotation{
				NodeID: looppkg.NodeID(row.NodeID),
				X:      row.X,
				Y:      row.Y,
			})
		}
		if deleteErr := queries.DeleteLoopUIAnnotations(ctx, sqlcgen.DeleteLoopUIAnnotationsParams{
			WorkspaceID: string(workspaceID),
			LoopName:    name,
		}); deleteErr != nil {
			return fmt.Errorf("store: delete loop annotations with definition state: %w", deleteErr)
		}
		if deleteErr := queries.DeleteLoopConfig(ctx, sqlcgen.DeleteLoopConfigParams{
			WorkspaceID: string(workspaceID),
			LoopName:    name,
		}); deleteErr != nil {
			return fmt.Errorf("store: delete loop config with definition state: %w", deleteErr)
		}
		return nil
	})
	if err != nil {
		return looppkg.DefinitionState{}, err
	}
	return state, nil
}

// RestoreLoopDefinitionState atomically restores sidecars after a failed file-backed delete.
func (g *LoopRepo) RestoreLoopDefinitionState(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
	state looppkg.DefinitionState,
) error {
	if err := g.checkReady(ctx, "restore loop definition state"); err != nil {
		return err
	}
	workspaceID, name, err := normalizeLoopAnnotationKey(ws, loopName)
	if err != nil {
		return err
	}
	annotations, err := normalizeLoopAnnotations(state.Annotations)
	if err != nil {
		return err
	}
	var normalizedConfig looppkg.LoopConfig
	if state.Config != nil {
		normalizedConfig, err = normalizeLoopConfigForStore(*state.Config)
		if err != nil {
			return err
		}
	}
	return g.withImmediateTransaction(ctx, "restore loop definition state", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		if deleteErr := queries.DeleteLoopUIAnnotations(ctx, sqlcgen.DeleteLoopUIAnnotationsParams{
			WorkspaceID: string(workspaceID),
			LoopName:    name,
		}); deleteErr != nil {
			return fmt.Errorf("store: clear loop annotations before restore: %w", deleteErr)
		}
		if deleteErr := queries.DeleteLoopConfig(ctx, sqlcgen.DeleteLoopConfigParams{
			WorkspaceID: string(workspaceID),
			LoopName:    name,
		}); deleteErr != nil {
			return fmt.Errorf("store: clear loop config before restore: %w", deleteErr)
		}
		if state.Config != nil {
			if upsertErr := upsertLoopConfigWithExecutor(
				ctx,
				exec,
				string(workspaceID),
				name,
				*state.Config,
				normalizedConfig,
			); upsertErr != nil {
				return fmt.Errorf("store: restore loop config: %w", upsertErr)
			}
		}
		for _, annotation := range annotations {
			if insertErr := queries.InsertLoopUIAnnotation(ctx, sqlcgen.InsertLoopUIAnnotationParams{
				WorkspaceID: string(workspaceID),
				LoopName:    name,
				NodeID:      string(annotation.NodeID),
				X:           annotation.X,
				Y:           annotation.Y,
			}); insertErr != nil {
				return fmt.Errorf("store: restore loop annotation %q: %w", annotation.NodeID, insertErr)
			}
		}
		return nil
	})
}
