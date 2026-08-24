package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ cmdpalette.PersonalizationStore = (*CmdPaletteRepo)(nil)

func (r *CmdPaletteRepo) RecordCmdPaletteUsage(
	ctx context.Context,
	usage cmdpalette.Usage,
	weights cmdpalette.Weights,
) error {
	if err := r.checkReady(ctx, "record command palette usage"); err != nil {
		return err
	}
	workspaceID := strings.TrimSpace(string(usage.WorkspaceID))
	profileLensID, err := requireCmdPaletteLens(usage.ProfileLens.ID)
	if err != nil {
		return err
	}
	commandID := strings.TrimSpace(string(usage.CommandID))
	query := cmdpalette.NormalizeQuery(usage.Query)
	if workspaceID == "" || commandID == "" {
		return errors.New("store: command palette usage requires workspace and command IDs")
	}
	usedAt := usage.UsedAt.UTC()
	if usedAt.IsZero() {
		usedAt = r.now().UTC()
	}
	if err := r.withImmediateTransaction(ctx, "record command palette usage", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		if err := putCmdPaletteUsage(
			ctx, queries, string(profileLensID), workspaceID, commandID, usedAt, weights,
		); err != nil {
			return err
		}
		if query == "" {
			return nil
		}
		return putCmdPaletteQueryHit(
			ctx, queries, string(profileLensID), workspaceID, query, commandID, usedAt, weights,
		)
	}); err != nil {
		return fmt.Errorf("store: record command palette usage for %q/%q: %w", workspaceID, commandID, err)
	}
	return nil
}

