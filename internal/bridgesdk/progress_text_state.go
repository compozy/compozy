package bridgesdk

import (
	"context"
	"errors"
	"strings"
)

func (a *ProgressAccumulator) applyRepeatLocked(ctx context.Context) error {
	oldSuffix := progressRepeatSuffix(a.repeatCount)
	if !strings.HasSuffix(a.currentText, oldSuffix) {
		return errors.New("bridgesdk: progress repeat state is inconsistent")
	}
	nextCount := a.repeatCount + 1
	nextSuffix := progressRepeatSuffix(nextCount)
	nextText := strings.TrimSuffix(a.currentText, oldSuffix) + nextSuffix
	applied, err := a.replaceCurrentTextLocked(ctx, nextText, nextSuffix)
	if applied {
		a.repeatCount = nextCount
	}
	return err
}

func (a *ProgressAccumulator) appendLineLocked(ctx context.Context, line string) (bool, error) {
	if a.lastLine == "" {
		return a.replaceCurrentTextLocked(ctx, line, "")
	}

	candidate := a.currentText + "\n" + line
	if a.config.Len(candidate) <= a.messageLimitLocked() {
		return a.replaceCurrentTextLocked(ctx, candidate, "")
	}
	if err := a.flushLocked(ctx, true); err != nil {
		return false, err
	}
	a.freezeCurrentBubbleLocked()
	return a.replaceCurrentTextLocked(ctx, line, "")
}

// replaceCurrentTextLocked reports applied only when the final chunk that owns
// protectedTail was consumed and remains addressable. Committed intermediate
// chunks are frozen before the next chunk so they are never replayed.
func (a *ProgressAccumulator) replaceCurrentTextLocked(
	ctx context.Context,
	text string,
	protectedTail string,
) (bool, error) {
	chunks, err := splitProgressText(text, a.messageLimitLocked(), a.config.Len, protectedTail)
	if err != nil {
		return false, err
	}

	var committedErr error
	for index, chunk := range chunks {
		a.currentText = chunk
		a.dirty = true
		force := len(chunks) > 1
		flushErr := a.flushLocked(ctx, force)
		if flushErr != nil {
			if !IsCommittedMutation(flushErr) {
				return false, errors.Join(committedErr, flushErr)
			}
			committedErr = errors.Join(committedErr, flushErr)
			if index == len(chunks)-1 {
				return a.currentBubbleID != "", committedErr
			}
			a.freezeCurrentBubbleLocked()
			continue
		}
		if index < len(chunks)-1 {
			a.freezeCurrentBubbleLocked()
		}
	}
	return true, committedErr
}
