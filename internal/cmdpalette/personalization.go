package cmdpalette

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Service) RecordUsage(ctx context.Context, usage Usage) error {
	if err := requirePersonalizationRequest(ctx, usage.WorkspaceID); err != nil {
		return err
	}
	if s.personalization == nil {
		return errors.New("cmd palette: personalization service is unavailable")
	}
	usage.CommandID = CommandID(strings.TrimSpace(string(usage.CommandID)))
	if usage.CommandID == "" {
		return errors.New("cmd palette: command ID is required")
	}
	if err := s.requireCatalogCommand(ctx, usage.WorkspaceID, usage.CommandID); err != nil {
		return err
	}
	enabled, err := s.personalizationEnabled(ctx, usage.WorkspaceID)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	usage.Query = NormalizeQuery(usage.Query)
	if usage.UsedAt.IsZero() {
		usage.UsedAt = s.now().UTC()
	} else {
		usage.UsedAt = usage.UsedAt.UTC()
	}
	if err := s.personalization.RecordCmdPaletteUsage(ctx, usage, WeightsV1); err != nil {
		return fmt.Errorf("cmd palette: record usage for %q: %w", usage.CommandID, err)
	}
	return nil
}

func (s *Service) recordDaemonUsage(ctx context.Context, execution ExecutionRequest) {
	if execution.Descriptor.Action.Kind != ActionKindTool || s.personalization == nil {
		return
	}
	enabled, err := s.personalizationEnabled(ctx, execution.WorkspaceID)
	if err != nil {
		s.logger.WarnContext(ctx, "resolve command palette personalization policy",
			"workspace_id", execution.WorkspaceID,
			"command_id", execution.Descriptor.ID,
			"error", err,
		)
		return
	}
	if !enabled {
		return
	}
	usage := Usage{
		WorkspaceID: execution.WorkspaceID,
		CommandID:   execution.Descriptor.ID,
		UsedAt:      s.now().UTC(),
	}
	if err := s.personalization.RecordCmdPaletteUsage(ctx, usage, WeightsV1); err != nil {
		s.logger.WarnContext(ctx, "record command palette usage",
			"workspace_id", execution.WorkspaceID,
			"command_id", execution.Descriptor.ID,
			"error", err,
		)
	}
}

