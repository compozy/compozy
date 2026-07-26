package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type networkWakeDecision struct {
	reservation *store.WakeReservation
	skip        *store.WakeSkip
}

type networkWakeBudget struct {
	wakesUsed        int
	wallMSUsed       int64
	inputTokensUsed  int64
	outputTokensUsed int64
}

type openNetworkWake struct {
	reservation store.WakeReservation
	runStatus   string
	rootID      string
}

const (
	networkWakeExhaustionMaxWakes         = "max_wakes"
	networkWakeExhaustionMaxTotalWallTime = "max_total_wall_time"
	networkWakeExhaustionMaxInputTokens   = "max_input_tokens"
	networkWakeExhaustionMaxOutputTokens  = "max_output_tokens"
)

func (g *NetworkRepo) admitNetworkWake(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	input store.NetworkWakeAdmissionInput,
	acceptanceSeq int64,
	now time.Time,
	networkEnabled bool,
) (networkWakeDecision, error) {
	if reason := structuralNetworkWakeSkip(input); reason != "" {
		return skippedNetworkWake(input, reason, ""), nil
	}
	if existingWakeID, exists, err := findNetworkWakeSource(
		ctx,
		exec,
		message.WorkspaceID,
		input.OwnerKey,
		message.MessageID,
	); err != nil {
		return networkWakeDecision{}, err
	} else if exists {
		return skippedNetworkWake(input, store.NetworkWakeSkipDuplicate, existingWakeID), nil
	}
	if !networkEnabled {
		return skippedNetworkWake(input, store.NetworkWakeSkipNetworkDisabled, ""), nil
	}
	if input.Depth >= input.Spec.Bounds.MaxWakeDepth {
		return skippedNetworkWake(input, store.NetworkWakeSkipDepthExceeded, ""), nil
	}

	openWake, exists, err := findOpenNetworkWake(ctx, exec, message.WorkspaceID, input.OwnerKey)
	if err != nil {
		return networkWakeDecision{}, err
	}
	if exists {
		return admitAgainstOpenNetworkWake(ctx, exec, message, input, openWake, now)
	}

	reservation, exhaustedReason, err := g.reserveNetworkWake(
		ctx,
		exec,
		message,
		input,
		acceptanceSeq,
		now,
	)
	if err != nil {
		return networkWakeDecision{}, err
	}
	if exhaustedReason != "" {
		return skippedNetworkWake(input, store.NetworkWakeSkipBudgetExhausted, ""), nil
	}
	return networkWakeDecision{reservation: &reservation}, nil
}

func admitAgainstOpenNetworkWake(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	input store.NetworkWakeAdmissionInput,
	openWake openNetworkWake,
	now time.Time,
) (networkWakeDecision, error) {
	if openWake.rootID == input.RootID &&
		openWake.runStatus == taskpkg.TaskRunStatusQueued.String() &&
		now.Before(openWake.reservation.CoalesceUntil) {
		if err := insertNetworkWakeSource(
			ctx,
			exec,
			message.WorkspaceID,
			input.OwnerKey,
			message.MessageID,
			openWake.reservation.WakeID,
		); err != nil {
			return networkWakeDecision{}, err
		}
		return skippedNetworkWake(
			input,
			store.NetworkWakeSkipCoalesced,
			openWake.reservation.WakeID,
		), nil
	}
	if openWake.rootID != input.RootID {
		budget, _, exhaustedReason, err := evaluateNetworkWakeBudget(
			ctx,
			exec,
			message.WorkspaceID,
			input.OwnerKey,
			input.Spec.Bounds,
		)
		if err != nil {
			return networkWakeDecision{}, err
		}
		if exhaustedReason != "" {
			if err := writeNetworkWakeBudgetExhaustion(
				ctx,
				exec,
				message.WorkspaceID,
				input.OwnerKey,
				budget,
				exhaustedReason,
				now,
			); err != nil {
				return networkWakeDecision{}, err
			}
			return skippedNetworkWake(input, store.NetworkWakeSkipBudgetExhausted, ""), nil
		}
	}
	return skippedNetworkWake(input, store.NetworkWakeSkipInFlight, openWake.reservation.WakeID), nil
}

