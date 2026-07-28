package daemon

import (
	"strings"

	"unicode/utf8"
)

func aggregatePromptInputBudget(descriptors []promptInputAugmenterDescriptor) int {
	total := 0
	for _, descriptor := range descriptors {
		if descriptor.Budget <= 0 {
			continue
		}
		total += descriptor.Budget
	}
	return total
}

func applyPromptInputAugmenterBudget(
	current string,
	next string,
	limited bool,
	remainingBudget int,
	behavior promptInputAugmenterBudgetBehavior,
) (string, int) {
	if !limited {
		return next, promptInputContributionRunes(current, next)
	}
	if remainingBudget <= 0 {
		return current, 0
	}

	before, after, wrapped := splitPromptInputAugmentation(current, next)
	if wrapped {
		contribution := utf8.RuneCountInString(before) + utf8.RuneCountInString(after)
		if contribution <= remainingBudget {
			return next, contribution
		}
		if normalizePromptInputAugmenterBudgetBehavior(behavior) == promptInputAugmenterBudgetBehaviorOmit {
			return current, 0
		}

		beforeBudget := min(utf8.RuneCountInString(before), remainingBudget)
		trimmedBefore := trimStringToRunes(before, beforeBudget)
		trimmedAfter := trimStringToRunes(after, remainingBudget-beforeBudget)
		return strings.TrimSpace(trimmedBefore + current + trimmedAfter), remainingBudget
	}

	contribution := promptInputContributionRunes(current, next)
	if contribution <= remainingBudget {
		return next, contribution
	}
	if normalizePromptInputAugmenterBudgetBehavior(behavior) == promptInputAugmenterBudgetBehaviorOmit {
		return current, 0
	}
	return current, 0
}

func splitPromptInputAugmentation(current string, next string) (string, string, bool) {
	if current == "" {
		return "", "", false
	}

	before, after, ok := strings.Cut(next, current)
	if !ok {
		return "", "", false
	}
	return before, after, true
}

func promptInputContributionRunes(current string, next string) int {
	before, after, wrapped := splitPromptInputAugmentation(current, next)
	if wrapped {
		return utf8.RuneCountInString(before) + utf8.RuneCountInString(after)
	}
	return max(utf8.RuneCountInString(next)-utf8.RuneCountInString(current), 0)
}
