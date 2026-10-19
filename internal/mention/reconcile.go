package mention

// Reconcile narrows admitted mentions to those whose exact Token still parses
// as a token in text, typically after an input.pre_submit hook rewrite
// (Business Rule 9). Hooks can never add mentions, so the result is always a
// subset of admitted.
//
// Surviving mentions keep their admitted offsets: those point into the
// authored text, which is what the transcript displays and renders chips
// over, even when a hook rewrote the delivered message.
//
// Repeated tokens pair in order: each occurrence of a token in text keeps one
// admitted mention carrying it, earliest first, and surplus admitted entries
// drop. The result keeps admitted order.
func Reconcile(text string, admitted []Mention) []Mention {
	if len(admitted) == 0 {
		return nil
	}
	remaining := make(map[string]int)
	for _, token := range Parse(text) {
		remaining[token.Raw]++
	}
	reconciled := make([]Mention, 0, len(admitted))
	for _, mention := range admitted {
		if remaining[mention.Token] == 0 {
			continue
		}
		remaining[mention.Token]--
		reconciled = append(reconciled, mention)
	}
	return reconciled
}