func structuralNetworkWakeSkip(input store.NetworkWakeAdmissionInput) string {
	if input.Spec.Mode != participation.ModeLive {
		return store.NetworkWakeSkipNotLive
	}
	if !input.Eligible {
		return store.NetworkWakeSkipNotEligible
	}
	if !input.Addressed {
		return store.NetworkWakeSkipNotAddressed
	}
	return ""
}

func skippedNetworkWake(
	input store.NetworkWakeAdmissionInput,
	reason string,
	wakeID string,
) networkWakeDecision {
	return networkWakeDecision{skip: &store.WakeSkip{
		RecipientSessionID: input.RecipientSessionID,
		OwnerKey:           input.OwnerKey,
		Reason:             reason,
		WakeID:             wakeID,
	}}
}

func (g *NetworkRepo) reserveNetworkWake(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	input store.NetworkWakeAdmissionInput,
	acceptanceSeq int64,
	now time.Time,
) (store.WakeReservation, string, error) {
	budget, wakeWall, reason, err := evaluateNetworkWakeBudget(
		ctx,
		exec,
		message.WorkspaceID,
		input.OwnerKey,
		input.Spec.Bounds,
	)
	if err != nil {
		return store.WakeReservation{}, "", err
	}
	coalesceWindow, err := time.ParseDuration(input.Spec.Bounds.CoalesceWindow)
	if err != nil {
		return store.WakeReservation{}, "", fmt.Errorf("store: parse network coalesce window: %w", err)
	}
	if reason != "" {
		if err := writeNetworkWakeBudgetExhaustion(
			ctx,
			exec,
			message.WorkspaceID,
			input.OwnerKey,
			budget,
			reason,
			now,
		); err != nil {
			return store.WakeReservation{}, "", err
		}
		return store.WakeReservation{}, reason, nil
	}

	reservation := store.WakeReservation{
		WakeID:           input.WakeID,
		TaskRunID:        input.TaskRunID,
		WorkspaceID:      message.WorkspaceID,
		OwnerKey:         input.OwnerKey,
		EnvelopeIDs:      []string{message.MessageID},
		ReservedWallTime: wakeWall.String(),
		CoalesceUntil:    now.Add(coalesceWindow),
	}
	if err := g.persistNetworkWakeReservation(
		ctx,
		exec,
		&message,
		&input,
		&reservation,
		budget,
		wakeWall,
		acceptanceSeq,
		now,
	); err != nil {
		return store.WakeReservation{}, "", err
	}
	return reservation, "", nil
}

func (g *NetworkRepo) persistNetworkWakeReservation(
	ctx context.Context,
	exec networkSQLExecutor,
	message *store.NetworkConversationMessage,
	input *store.NetworkWakeAdmissionInput,
	reservation *store.WakeReservation,
	budget networkWakeBudget,
	wakeWall time.Duration,
	acceptanceSeq int64,
	now time.Time,
) error {
	if err := g.insertNetworkWakeRun(ctx, exec, *message, *input, acceptanceSeq, now); err != nil {
		return err
	}
	reservedInput := input.Spec.Bounds.MaxInputTokens - budget.inputTokensUsed
	reservedOutput := input.Spec.Bounds.MaxOutputTokens - budget.outputTokensUsed
	if err := insertNetworkWakeLedger(
		ctx,
		exec,
		*message,
		*input,
		*reservation,
		durationMillisecondsCeil(wakeWall),
		reservedInput,
		reservedOutput,
		now,
	); err != nil {
		return err
	}
	if err := insertNetworkWakeSource(
		ctx,
		exec,
		message.WorkspaceID,
		input.OwnerKey,
		message.MessageID,
		input.WakeID,
	); err != nil {
		return err
	}
	if err := reserveNetworkWakeBudget(
		ctx,
		exec,
		message.WorkspaceID,
		input.OwnerKey,
		durationMillisecondsCeil(wakeWall),
		reservedInput,
		reservedOutput,
		now,
	); err != nil {
		return err
	}
	if err := appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
		workspaceID: message.WorkspaceID, wakeID: input.WakeID, taskRunID: input.TaskRunID,
		ownerKey: input.OwnerKey, targetSessionID: input.RecipientSessionID,
		eventType: networkWakeEventAdmitted, state: taskpkg.TaskRunStatusQueued.String(),
		actor: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindNetworkPeer, Ref: message.PeerFrom},
		at:    now,
	}); err != nil {
		return err
	}
	return nil
}

