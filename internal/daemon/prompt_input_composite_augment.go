package daemon

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/compozy/agh/internal/session"
)

func (c *promptInputComposite) Augment(
	ctx context.Context,
	sess *session.Session,
	message string,
) (string, error) {
	if c == nil || c.resolver == nil || sess == nil {
		return message, nil
	}

	info := sess.Info()
	if info == nil {
		return message, nil
	}

	source := sess.CurrentTurnSource()
	if source == session.TurnSourceSynthetic {
		return message, nil
	}

	resolved, err := c.resolver.ResolvePrompt(info, source, sess.CurrentPromptMeta())
	if err != nil {
		return "", fmt.Errorf("daemon: resolve prompt augmentation policy: %w", err)
	}
	timestamp := time.Now().UTC()
	if c.recorder != nil {
		timestamp = c.recorder.timestamp(time.Time{})
		c.recorder.RecordPromptContextResolved(ctx, info, resolved, timestamp)
	}

	descriptors, err := c.selectedDescriptors(resolved.Policy.EnableAugmenters)
	if err != nil {
		return "", err
	}
	if len(descriptors) == 0 {
		return message, nil
	}

	limited := aggregatePromptInputBudget(descriptors) > 0
	remainingBudget := aggregatePromptInputBudget(descriptors)
	current := message

	for _, descriptor := range descriptors {
		var stepErr error
		current, remainingBudget, stepErr = c.applyAugmenterDescriptor(
			ctx,
			sess,
			info,
			resolved,
			descriptor,
			current,
			remainingBudget,
			limited,
			timestamp,
		)
		if stepErr != nil {
			return "", stepErr
		}
	}

	return current, nil
}

func (c *promptInputComposite) applyAugmenterDescriptor(
	ctx context.Context,
	sess *session.Session,
	info *session.Info,
	resolved ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	current string,
	remainingBudget int,
	limited bool,
	timestamp time.Time,
) (string, int, error) {
	next, augmentErr := descriptor.Augmenter(ctx, sess, current)
	if augmentErr != nil {
		return c.handleAugmenterFailure(
			ctx,
			sess,
			info,
			resolved,
			descriptor,
			current,
			remainingBudget,
			timestamp,
			augmentErr,
		)
	}

	nextCurrent, nextBudget := c.applyAugmentedMessage(
		ctx,
		info,
		resolved,
		descriptor,
		current,
		next,
		remainingBudget,
		limited,
		timestamp,
	)
	return nextCurrent, nextBudget, nil
}

func (c *promptInputComposite) handleAugmenterFailure(
	ctx context.Context,
	sess *session.Session,
	info *session.Info,
	resolved ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	current string,
	remainingBudget int,
	timestamp time.Time,
	augmentErr error,
) (string, int, error) {
	wrappedErr := fmt.Errorf("daemon: prompt augmenter %q: %w", descriptor.Name, augmentErr)
	if c.recorder != nil {
		c.recorder.RecordAugmenterFailed(ctx, info, resolved, descriptor, augmentErr, timestamp)
	}
	if descriptor.Critical ||
		errors.Is(augmentErr, context.Canceled) ||
		errors.Is(augmentErr, context.DeadlineExceeded) {
		return "", remainingBudget, wrappedErr
	}
	c.loggerForSession(sess).Warn(
		"daemon: noncritical prompt augmenter failed",
		"augmenter",
		descriptor.Name,
		"error",
		augmentErr,
	)
	return current, remainingBudget, nil
}
