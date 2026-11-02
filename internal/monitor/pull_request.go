package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Pull request watches poll one forge read per target per interval and coalesce what they see into wakes.
const (
	pullRequestPollInterval = time.Minute
	// Monitors of one target tick together, so a read this recent answers the whole tick. It stays short so
	// the first tick after a registration still reads the forge rather than that registration's own read.
	pullRequestReadSpacing = 5 * time.Second
	// The read may queue behind other forge reads, so it outlives one forge request timeout.
	pullRequestReadTimeout = 45 * time.Second
	// A burst of review activity wakes after this much quiet, held at most reviewHold from its first event.
	reviewQuiet = 2 * time.Minute
	reviewHold  = 5 * time.Minute
	// forgeOutageGrace is how long lookups must keep failing before the agent hears about it.
	forgeOutageGrace = 15 * time.Minute
	// MaxPullRequestFires caps the wakes of one monitor; the last one says so and ends it.
	MaxPullRequestFires = 64
	// Cursor and wake bounds keep a durable row small whatever the forge returns.
	maxTrackedFailedNames = 64
	maxTrackedReviews     = 100
	maxSeenReviews        = 128
	maxBurstReviews       = 32
	maxForgeTextRunes     = 200
)

// PullRequestLookup says what one forge read established.
type PullRequestLookup string

const (
	// PullRequestFound means the read succeeded and names a pull request.
	PullRequestFound PullRequestLookup = "found"
	// PullRequestAbsent means no pull request is known for the branch yet, which is a wait.
	PullRequestAbsent PullRequestLookup = "absent"
	// PullRequestFailed means the read failed; it says nothing about checks or reviews.
	PullRequestFailed PullRequestLookup = "failed"
)

// PullRequestTarget identifies the watched pull request. Watches of one target share reads.
type PullRequestTarget struct {
	WorkspaceID string
	RepoKey     string
	Branch      string
}

// PullRequestChecks are the check facts of one head.
type PullRequestChecks struct {
	Passed, Failed, Pending int
	FailedNames             []string
}

// PullRequestReview is one submitted review. Author is untrusted forge text.
type PullRequestReview struct {
	ID     string `json:"id"`
	Author string `json:"author,omitempty"`
	IsBot  bool   `json:"is_bot,omitempty"`
	State  string `json:"state"`
	Commit string `json:"commit,omitempty"`
}

// PullRequestReviews is review activity. A nil value means the forge cannot observe it.
type PullRequestReviews struct {
	Items             []PullRequestReview
	UnresolvedThreads int
	Truncated         bool
}

// PullRequestObservation is one forge read. Checks is nil when the forge could not establish them.
type PullRequestObservation struct {
	Lookup PullRequestLookup
	// Cause says why a lookup failed.
	Cause   string
	Number  int
	URL     string
	State   string
	Head    string
	Checks  *PullRequestChecks
	Reviews *PullRequestReviews
}

// PullRequestSource reads pull requests through the daemon's forge. ResolvePullRequest binds a spec to its
// target, defaulting the branch to the session's, and refuses an unusable one with InvalidSpec.
type PullRequestSource interface {
	ResolvePullRequest(ctx context.Context, spec Spec) (PullRequestTarget, error)
	ReadPullRequest(ctx context.Context, target PullRequestTarget) (PullRequestObservation, error)
}

// WithPullRequests enables pull_request monitors.
func WithPullRequests(source PullRequestSource) Option {
	return func(s *Service) { s.pullRequests = source }
}

// pullRequestWatch is the durable watch state of one pull_request monitor. Reported facts in cursor
// advance only after a wake is admitted; pending holds the fire a restart must replay unchanged.
type pullRequestWatch struct {
	repoKey string
	cursor  prCursor
	pending *prFire
}

// prCursor is what the agent was already told, plus quiet progress that needs no wake.
type prCursor struct {
	Number           int            `json:"number,omitempty"`
	Head             string         `json:"head,omitempty"`
	Failed           int            `json:"failed,omitempty"`
	FailedNames      []string       `json:"failed_names,omitempty"`
	Finished         bool           `json:"finished,omitempty"`
	ReviewsBaselined bool           `json:"reviews_baselined,omitempty"`
	SeenReviews      []string       `json:"seen_reviews,omitempty"`
	Unresolved       int            `json:"unresolved,omitempty"`
	Burst            *prReviewBurst `json:"burst,omitempty"`
	OutageSince      time.Time      `json:"outage_since,omitzero"`
	OutageReported   bool           `json:"outage_reported,omitempty"`
}

// prReviewBurst is review activity waiting out its debounce.
type prReviewBurst struct {
	Since      time.Time           `json:"since"`
	Last       time.Time           `json:"last"`
	Reviews    []PullRequestReview `json:"reviews,omitempty"`
	Unresolved int                 `json:"unresolved"`
}

// prFire is one wake from its durable decision until admission.
type prFire struct {
	Changes PullRequestChanges `json:"changes"`
	Next    prCursor           `json:"next"`
	Final   bool               `json:"final,omitempty"`
}

