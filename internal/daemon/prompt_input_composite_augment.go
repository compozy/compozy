package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"

	"time"

	"github.com/compozy/compozy/internal/session"
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
	resolved, err := c.resolver.ResolvePrompt(info, source, sess.CurrentPromptMeta())
	if err != nil {
		return "", fmt.Errorf("daemon: resolve prompt augmentation policy: %w", err)
	}
	timestamp := time.Now().UTC()
	if c.recorder != nil {
		timestamp = c.recorder.timestamp(time.Time{})
		c.recorder.RecordPromptContextResolved(ctx, info, &resolved, timestamp)
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
			&resolved,
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
	resolved *ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	current string,
	remainingBudget int,
	limited bool,
	timestamp time.Time,
) (string, int, error) {
	var next string
	var augmentErr error
	if descriptor.PolicyAugmenter != nil {
		next, augmentErr = descriptor.PolicyAugmenter(ctx, sess, current, resolved)
	} else {
		next, augmentErr = descriptor.Augmenter(ctx, sess, current)
	}
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
	registerDeliveredAugmentation(ctx, descriptor.Name, current, nextCurrent)
	return nextCurrent, nextBudget, nil
}

func (c *promptInputComposite) handleAugmenterFailure(
	ctx context.Context,
	sess *session.Session,
	info *session.Info,
	resolved *ResolvedHarnessContext,
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

// Register the final budgeted prefix, excluding user text and section separators.
// Skills owns its registration because it also carries startup/stub signatures.
func registerDeliveredAugmentation(ctx context.Context, name HarnessAugmenter, current, next string) {
	var key string
	switch name {
	case HarnessAugmenterWorkspaceKnowledge:
		key = "knowledge"
	case HarnessAugmenterDurableMemory:
		key = "memory"
	case HarnessAugmenterSituation:
		key = "situation"
	default:
		return
	}
	if current == next {
		return
	}
	content := next
	if current != "" {
		before, after, ok := splitPromptInputAugmentation(current, next)
		if !ok || after != "" {
			return
		}
		content = before
	}
	content = strings.TrimSpace(content)
	if content != "" {
		acp.RegisterPromptSection(ctx, acp.PromptSection{Key: key, Content: content})
	}
}