func evaluateNetworkWakeBudget(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	ownerKey string,
	bounds participation.Bounds,
) (networkWakeBudget, time.Duration, string, error) {
	wakeWall, err := time.ParseDuration(bounds.MaxWakeWallTime)
	if err != nil {
		return networkWakeBudget{}, 0, "", fmt.Errorf("store: parse network wake wall time: %w", err)
	}
	totalWall, err := time.ParseDuration(bounds.MaxTotalWallTime)
	if err != nil {
		return networkWakeBudget{}, 0, "", fmt.Errorf("store: parse network total wall time: %w", err)
	}
	budget, err := getNetworkWakeBudget(ctx, exec, workspaceID, ownerKey)
	if err != nil {
		return networkWakeBudget{}, 0, "", err
	}
	reason := exhaustedNetworkWakeBudgetReason(
		budget,
		bounds,
		durationMillisecondsCeil(wakeWall),
		durationMillisecondsCeil(totalWall),
	)
	return budget, wakeWall, reason, nil
}

func durationMillisecondsCeil(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	return int64((duration + time.Millisecond - 1) / time.Millisecond)
}

func (g *NetworkRepo) insertNetworkWakeRun(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	input store.NetworkWakeAdmissionInput,
	acceptanceSeq int64,
	now time.Time,
) error {
	run := taskpkg.Run{
		ID: input.TaskRunID, RunKind: taskpkg.RunKindNetworkWake,
		Status: taskpkg.TaskRunStatusQueued, Attempt: 1,
		WorkspaceID:    strings.TrimSpace(message.WorkspaceID),
		Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindNetwork, Ref: "network.accept:" + message.MessageID},
		IdempotencyKey: fmt.Sprintf("network:%s:%s:%d", input.OwnerKey, message.MessageID, acceptanceSeq),
		QueuedAt:       now,
	}
	run.SetNetworkState(input.Spec, input.WakeID, input.RecipientSessionID, input.OwnerKey)
	normalized, err := g.tasks.normalizeTaskRunForCreate(run)
	if err != nil {
		return fmt.Errorf("store: normalize network wake task run: %w", err)
	}
	if err := insertTaskRunWithExecutor(ctx, exec, normalized); err != nil {
		return fmt.Errorf("store: enqueue network wake task run: %w", err)
	}
	return nil
}

func exhaustedNetworkWakeBudgetReason(
	budget networkWakeBudget,
	bounds participation.Bounds,
	reservedWallMS int64,
	totalWallMS int64,
) string {
	switch {
	case budget.wakesUsed >= bounds.MaxWakes:
		return networkWakeExhaustionMaxWakes
	case budget.wallMSUsed+reservedWallMS > totalWallMS:
		return networkWakeExhaustionMaxTotalWallTime
	case budget.inputTokensUsed >= bounds.MaxInputTokens:
		return networkWakeExhaustionMaxInputTokens
	case budget.outputTokensUsed >= bounds.MaxOutputTokens:
		return networkWakeExhaustionMaxOutputTokens
	default:
		return ""
	}
}

func normalizeNetworkWakeState(value string) string {
	return strings.TrimSpace(value)
}
