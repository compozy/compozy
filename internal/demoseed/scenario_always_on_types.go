package demoseed

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
)

const (
	checkResultQuiet    = "quiet"
	checkResultReported = "reported"
	checkResultFailed   = "failed"
	checkResultSkipped  = "skipped"
	reasonTriageTimeout = "triage_timeout"
	saoPauloOffset      = -3 * time.Hour
)

// instanceStory is one always-on agent's life in one project: when it was turned on, how it
// checked in since, what its main session holds, and where the operator left it.
type instanceStory struct {
	Agent        string
	WorkspaceKey string
	MainID       string
	MainName     string
	Provider     string
	Model        string
	CreatedAt    time.Time
	// Interval is the check-in cadence; routine checks run every Interval inside the active hours.
	Interval   time.Duration
	ActiveFrom int
	ActiveTo   int
	// QuietEvery makes every Nth routine check a model turn that found nothing to report;
	// the others stop at the digest gate without a model call.
	QuietEvery int
	Beats      []checkInBeat
	Turns      []mainTurn
	Notes      []noteStory
	Actions    []heartbeat.ActionDecl
	SeenAt     time.Time
	PauseAt    time.Time
	// NextCheckIn is when the next check is due after the scenario's now.
	NextCheckIn time.Duration
}

// checkInBeat is one check-in that did something worth telling.
type checkInBeat struct {
	At        time.Time
	Result    string
	Reason    string
	Detail    string
	Proposals []proposalStory
	Failure   string
	// PickAt is when the operator picked proposal PickIndex; zero leaves it waiting.
	PickAt    time.Time
	PickIndex int
	// Reply is the main-session turn the picked proposal started, operator or auto-accepted.
	Reply []transcriptStep
}

type proposalStory struct {
	Title    string
	Body     string
	ItemKeys []string
	Action   string
}

// mainTurn is one conversation turn in the agent's main session that no check-in started.
type mainTurn struct {
	At    time.Time
	Steps []transcriptStep
}

type noteStory struct {
	At    time.Time
	Scope string
	Text  string
}

// routineCheckTimes lays checks on the cadence inside the agent's local active hours, leaving
// room around each scripted beat so two checks never overlap.
func routineCheckTimes(story instanceStory, now time.Time) []time.Time {
	start := story.CreatedAt.Add(story.Interval)
	end := now.Add(-time.Minute)
	if !story.PauseAt.IsZero() {
		end = story.PauseAt
	}
	times := []time.Time{}
	for at := start; at.Before(end); at = at.Add(story.Interval) {
		if !inActiveHours(at, story.ActiveFrom, story.ActiveTo) || nearBeat(story.Beats, at, story.Interval/2) {
			continue
		}
		times = append(times, at)
	}
	return times
}

func inActiveHours(at time.Time, from int, to int) bool {
	hour := at.Add(saoPauloOffset).Hour()
	return hour >= from && hour < to
}

func nearBeat(beats []checkInBeat, at time.Time, window time.Duration) bool {
	for _, beat := range beats {
		gap := beat.At.Sub(at)
		if gap < window && gap > -window {
			return true
		}
	}
	return false
}

// jsonObject renders an authored tool payload; string maps always encode.
func jsonObject(fields map[string]any) string {
	raw, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