type pullRequestDurable struct {
	Cursor  prCursor `json:"cursor"`
	Pending *prFire  `json:"pending,omitempty"`
}

// PullRequestChanges is what one wake tells the agent.
type PullRequestChanges struct {
	Wake         bool                `json:"wake,omitempty"`
	Number       int                 `json:"number,omitempty"`
	URL          string              `json:"url,omitempty"`
	State        string              `json:"state,omitempty"`
	Head         string              `json:"head,omitempty"`
	ChecksKnown  bool                `json:"checks_known,omitempty"`
	Passed       int                 `json:"passed,omitempty"`
	Failed       int                 `json:"failed,omitempty"`
	Pending      int                 `json:"pending,omitempty"`
	FailedNames  []string            `json:"failed_names,omitempty"`
	NewlyFailed  []string            `json:"newly_failed,omitempty"`
	FailureRose  bool                `json:"failure_rose,omitempty"`
	AllFinished  bool                `json:"all_finished,omitempty"`
	ReviewsKnown bool                `json:"reviews_known,omitempty"`
	Truncated    bool                `json:"truncated,omitempty"`
	Reviews      []PullRequestReview `json:"reviews,omitempty"`
	NewThreads   int                 `json:"new_threads,omitempty"`
	Unresolved   int                 `json:"unresolved,omitempty"`
	Terminal     string              `json:"terminal,omitempty"`
	ForgeDown    bool                `json:"forge_down,omitempty"`
	Cause        string              `json:"cause,omitempty"`
	TimedOut     bool                `json:"timed_out,omitempty"`
	FireCapHit   bool                `json:"fire_cap_hit,omitempty"`
}

func (changes PullRequestChanges) reason() string {
	switch {
	case changes.Terminal == pullRequestMerged:
		return ReasonPullRequestMerged
	case changes.Terminal != "":
		return ReasonPullRequestClosed
	case changes.ForgeDown:
		return ReasonForgeUnavailable
	default:
		return ReasonPullRequestChanged
	}
}

func newPullRequestWatch(spec Spec, target PullRequestTarget) *pullRequestWatch {
	if spec.Kind != KindPullRequest {
		return nil
	}
	return &pullRequestWatch{repoKey: target.RepoKey}
}

func (w *pullRequestWatch) encode() string {
	if w == nil {
		return ""
	}
	data, err := json.Marshal(pullRequestDurable{Cursor: w.cursor, Pending: w.pending})
	if err != nil {
		panic("invariant: pull request cursor holds only marshalable values: " + err.Error())
	}
	return string(data)
}

// decodePullRequestWatch restores a durable watch. An unreadable cursor restarts from a new baseline,
// which can repeat a report but never ends the watch.
func decodePullRequestWatch(repoKey, raw string) (*pullRequestWatch, error) {
	watch := &pullRequestWatch{repoKey: repoKey}
	if raw == "" {
		return watch, nil
	}
	var durable pullRequestDurable
	if err := json.Unmarshal([]byte(raw), &durable); err != nil {
		return watch, fmt.Errorf("monitor: decode pull request cursor: %w", err)
	}
	watch.cursor, watch.pending = durable.Cursor, durable.Pending
	return watch, nil
}

func (e *entry) target() PullRequestTarget {
	return PullRequestTarget{WorkspaceID: e.workspaceID, RepoKey: e.pullRequest.repoKey, Branch: e.state.Branch}
}

// sameWatch reports whether e already covers the watch candidate would add.
func (e *entry) sameWatch(candidate *entry) bool {
	if e.pullRequest != nil && candidate.pullRequest != nil {
		return e.target() == candidate.target()
	}
	return e.terminalKey != "" && e.terminalKey == candidate.terminalKey &&
		e.state.Until == candidate.state.Until && e.state.Pattern == candidate.state.Pattern &&
		e.timeout == candidate.timeout && e.state.Description == candidate.state.Description
}

func (s *Service) resolvePullRequest(ctx context.Context, spec Spec) (PullRequestTarget, error) {
	if s.pullRequests == nil {
		return PullRequestTarget{}, errors.New("monitor: pull request monitors are unavailable")
	}
	target, err := s.pullRequests.ResolvePullRequest(ctx, spec)
	if errors.Is(err, ErrInvalid) {
		return PullRequestTarget{}, err // Its message is the caller's correction; a prefix would bury it.
	}
	if err != nil {
		return PullRequestTarget{}, fmt.Errorf("monitor: resolve pull request: %w", err)
	}
	return target, nil
}

// WakeMessageID is the stable queue identity of this wake. A repeating monitor needs one per fire, or the
// ledger would swallow every wake after the first as already admitted.
func (n Notification) WakeMessageID() string {
	if n.Monitor.Kind == KindPullRequest {
		return "monitor:" + n.Monitor.ID + ":" + strconv.Itoa(n.Monitor.FireCount)
	}
	return "monitor:" + n.Monitor.ID
}