func putCmdPaletteUsage(
	ctx context.Context,
	queries *sqlcgen.Queries,
	profileLensID string,
	workspaceID string,
	commandID string,
	usedAt time.Time,
	weights cmdpalette.Weights,
) error {
	row, err := queries.GetCmdPaletteUsage(ctx, sqlcgen.GetCmdPaletteUsageParams{
		ProfileLensID: profileLensID,
		WorkspaceID:   workspaceID,
		CommandID:     commandID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read prior command palette usage: %w", err)
	}
	count := int64(1)
	weight := 1.0
	if err == nil {
		count = row.UseCount + 1
		weight += cmdpalette.DecayFrecency(
			row.FrecencyWeight,
			time.UnixMilli(row.LastUsedAt),
			usedAt,
			time.Duration(weights.FrecencyHalfLifeDays)*24*time.Hour,
		)
	}
	if err := queries.PutCmdPaletteUsage(ctx, sqlcgen.PutCmdPaletteUsageParams{
		ProfileLensID: profileLensID,
		WorkspaceID:   workspaceID, CommandID: commandID, UseCount: count,
		FrecencyWeight: weight, LastUsedAt: usedAt.UnixMilli(), UpdatedAt: usedAt.UnixMilli(),
	}); err != nil {
		return fmt.Errorf("upsert command palette usage: %w", err)
	}
	return nil
}

func putCmdPaletteQueryHit(
	ctx context.Context,
	queries *sqlcgen.Queries,
	profileLensID string,
	workspaceID string,
	query string,
	commandID string,
	usedAt time.Time,
	weights cmdpalette.Weights,
) error {
	row, err := queries.GetCmdPaletteQueryHit(ctx, sqlcgen.GetCmdPaletteQueryHitParams{
		ProfileLensID: profileLensID,
		WorkspaceID:   workspaceID,
		Query:         query,
		CommandID:     commandID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read prior command palette query hit: %w", err)
	}
	weight := 1.0
	if err == nil {
		weight += cmdpalette.DecayFrecency(
			row.Weight,
			time.UnixMilli(row.LastUsedAt),
			usedAt,
			time.Duration(weights.QueryHalfLifeDays)*24*time.Hour,
		)
	}
	if err := queries.PutCmdPaletteQueryHit(ctx, sqlcgen.PutCmdPaletteQueryHitParams{
		ProfileLensID: profileLensID,
		WorkspaceID:   workspaceID, Query: query, CommandID: commandID,
		Weight: weight, LastUsedAt: usedAt.UnixMilli(),
	}); err != nil {
		return fmt.Errorf("upsert command palette query hit: %w", err)
	}
	return nil
}

func (r *CmdPaletteRepo) CmdPalettePersonalization(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
) (cmdpalette.PersonalizationRows, error) {
	if err := r.checkReady(ctx, "read command palette personalization"); err != nil {
		return cmdpalette.PersonalizationRows{}, err
	}
	workspace := strings.TrimSpace(string(workspaceID))
	profileLensID, err := requireCmdPaletteLens(profileLensID)
	if err != nil {
		return cmdpalette.PersonalizationRows{}, err
	}
	if workspace == "" {
		return cmdpalette.PersonalizationRows{}, errors.New("store: command palette workspace ID is required")
	}
	usageRows, err := r.queries.ListCmdPaletteUsage(ctx, sqlcgen.ListCmdPaletteUsageParams{
		ProfileLensID: string(profileLensID), WorkspaceID: workspace,
	})
	if err != nil {
		return cmdpalette.PersonalizationRows{}, fmt.Errorf("store: list command palette usage: %w", err)
	}
	queryRows, err := r.queries.ListCmdPaletteQueryHits(ctx, sqlcgen.ListCmdPaletteQueryHitsParams{
		ProfileLensID: string(profileLensID), WorkspaceID: workspace,
	})
	if err != nil {
		return cmdpalette.PersonalizationRows{}, fmt.Errorf("store: list command palette query hits: %w", err)
	}
	pinRows, err := r.queries.ListCmdPalettePins(ctx, sqlcgen.ListCmdPalettePinsParams{
		ProfileLensID: string(profileLensID), WorkspaceID: workspace,
	})
	if err != nil {
		return cmdpalette.PersonalizationRows{}, fmt.Errorf("store: list command palette pins: %w", err)
	}
	rows := cmdpalette.PersonalizationRows{
		Usage:     make([]cmdpalette.UsageSignal, 0, len(usageRows)),
		QueryHits: make([]cmdpalette.QueryHit, 0, len(queryRows)),
		Pins:      make([]cmdpalette.Pin, 0, len(pinRows)),
	}
	for _, row := range usageRows {
		if row.UseCount < 0 || invalidCmdPaletteWeight(row.FrecencyWeight) || row.LastUsedAt < 0 {
			return cmdpalette.PersonalizationRows{}, fmt.Errorf(
				"store: corrupt command palette usage row %q",
				row.CommandID,
			)
		}
		rows.Usage = append(rows.Usage, cmdpalette.UsageSignal{
			CommandID: cmdpalette.CommandID(row.CommandID), UseCount: row.UseCount,
			Weight: row.FrecencyWeight, LastUsedAt: row.LastUsedAt,
		})
	}
	for _, row := range queryRows {
		if strings.TrimSpace(row.Query) == "" || invalidCmdPaletteWeight(row.Weight) || row.LastUsedAt < 0 {
			return cmdpalette.PersonalizationRows{}, fmt.Errorf(
				"store: corrupt command palette query row %q",
				row.CommandID,
			)
		}
		rows.QueryHits = append(rows.QueryHits, cmdpalette.QueryHit{
			Query: row.Query, CommandID: cmdpalette.CommandID(row.CommandID),
			Weight: row.Weight, LastUsedAt: row.LastUsedAt,
		})
	}
	for _, row := range pinRows {
		if row.PinnedAt < 0 {
			return cmdpalette.PersonalizationRows{}, fmt.Errorf(
				"store: corrupt command palette pin row %q",
				row.CommandID,
			)
		}
		rows.Pins = append(rows.Pins, cmdpalette.Pin{
			CommandID: cmdpalette.CommandID(row.CommandID),
			PinnedAt:  row.PinnedAt,
		})
	}
	return rows, nil
}

func invalidCmdPaletteWeight(weight float64) bool {
	return weight < 0 || math.IsNaN(weight) || math.IsInf(weight, 0)
}

func (r *CmdPaletteRepo) PutCmdPalettePin(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
	pinnedAt time.Time,
) error {
	if err := r.checkReady(ctx, "pin command palette command"); err != nil {
		return err
	}
	workspaceID, commandID, err := requireCmdPaletteIdentity(workspaceID, commandID)
	if err != nil {
		return err
	}
	profileLensID, err = requireCmdPaletteLens(profileLensID)
	if err != nil {
		return err
	}
	if pinnedAt.IsZero() {
		pinnedAt = r.now().UTC()
	}
	if err := r.queries.PutCmdPalettePin(ctx, sqlcgen.PutCmdPalettePinParams{
		ProfileLensID: string(profileLensID),
		WorkspaceID:   string(workspaceID), CommandID: string(commandID), PinnedAt: pinnedAt.UTC().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("store: pin command palette command %q: %w", commandID, err)
	}
	return nil
}

func (r *CmdPaletteRepo) DeleteCmdPalettePin(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) error {
	if err := r.checkReady(ctx, "unpin command palette command"); err != nil {
		return err
	}
	workspaceID, commandID, err := requireCmdPaletteIdentity(workspaceID, commandID)
	if err != nil {
		return err
	}
	profileLensID, err = requireCmdPaletteLens(profileLensID)
	if err != nil {
		return err
	}
	if err := r.queries.DeleteCmdPalettePin(ctx, sqlcgen.DeleteCmdPalettePinParams{
		ProfileLensID: string(profileLensID),
		WorkspaceID:   string(workspaceID), CommandID: string(commandID),
	}); err != nil {
		return fmt.Errorf("store: unpin command palette command %q: %w", commandID, err)
	}
	return nil
}

func (r *CmdPaletteRepo) PruneCmdPaletteCommand(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) error {
	if err := r.checkReady(ctx, "prune command palette command"); err != nil {
		return err
	}
	workspaceID, commandID, err := requireCmdPaletteIdentity(workspaceID, commandID)
	if err != nil {
		return err
	}
	profileLensID, err = requireCmdPaletteLens(profileLensID)
	if err != nil {
		return err
	}
	if err := r.withImmediateTransaction(ctx, "prune command palette command", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		params := sqlcgen.DeleteCmdPaletteUsageParams{
			ProfileLensID: string(profileLensID),
			WorkspaceID:   string(workspaceID), CommandID: string(commandID),
		}
		if err := queries.DeleteCmdPaletteUsage(ctx, params); err != nil {
			return fmt.Errorf("delete usage: %w", err)
		}
		if err := queries.DeleteCmdPaletteQueryHitsByCommand(
			ctx,
			sqlcgen.DeleteCmdPaletteQueryHitsByCommandParams(params),
		); err != nil {
			return fmt.Errorf("delete query hits: %w", err)
		}
		if err := queries.DeleteCmdPalettePin(ctx, sqlcgen.DeleteCmdPalettePinParams(params)); err != nil {
			return fmt.Errorf("delete pin: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("store: prune command palette command %q: %w", commandID, err)
	}
	return nil
}

func (r *CmdPaletteRepo) PruneCmdPaletteUsage(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) error {
	if err := r.checkReady(ctx, "prune command palette usage"); err != nil {
		return err
	}
	workspaceID, commandID, err := requireCmdPaletteIdentity(workspaceID, commandID)
	if err != nil {
		return err
	}
	profileLensID, err = requireCmdPaletteLens(profileLensID)
	if err != nil {
		return err
	}
	if err := r.queries.DeleteCmdPaletteUsage(ctx, sqlcgen.DeleteCmdPaletteUsageParams{
		ProfileLensID: string(profileLensID),
		WorkspaceID:   string(workspaceID), CommandID: string(commandID),
	}); err != nil {
		return fmt.Errorf("store: prune command palette usage %q: %w", commandID, err)
	}
	return nil
}

func (r *CmdPaletteRepo) PruneCmdPaletteQueryHit(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
	query string,
	commandID cmdpalette.CommandID,
) error {
	if err := r.checkReady(ctx, "prune command palette query hit"); err != nil {
		return err
	}
	workspaceID, commandID, err := requireCmdPaletteIdentity(workspaceID, commandID)
	if err != nil {
		return err
	}
	profileLensID, err = requireCmdPaletteLens(profileLensID)
	if err != nil {
		return err
	}
	query = cmdpalette.NormalizeQuery(query)
	if query == "" {
		return errors.New("store: command palette query is required")
	}
	if err := r.queries.DeleteCmdPaletteQueryHit(ctx, sqlcgen.DeleteCmdPaletteQueryHitParams{
		ProfileLensID: string(profileLensID),
		WorkspaceID:   string(workspaceID), Query: query, CommandID: string(commandID),
	}); err != nil {
		return fmt.Errorf("store: prune command palette query hit %q/%q: %w", query, commandID, err)
	}
	return nil
}

func (r *CmdPaletteRepo) ResetCmdPalettePersonalization(
	ctx context.Context,
	profileLensID cmdpalette.ProfileLensID,
	workspaceID cmdpalette.WorkspaceID,
) error {
	if err := r.checkReady(ctx, "reset command palette personalization"); err != nil {
		return err
	}
	workspace := strings.TrimSpace(string(workspaceID))
	profileLensID, err := requireCmdPaletteLens(profileLensID)
	if err != nil {
		return err
	}
	if workspace == "" {
		return errors.New("store: command palette workspace ID is required")
	}
	if err := r.withImmediateTransaction(
		ctx,
		"reset command palette personalization",
		func(exec globalSQLExecutor) error {
			queries := sqlcgen.New(exec)
			if err := queries.DeleteCmdPalettePersonalization(
				ctx,
				sqlcgen.DeleteCmdPalettePersonalizationParams{
					ProfileLensID: string(profileLensID), WorkspaceID: workspace,
				},
			); err != nil {
				return fmt.Errorf("delete usage: %w", err)
			}
			if err := queries.DeleteCmdPaletteQueryHistory(
				ctx,
				sqlcgen.DeleteCmdPaletteQueryHistoryParams{
					ProfileLensID: string(profileLensID), WorkspaceID: workspace,
				},
			); err != nil {
				return fmt.Errorf("delete query history: %w", err)
			}
			if err := queries.DeleteCmdPalettePins(
				ctx,
				sqlcgen.DeleteCmdPalettePinsParams{
					ProfileLensID: string(profileLensID), WorkspaceID: workspace,
				},
			); err != nil {
				return fmt.Errorf("delete pins: %w", err)
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf("store: reset command palette personalization for %q: %w", workspace, err)
	}
	return nil
}

func requireCmdPaletteIdentity(
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) (cmdpalette.WorkspaceID, cmdpalette.CommandID, error) {
	workspace := cmdpalette.WorkspaceID(strings.TrimSpace(string(workspaceID)))
	command := cmdpalette.CommandID(strings.TrimSpace(string(commandID)))
	if workspace == "" || command == "" {
		return "", "", errors.New("store: command palette workspace and command IDs are required")
	}
	return workspace, command, nil
}

func requireCmdPaletteLens(profileLensID cmdpalette.ProfileLensID) (cmdpalette.ProfileLensID, error) {
	normalized := cmdpalette.ProfileLensID(strings.TrimSpace(string(profileLensID)))
	if err := normalized.Validate(); err != nil {
		return "", err
	}
	return normalized, nil
}
