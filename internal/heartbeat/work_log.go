package heartbeat

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"
)

type WorkLogFact struct {
	ID         string
	AcceptedAt time.Time
}

var workLogID = regexp.MustCompile(`^wl-[0-9]{8}-[0-9]{2,}$`)

// ParseWorkLog returns accepted entries and malformed line numbers for daemon logging.
func ParseWorkLog(content string) ([]WorkLogFact, []int) {
	var facts []WorkLogFact
	var malformed []int
	for i, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "# Work log" {
			continue
		}
		fields := strings.Split(line, " | ")
		if len(fields) < 3 {
			malformed = append(malformed, i+1)
			continue
		}
		state, id, ok := strings.Cut(fields[0], "] ")
		accepted, err := time.Parse(time.RFC3339, fields[1])
		if !ok || !slices.Contains([]string{"- [ ", "- [~", "- [x"}, state) || !workLogID.MatchString(id) ||
			err != nil ||
			!strings.HasSuffix(fields[1], "Z") ||
			strings.TrimSpace(fields[2]) == "" {
			malformed = append(malformed, i+1)
			continue
		}
		if state == "- [ " {
			facts = append(facts, WorkLogFact{ID: id, AcceptedAt: accepted})
		}
	}
	return facts, malformed
}

func collectWorkLogFacts(scope CollectorScope, facts CollectorFacts) []DigestItem {
	if scope.WorkspaceID == "" {
		return nil
	}
	stale := make([]WorkLogFact, 0, len(facts.WorkLog))
	for _, fact := range facts.WorkLog {
		if fact.AcceptedAt.Before(scope.Now.Add(-30 * time.Minute)) {
			stale = append(stale, fact)
		}
	}
	slices.SortFunc(
		stale,
		func(a, b WorkLogFact) int { return cmp.Or(a.AcceptedAt.Compare(b.AcceptedAt), cmp.Compare(a.ID, b.ID)) },
	)
	var items []DigestItem
	seen := map[string]bool{}
	for _, fact := range stale {
		if seen[fact.ID] {
			continue
		}
		seen[fact.ID] = true
		if len(items) == 9 {
			items = append(items, DigestItem{Key: "work:more"})
			break
		}
		items = append(items, DigestItem{Key: "work:" + fact.ID})
	}
	if facts.WorkLogTruncated {
		items = append(items, DigestItem{Key: "work:log-truncated"})
	}
	return items
}
