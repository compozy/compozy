package monitor

import (
	"fmt"
	"strings"
)

// A wake stays readable whatever the forge returned.
const (
	maxWakeNames   = 20
	maxWakeReviews = 10
	maxWakeRunes   = 120
)

func pullRequestStatusLine(changes *PullRequestChanges) string {
	switch {
	case changes.FireCapHit:
		return "wake limit reached; this monitor has ended"
	case changes.TimedOut:
		return "timed out; this monitor has ended, and the pull request may still be open"
	case changes.Terminal != "":
		return "pull request " + changes.Terminal + "; this monitor has ended"
	case changes.ForgeDown:
		return "the forge has been unavailable for 15 minutes; this monitor stays armed"
	default:
		return "pull request changed; this monitor stays armed"
	}
}

// pullRequestLines renders forge facts inside an entity-encoded block marked untrusted, the same way a
// terminal wake carries output.
func pullRequestLines(mon Monitor, changes *PullRequestChanges) []string {
	body := make([]string, 0, 12)
	add := func(format string, args ...any) { body = append(body, fmt.Sprintf(format, args...)) }
	if changes.Number > 0 {
		add("pull request: #%d", changes.Number)
	}
	add("branch: %s", forgeLine(mon.Branch))
	if changes.URL != "" {
		add("url: %s", forgeLine(changes.URL))
	}
	if changes.Head != "" {
		add("head: %s", forgeLine(changes.Head))
	}
	if changes.ForgeDown {
		add("forge lookup cause: %s", forgeLine(changes.Cause))
	}
	if changes.EndedNumber > 0 {
		add("no open pull request seen; #%d had already ended when this monitor read it", changes.EndedNumber)
	}
	if changes.Conflict {
		add("PR #%d now conflicts with %s; rebase", changes.Number, forgeLine(changes.Base))
	}
	if changes.ChecksKnown {
		add("checks: %d passed, %d failed, %d pending", changes.Passed, changes.Failed, changes.Pending)
		if changes.AllFinished {
			add("all checks finished on this head")
		}
		if len(changes.NewlyFailed) > 0 {
			add("newly failed: %s", forgeList(changes.NewlyFailed))
		} else if changes.FailureRose {
			add("another check failed; the forge did not name it")
		}
		if len(changes.FailedNames) > 0 {
			add("failing now: %s", forgeList(changes.FailedNames))
		}
	}
	reviewLines(changes, add)
	if notes := unobservable(changes); notes != "" {
		add("not observable: %s", notes)
	}
	lines := []string{"<pull-request untrusted=\"true\" encoding=\"entities\">"}
	return append(append(lines, body...), "</pull-request>")
}

func reviewLines(changes *PullRequestChanges, add func(string, ...any)) {
	for i, review := range changes.Reviews {
		if i == maxWakeReviews {
			add("and %d more new reviews", len(changes.Reviews)-maxWakeReviews)
			break
		}
		author := forgeLine(review.Author)
		if review.IsBot {
			author += " (bot)"
		}
		add("new review: %s by %s", forgeLine(review.State), author)
	}
	if changes.NewThreads > 0 {
		add("new unresolved threads: %d", changes.NewThreads)
	}
	if changes.ReviewsKnown {
		add("unresolved threads: %d", changes.Unresolved)
	}
	if changes.Truncated {
		add("review counts are partial: the forge reported more than one read returns")
	}
	if changes.ReviewsDown {
		add("reviews have been unreadable for 15 minutes; this monitor stays armed and will report them when it can")
	}
}

// unobservable names what an observed wake could not see. A wake with no observation behind it, such as
// a timeout or an outage, already says so.
func unobservable(changes *PullRequestChanges) string {
	if changes.State == "" || changes.Terminal != "" {
		return ""
	}
	var missing []string
	if !changes.ChecksKnown {
		missing = append(missing, "checks")
	}
	switch {
	case changes.ReviewsFailed:
		missing = append(missing, "reviews and threads (this read failed)")
	case !changes.ReviewsKnown:
		missing = append(missing, "reviews and threads")
	}
	return strings.Join(missing, ", ")
}

func pullRequestNextAction(changes *PullRequestChanges) string {
	switch {
	case changes.FireCapHit, changes.TimedOut:
		return "Check the pull request yourself. Register a new pull_request monitor if you still need to wait on it."
	case changes.Terminal != "":
		return "Continue the task that registered this monitor and report the outcome to the user."
	case changes.ForgeDown:
		return "Nothing is known about checks or reviews right now. Do not treat this as a CI result. " +
			"The monitor stays armed, so do not poll."
	default:
		return "Read what changed before acting: the failing check logs, and every new review with its review body " +
			"and threads, because follow-ups can live only in the review body. This wake carries no review text. " +
			"You decide when delivery is done. The monitor stays armed, so do not poll, and cancel it with " +
			"compozy__monitor_cancel when you no longer need it."
	}
}

func forgeLine(value string) string {
	value = strings.Join(strings.Fields(strings.ToValidUTF8(value, "�")), " ")
	if runes := []rune(value); len(runes) > maxWakeRunes {
		value = string(runes[:maxWakeRunes]) + "…"
	}
	return escapeOutput.Replace(value)
}

func forgeList(names []string) string {
	shown := make([]string, 0, min(len(names), maxWakeNames))
	for _, name := range names[:min(len(names), maxWakeNames)] {
		shown = append(shown, forgeLine(name))
	}
	line := strings.Join(shown, ", ")
	if len(names) > maxWakeNames {
		line += fmt.Sprintf(" and %d more", len(names)-maxWakeNames)
	}
	return line
}
