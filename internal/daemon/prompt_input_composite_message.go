package daemon

import (
	"context"

	"fmt"
	"log/slog"

	"strings"
	"time"

	"github.com/compozy/agh/internal/session"
)

func (c *promptInputComposite) applyAugmentedMessage(
	ctx context.Context,
	info *session.Info,
	resolved ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	current string,
	next string,
	remainingBudget int,
	limited bool,
	timestamp time.Time,
) (string, int) {
	if strings.TrimSpace(next) == "" {
		c.recordAugmenterApplied(
			ctx,
			info,
			resolved,
			descriptor,
			"blank",
			0,
			remainingBudget,
			timestamp,
		)
		return current, remainingBudget
	}

	descriptorBudget := remainingBudget
	if limited && descriptor.Budget > 0 {
		descriptorBudget = min(remainingBudget, descriptor.Budget)
	}
	bounded, consumed := applyPromptInputAugmenterBudget(
		current,
		next,
		limited,
		descriptorBudget,
		descriptor.BudgetBehavior,
	)
	if strings.TrimSpace(bounded) == "" {
		nextBudget := max(remainingBudget-consumed, 0)
		c.recordAugmenterApplied(
			ctx,
			info,
			resolved,
			descriptor,
			"omitted",
			consumed,
			nextBudget,
			timestamp,
		)
		return current, remainingBudget
	}

	outcome := nativeAppliedValue
	if bounded == current {
		outcome = "unchanged"
	} else if bounded != next {
		outcome = "trimmed"
	}

	nextBudget := remainingBudget
	if limited {
		nextBudget = max(remainingBudget-consumed, 0)
	}
	c.recordAugmenterApplied(
		ctx,
		info,
		resolved,
		descriptor,
		outcome,
		consumed,
		nextBudget,
		timestamp,
	)
	return bounded, nextBudget
}

func (c *promptInputComposite) recordAugmenterApplied(
	ctx context.Context,
	info *session.Info,
	resolved ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	outcome string,
	consumed int,
	remaining int,
	timestamp time.Time,
) {
	if c.recorder == nil {
		return
	}
	c.recorder.RecordAugmenterApplied(ctx, info, resolved, harnessAugmenterObservation{
		Name:           descriptor.Name,
		Outcome:        outcome,
		Critical:       descriptor.Critical,
		Budget:         descriptor.Budget,
		BudgetBehavior: descriptor.BudgetBehavior,
		Consumed:       consumed,
		Remaining:      remaining,
	}, timestamp)
}

func (c *promptInputComposite) selectedDescriptors(
	enabled []HarnessAugmenter,
) ([]promptInputAugmenterDescriptor, error) {
	if len(enabled) == 0 {
		return nil, nil
	}

	enabledSet := make(map[HarnessAugmenter]struct{}, len(enabled))
	for _, name := range enabled {
		enabledSet[name] = struct{}{}
		if _, ok := c.byName[name]; !ok {
			return nil, fmt.Errorf("daemon: enabled prompt augmenter %q is not registered", name)
		}
	}

	selected := make([]promptInputAugmenterDescriptor, 0, len(enabled))
	for _, descriptor := range c.descriptors {
		if _, ok := enabledSet[descriptor.Name]; ok {
			selected = append(selected, descriptor)
		}
	}
	return selected, nil
}

func (c *promptInputComposite) loggerForSession(sess *session.Session) *slog.Logger {
	logger := c.logger
	if logger == nil {
		logger = slog.Default()
	}
	if sess == nil {
		return logger
	}

	info := sess.Info()
	if info == nil {
		return logger
	}
	return logger.With("session_id", info.ID, "agent_name", info.AgentName)
}
