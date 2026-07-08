package executor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/sound"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// recordingPlayer captures the sound names the alert path asks it to play. The
// mutex keeps it safe for the parallel-siblings case, where several job
// goroutines can reach the alert concurrently.
type recordingPlayer struct {
	mu     sync.Mutex
	played []string
}

func (p *recordingPlayer) Play(_ context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.played = append(p.played, name)
	return nil
}

func (p *recordingPlayer) snapshot() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.played...)
}

var _ sound.Player = (*recordingPlayer)(nil)

func TestSoundConfigForGatesOnTheSoundFeatureFlag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		cfg         *config
		wantEnabled bool
		wantParked  string
	}{
		{
			name:        "nil config is silent",
			cfg:         nil,
			wantEnabled: false,
		},
		{
			name:        "sound disabled is silent even with a parked preset",
			cfg:         &config{SoundEnabled: false, SoundOnParked: "funk"},
			wantEnabled: false,
		},
		{
			name:        "sound enabled carries the parked preset",
			cfg:         &config{SoundEnabled: true, SoundOnCompleted: "glass", SoundOnParked: "funk"},
			wantEnabled: true,
			wantParked:  "funk",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			player := &recordingPlayer{}
			got, enabled := soundConfigFor(tc.cfg, player)
			if enabled != tc.wantEnabled {
				t.Fatalf("soundConfigFor enabled = %v, want %v", enabled, tc.wantEnabled)
			}
			if got.OnParked != tc.wantParked {
				t.Fatalf("soundConfigFor OnParked = %q, want %q", got.OnParked, tc.wantParked)
			}
		})
	}
}

// A disabled run must never reach the player, and the gate must return promptly
// rather than blocking the parked job's goroutine on playback.
func TestNotifyParkedAlertIsANoOpWhenSoundIsDisabled(t *testing.T) {
	t.Parallel()

	player := &recordingPlayer{}
	execCtx := &jobExecutionContext{
		ctx:         context.Background(),
		cfg:         &config{SoundEnabled: false, SoundOnParked: "funk"},
		alertPlayer: player,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		execCtx.notifyParkedAlert()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("parked alert blocked the caller while sound was disabled")
	}
	if got := player.snapshot(); len(got) != 0 {
		t.Fatalf("expected no playback when sound is disabled, got %#v", got)
	}
}

func TestNotifyParkedAlertOnNilExecutionContextDoesNotPanic(t *testing.T) {
	t.Parallel()
	var execCtx *jobExecutionContext
	execCtx.notifyParkedAlert()
}

// The end-to-end contract: a job that stalls, retries, then stalls again alerts
// exactly once — on the park — with the configured preset. The intervening
// job.retry_scheduled must stay silent, and the park must carry the triage detail
// the returning user needs.
func TestParkFiresExactlyOneAlertAndSurfacesTriageDetail(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	root := initStallGitRepo(t)
	harness := newStallHarness(
		t,
		stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true},
		stallResult(&job{SafeName: "task_01"}, "Bash go test ./..."),
		stallResult(&job{SafeName: "task_01"}, "Bash go test ./..."),
	)
	player := &recordingPlayer{}
	harness.execCtx.alertPlayer = player
	harness.execCtx.cfg.SoundEnabled = true
	harness.execCtx.cfg.SoundOnParked = model.DefaultSoundOnParked

	harness.runner.executeAttempts(context.Background())

	if got := player.snapshot(); len(got) != 1 || got[0] != model.DefaultSoundOnParked {
		t.Fatalf("expected exactly one %q alert on park, got %#v", model.DefaultSoundOnParked, got)
	}

	evs := harness.drain(t, 5)
	if !hasEvent(evs, eventspkg.EventKindJobRetryScheduled) {
		t.Fatalf("expected a silent stall retry before the park, got %v", eventKinds(evs))
	}
	var payload kinds.JobParkedPayload
	decodeRuntimeEventPayload(t, findEvent(t, evs, eventspkg.EventKindJobParked), &payload)
	if payload.Reason == "" {
		t.Fatal("parked payload must carry a plain-language reason")
	}
	if payload.LastToolCall != "Bash go test ./..." {
		t.Fatalf("parked payload last tool call = %q", payload.LastToolCall)
	}
	if payload.LastProgressSeq == 0 {
		t.Fatal("parked payload must carry the journal high-water sequence")
	}
	if payload.WorktreePath != root {
		t.Fatalf("parked payload worktree = %q, want %q", payload.WorktreePath, root)
	}
	if payload.LogPath != harness.job.OutLog {
		t.Fatalf("parked payload log = %q, want the preserved %q", payload.LogPath, harness.job.OutLog)
	}
}
