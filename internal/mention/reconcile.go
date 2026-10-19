package mention

import "slices"

// Reconcile narrows admitted mentions to those whose exact Token still parses
// as a token in text, typically after an input.pre_submit hook rewrite
// (Business Rule 9). Offsets move to the new positions; hooks can never add
// mentions, so the result is always a subset of admitted.
//
// Repeated tokens pair in order: the n-th admitted mention carrying a token
// takes the n-th occurrence of that token in text, so each occurrence keeps
// its own entry and surplus admitted entries drop. The result is in text order.
func Reconcile(text string, admitted []Mention) []Mention {
	if len(admitted) == 0 {
		return nil
	}
	occurrences := make(map[string][]Token)
	for _, token := range Parse(text) {
		occurrences[token.Raw] = append(occurrences[token.Raw], token)
	}
	reconciled := make([]Mention, 0, len(admitted))
	for _, mention := range admitted {
		queue := occurrences[mention.Token]
		if len(queue) == 0 {
			continue
		}
		mention.Start, mention.End = queue[0].Start, queue[0].End
		occurrences[mention.Token] = queue[1:]
		reconciled = append(reconciled, mention)
	}
	slices.SortStableFunc(reconciled, func(left, right Mention) int {
		return left.Start - right.Start
	})
	return reconciled
}