func (s *Service) Personalization(ctx context.Context, workspaceID WorkspaceID) (Snapshot, error) {
	if err := requirePersonalizationRequest(ctx, workspaceID); err != nil {
		return Snapshot{}, err
	}
	if s.personalization == nil {
		return Snapshot{}, errors.New("cmd palette: personalization service is unavailable")
	}
	enabled, err := s.personalizationEnabled(ctx, workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	if !enabled {
		return newPersonalizationSnapshot(nil, nil, nil)
	}
	descriptors, _, _, err := s.collectDescriptors(ctx, workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	valid := make(map[CommandID]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		valid[descriptor.ID] = struct{}{}
	}
	rows, err := s.personalization.CmdPalettePersonalization(ctx, workspaceID)
	if err != nil {
		s.logPersonalizationDegraded(workspaceID, err)
		return newPersonalizationSnapshot(nil, nil, nil)
	}
	usage, queryHits, pins := s.maintainPersonalization(ctx, workspaceID, valid, rows)
	return newPersonalizationSnapshot(usage, queryHits, pins)
}

func (s *Service) personalizationEnabled(ctx context.Context, workspaceID WorkspaceID) (bool, error) {
	if s.policy == nil {
		return true, nil
	}
	enabled, err := s.policy.PersonalizationEnabled(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("cmd palette: resolve personalization policy: %w", err)
	}
	return enabled, nil
}

func (s *Service) PersonalizationSummary(
	ctx context.Context,
	workspaceID WorkspaceID,
) (PersonalizationSummary, error) {
	snapshot, err := s.Personalization(ctx, workspaceID)
	if err != nil {
		return PersonalizationSummary{}, err
	}
	pins := make([]CommandID, 0, len(snapshot.Pins))
	for _, pin := range snapshot.Pins {
		pins = append(pins, pin.CommandID)
	}
	return PersonalizationSummary{
		Workspace: workspaceID, Pins: pins, Recents: len(snapshot.Usage),
		FrecencyEntries: len(snapshot.Usage), QueryAssociations: len(snapshot.QueryHits),
	}, nil
}

func (s *Service) Pin(ctx context.Context, workspaceID WorkspaceID, commandID CommandID) error {
	return s.changePin(ctx, workspaceID, commandID, true)
}

func (s *Service) Unpin(ctx context.Context, workspaceID WorkspaceID, commandID CommandID) error {
	return s.changePin(ctx, workspaceID, commandID, false)
}

func (s *Service) changePin(
	ctx context.Context,
	workspaceID WorkspaceID,
	commandID CommandID,
	pinned bool,
) error {
	if err := requirePersonalizationRequest(ctx, workspaceID); err != nil {
		return err
	}
	if s.personalization == nil {
		return errors.New("cmd palette: personalization service is unavailable")
	}
	commandID = CommandID(strings.TrimSpace(string(commandID)))
	if err := s.requireCatalogCommand(ctx, workspaceID, commandID); err != nil {
		return err
	}
	var err error
	if pinned {
		err = s.personalization.PutCmdPalettePin(ctx, workspaceID, commandID, s.now().UTC())
	} else {
		err = s.personalization.DeleteCmdPalettePin(ctx, workspaceID, commandID)
	}
	if err != nil {
		return fmt.Errorf("cmd palette: change pin for %q: %w", commandID, err)
	}
	s.emit(ctx, Event{
		Name: EventPinChanged, WorkspaceID: workspaceID, CommandID: commandID,
		Pinned: &pinned, OccurredAt: s.now().UTC(),
	})
	return s.NotifyCatalogChanged(ctx, workspaceID)
}

func (s *Service) ResetPersonalization(ctx context.Context, workspaceID WorkspaceID) error {
	if err := requirePersonalizationRequest(ctx, workspaceID); err != nil {
		return err
	}
	if s.personalization == nil {
		return errors.New("cmd palette: personalization service is unavailable")
	}
	if err := s.personalization.ResetCmdPalettePersonalization(ctx, workspaceID); err != nil {
		return fmt.Errorf("cmd palette: reset personalization: %w", err)
	}
	s.emit(ctx, Event{
		Name: EventPersonalizationReset, WorkspaceID: workspaceID, OccurredAt: s.now().UTC(),
	})
	return s.NotifyCatalogChanged(ctx, workspaceID)
}

func (s *Service) maintainPersonalization(
	ctx context.Context,
	workspaceID WorkspaceID,
	valid map[CommandID]struct{},
	rows PersonalizationRows,
) ([]UsageSignal, []QueryHit, []Pin) {
	now := s.now().UTC()
	usage := make([]UsageSignal, 0, len(rows.Usage))
	queryHits := make([]QueryHit, 0, len(rows.QueryHits))
	pins := make([]Pin, 0, len(rows.Pins))
	staleCommands := make(map[CommandID]struct{})
	for _, signal := range rows.Usage {
		if _, ok := valid[signal.CommandID]; !ok {
			staleCommands[signal.CommandID] = struct{}{}
			continue
		}
		last := time.UnixMilli(signal.LastUsedAt)
		signal.Weight = DecayFrecency(signal.Weight, last, now, WeightsV1.frecencyHalfLife())
		if shouldPruneSignal(signal.Weight, last, now) {
			if err := s.personalization.PruneCmdPaletteUsage(ctx, workspaceID, signal.CommandID); err != nil {
				s.logPruneError(ctx, workspaceID, signal.CommandID, err)
			}
			continue
		}
		usage = append(usage, signal)
	}
	for _, hit := range rows.QueryHits {
		if _, ok := valid[hit.CommandID]; !ok {
			staleCommands[hit.CommandID] = struct{}{}
			continue
		}
		last := time.UnixMilli(hit.LastUsedAt)
		hit.Weight = DecayFrecency(hit.Weight, last, now, WeightsV1.queryHalfLife())
		if shouldPruneSignal(hit.Weight, last, now) {
			if err := s.personalization.PruneCmdPaletteQueryHit(
				ctx, workspaceID, hit.Query, hit.CommandID,
			); err != nil {
				s.logPruneQueryError(ctx, workspaceID, hit.Query, hit.CommandID, err)
			}
			continue
		}
		queryHits = append(queryHits, hit)
	}
	for _, pin := range rows.Pins {
		if _, ok := valid[pin.CommandID]; !ok {
			staleCommands[pin.CommandID] = struct{}{}
			continue
		}
		pins = append(pins, pin)
	}
	for commandID := range staleCommands {
		if err := s.personalization.PruneCmdPaletteCommand(ctx, workspaceID, commandID); err != nil {
			s.logPruneError(ctx, workspaceID, commandID, err)
		}
	}
	return usage, queryHits, pins
}

func (s *Service) logPruneError(
	ctx context.Context,
	workspaceID WorkspaceID,
	commandID CommandID,
	err error,
) {
	s.logger.WarnContext(ctx, "prune stale command palette personalization",
		"workspace_id", workspaceID, "command_id", commandID, "error", err)
}

func (s *Service) logPruneQueryError(
	ctx context.Context,
	workspaceID WorkspaceID,
	query string,
	commandID CommandID,
	err error,
) {
	s.logger.WarnContext(ctx, "prune stale command palette personalization",
		"workspace_id", workspaceID, "query", query, "command_id", commandID, "error", err)
}

func shouldPruneSignal(weight float64, last, now time.Time) bool {
	return now.Sub(last) >= time.Duration(WeightsV1.PruneAfterDays)*24*time.Hour &&
		weight < WeightsV1.PruneThreshold
}

func (s *Service) requireCatalogCommand(
	ctx context.Context,
	workspaceID WorkspaceID,
	commandID CommandID,
) error {
	if commandID == "" {
		return errors.New("cmd palette: command ID is required")
	}
	descriptors, _, _, err := s.collectDescriptors(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, descriptor := range descriptors {
		if descriptor.ID == commandID {
			return nil
		}
	}
	return ErrCommandNotFound
}

func requirePersonalizationRequest(ctx context.Context, workspaceID WorkspaceID) error {
	if ctx == nil {
		return errors.New("cmd palette: personalization context is required")
	}
	if strings.TrimSpace(string(workspaceID)) == "" {
		return errors.New("cmd palette: workspace ID is required")
	}
	return nil
}

func (s *Service) logPersonalizationDegraded(workspaceID WorkspaceID, err error) {
	s.degradedOnce.Do(func() {
		s.logger.Warn("command palette personalization degraded to empty signals",
			"workspace_id", workspaceID, "error", err)
	})
}

func newPersonalizationSnapshot(
	usage []UsageSignal,
	queryHits []QueryHit,
	pins []Pin,
) (Snapshot, error) {
	weights := WeightsV1
	weights.GroupOrder = append([]string(nil), WeightsV1.GroupOrder...)
	snapshot := Snapshot{
		Weights:   weights,
		Usage:     append(make([]UsageSignal, 0, len(usage)), usage...),
		QueryHits: append(make([]QueryHit, 0, len(queryHits)), queryHits...),
		Pins:      append(make([]Pin, 0, len(pins)), pins...),
	}
	sort.Slice(snapshot.Usage, func(left, right int) bool {
		if snapshot.Usage[left].LastUsedAt != snapshot.Usage[right].LastUsedAt {
			return snapshot.Usage[left].LastUsedAt > snapshot.Usage[right].LastUsedAt
		}
		return snapshot.Usage[left].CommandID < snapshot.Usage[right].CommandID
	})
	sort.Slice(snapshot.QueryHits, func(left, right int) bool {
		if snapshot.QueryHits[left].Query != snapshot.QueryHits[right].Query {
			return snapshot.QueryHits[left].Query < snapshot.QueryHits[right].Query
		}
		return snapshot.QueryHits[left].CommandID < snapshot.QueryHits[right].CommandID
	})
	sort.Slice(snapshot.Pins, func(left, right int) bool {
		if snapshot.Pins[left].PinnedAt != snapshot.Pins[right].PinnedAt {
			return snapshot.Pins[left].PinnedAt < snapshot.Pins[right].PinnedAt
		}
		return snapshot.Pins[left].CommandID < snapshot.Pins[right].CommandID
	})
	payload, err := json.Marshal(struct {
		Weights Weights       `json:"weights"`
		Usage   []UsageSignal `json:"usage"`
		Hits    []QueryHit    `json:"query_hits"`
		Pins    []Pin         `json:"pins"`
	}{Weights: snapshot.Weights, Usage: snapshot.Usage, Hits: snapshot.QueryHits, Pins: snapshot.Pins})
	if err != nil {
		return Snapshot{}, fmt.Errorf("cmd palette: encode personalization snapshot: %w", err)
	}
	digest := sha256.Sum256(payload)
	snapshot.Revision = "ps_" + hex.EncodeToString(digest[:])
	return snapshot, nil
}
