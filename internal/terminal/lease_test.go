package terminal

// Suite: terminal control lease.
// Invariant: exactly one actor can deliver whole input, and ownership changes fence stale generations atomically.
// Boundary IN: attach/takeover/yield/write actions. Boundary OUT: bytes handed to the process writer.

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestLeaseMachineContract(t *testing.T) {
	t.Parallel()

	agent := Actor{Kind: ActorKindAgent, ID: "agent", ProfileID: "profile-a", SessionID: "session", RunID: "run", Generation: 3}
	humanA := Actor{Kind: ActorKindHuman, ID: "human-a", ProfileID: "profile-a"}
	humanB := Actor{Kind: ActorKindHuman, ID: "human-b", ProfileID: "profile-a"}

	t.Run("Should allow only the controller to write [UT-017][UT-021]", func(t *testing.T) {
		t.Parallel()
		var output lockedBuffer
		lease := newLeaseMachine(humanA, &output, time.Second, nil)
		start := make(chan struct{})
		errorsSeen := make(chan error, 2)
		for _, attempt := range []struct {
			actor Actor
			text  string
		}{{humanA, "accepted"}, {humanB, "rejected"}} {
			attempt := attempt
			go func() {
				<-start
				errorsSeen <- lease.deliver(attempt.actor, []byte(attempt.text))
			}()
		}
		close(start)
		first, second := <-errorsSeen, <-errorsSeen
		if (first == nil) == (second == nil) {
			t.Fatalf("write errors = %v, %v, want exactly one success", first, second)
		}
		failed := first
		if failed == nil {
			failed = second
		}
		var terminalErr *Error
		if !errors.As(failed, &terminalErr) || terminalErr.Code != "write_owner_held" || terminalErr.Controller == nil || terminalErr.Controller.ID != humanA.ID {
			t.Fatalf("rejected write error = %#v, want named controller", failed)
		}
		if got := output.String(); got != "accepted" {
			t.Fatalf("process bytes = %q, want accepted", got)
		}
	})

	t.Run("Should let a human take control from an agent immediately [UT-018]", func(t *testing.T) {
		t.Parallel()
		transitions := make(chan string, 1)
		lease := newLeaseMachine(agent, io.Discard, time.Second, func(from, to LeaseState, reason string, _ Actor, _ *Actor) {
			transitions <- string(from) + ">" + string(to) + ":" + reason
		})
		if err := lease.takeover(humanA, true); err != nil {
			t.Fatalf("takeover() error = %v", err)
		}
		state, controller := lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || controller.ID != humanA.ID {
			t.Fatalf("lease = %s %#v, want human-a", state, controller)
		}
		if got := <-transitions; got != "agent_owned>human_owned:takeover" {
			t.Fatalf("transition = %q", got)
		}
	})

	t.Run("Should linearize takeover after a whole in-flight write [UT-019][IT-007]", func(t *testing.T) {
		t.Parallel()
		writer := newBlockingPartialWriter()
		lease := newLeaseMachine(agent, writer, time.Second, nil)
		writeDone := make(chan error, 1)
		go func() { writeDone <- lease.deliver(agent, []byte("whole-input")) }()
		<-writer.firstWrite
		takeoverDone := make(chan error, 1)
		go func() { takeoverDone <- lease.takeover(humanA, true) }()
		select {
		case err := <-takeoverDone:
			t.Fatalf("takeover completed before write boundary: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
		close(writer.resume)
		if err := <-writeDone; err != nil {
			t.Fatalf("in-flight write error = %v", err)
		}
		if err := <-takeoverDone; err != nil {
			t.Fatalf("takeover error = %v", err)
		}
		if got := writer.String(); got != "whole-input" {
			t.Fatalf("delivered bytes = %q", got)
		}
		if err := lease.deliver(agent, []byte("late")); !errors.Is(err, ErrLeaseRevoked) {
			t.Fatalf("late write error = %v, want ErrLeaseRevoked", err)
		}
	})

	t.Run("Should select one winner for simultaneous forced human takeovers [UT-020]", func(t *testing.T) {
		t.Parallel()
		lease := newLeaseMachine(agent, io.Discard, time.Second, nil)
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, actor := range []Actor{humanA, humanB} {
			actor := actor
			go func() {
				<-start
				results <- lease.takeover(actor, true)
			}()
		}
		close(start)
		first, second := <-results, <-results
		if (first == nil) == (second == nil) {
			t.Fatalf("takeover errors = %v, %v, want one winner", first, second)
		}
		loser := first
		if loser == nil {
			loser = second
		}
		var terminalErr *Error
		if !errors.As(loser, &terminalErr) || terminalErr.Controller == nil {
			t.Fatalf("loser error = %#v, want winner identity", loser)
		}
	})

	t.Run("Should fence a stale runtime generation without side effects [UT-022][UT-109]", func(t *testing.T) {
		t.Parallel()
		var output lockedBuffer
		lease := newLeaseMachine(agent, &output, time.Second, nil)
		stale := agent
		stale.Generation = 2
		if err := lease.deliver(stale, []byte("forbidden")); !errors.Is(err, ErrGenerationFenced) {
			t.Fatalf("stale write error = %v, want ErrGenerationFenced", err)
		}
		if err := lease.authorize(stale); !errors.Is(err, ErrGenerationFenced) {
			t.Fatalf("stale attachment authorization error = %v, want ErrGenerationFenced", err)
		}
		if err := lease.takeover(stale, true); !errors.Is(err, ErrGenerationFenced) {
			t.Fatalf("stale takeover error = %v, want ErrGenerationFenced", err)
		}
		state, controller := lease.snapshot()
		if output.Len() != 0 || state != LeaseAgentOwned || controller == nil || !sameActor(*controller, agent) {
			t.Fatalf("stale write mutated output=%q lease=%s controller=%#v", output.String(), state, controller)
		}
	})

	t.Run("Should auto-yield one time and keep human ownership on release [UT-023]", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		transitions := 0
		lease := newLeaseMachine(agent, io.Discard, time.Second, func(LeaseState, LeaseState, string, Actor, *Actor) {
			mu.Lock()
			transitions++
			mu.Unlock()
		})
		lease.runEnded(agent, "run_ended")
		lease.runEnded(agent, "run_ended")
		if err := lease.yield(Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: agent.ProfileID}); err != nil {
			t.Fatalf("human yield error = %v", err)
		}
		mu.Lock()
		gotTransitions := transitions
		mu.Unlock()
		state, _ := lease.snapshot()
		if gotTransitions != 1 || state != LeaseHumanOwned {
			t.Fatalf("transitions=%d state=%s, want one and human_owned", gotTransitions, state)
		}
	})

	t.Run("Should start grace only after the final controller attachment closes [UT-024]", func(t *testing.T) {
		t.Parallel()
		transition := make(chan LeaseState, 2)
		lease := newLeaseMachine(humanA, io.Discard, 35*time.Millisecond, func(_ LeaseState, to LeaseState, _ string, _ Actor, _ *Actor) {
			transition <- to
		})
		first := lease.attachWriter(humanA)
		second := lease.attachWriter(humanA)
		lease.detachWriter(first)
		time.Sleep(50 * time.Millisecond)
		state, _ := lease.snapshot()
		if state != LeaseHumanOwned {
			t.Fatalf("state after one detach = %s, want held", state)
		}
		lease.detachWriter(second)
		resumed := lease.attachWriter(humanA)
		time.Sleep(50 * time.Millisecond)
		state, _ = lease.snapshot()
		if state != LeaseHumanOwned {
			t.Fatalf("state after grace cancellation = %s, want held", state)
		}
		lease.detachWriter(resumed)
		select {
		case got := <-transition:
			if got != LeaseAvailable {
				t.Fatalf("grace transition = %s, want available", got)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("grace did not expire")
		}
	})
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(input []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(input)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Len()
}

type blockingPartialWriter struct {
	mu         sync.Mutex
	b          bytes.Buffer
	firstWrite chan struct{}
	resume     chan struct{}
	once       sync.Once
}

func newBlockingPartialWriter() *blockingPartialWriter {
	return &blockingPartialWriter{firstWrite: make(chan struct{}), resume: make(chan struct{})}
}

func (w *blockingPartialWriter) Write(input []byte) (int, error) {
	w.mu.Lock()
	count := len(input)
	if count > 2 {
		count = 2
	}
	_, err := w.b.Write(input[:count])
	w.mu.Unlock()
	w.once.Do(func() {
		close(w.firstWrite)
		<-w.resume
	})
	return count, err
}

func (w *blockingPartialWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}
