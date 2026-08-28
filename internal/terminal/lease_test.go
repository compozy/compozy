package terminal

// Suite: terminal control lease.
// Invariant: exactly one actor can deliver whole input, and ownership changes fence stale generations atomically.
// Boundary IN: attach/takeover/yield/write actions. Boundary OUT: bytes handed to the process writer.

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLeaseMachineContract(t *testing.T) {
	t.Parallel()

	agent := Actor{
		Kind:       ActorKindAgent,
		ID:         "agent",
		ProfileID:  "profile-a",
		SessionID:  "session",
		RunID:      "run",
		Generation: 3,
	}
	humanA := Actor{Kind: ActorKindHuman, ID: "human-a", ProfileID: "profile-a"}
	humanB := Actor{Kind: ActorKindHuman, ID: "human-b", ProfileID: "profile-a"}

	t.Run("Should reserve write-owner-held for a real competing controller", func(t *testing.T) {
		t.Parallel()
		lease := newLeaseMachine(Actor{}, io.Discard, time.Second, nil)
		for operation, err := range map[string]error{
			"write without lease":          lease.authorize(humanA),
			"agent takeover without owner": lease.takeover(agent, false),
			"input request without owner":  lease.withAgentController(func(Actor) error { return nil }),
		} {
			var terminalErr *Error
			if !errors.Is(err, ErrWriteLeaseRequired) || errors.As(err, &terminalErr) {
				t.Fatalf("%s error = %v, want untyped write-lease requirement", operation, err)
			}
		}
		if err := lease.claim(humanA); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("human Claim() error = %v, want unsupported", err)
		}
	})

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
		if !errors.As(failed, &terminalErr) || terminalErr.Code != "write_owner_held" ||
			terminalErr.Controller == nil ||
			terminalErr.Controller.ID != humanA.ID {
			t.Fatalf("rejected write error = %#v, want named controller", failed)
		}
		if got := output.String(); got != "accepted" {
			t.Fatalf("process bytes = %q, want accepted", got)
		}
	})

	t.Run("Should let a human take control from an agent immediately [UT-018]", func(t *testing.T) {
		t.Parallel()
		transitions := make(chan string, 1)
		lease := newLeaseMachine(
			agent,
			io.Discard,
			time.Second,
			func(from, to LeaseState, reason string, _ Actor, _ *Actor) {
				transitions <- string(from) + ">" + string(to) + ":" + reason
			},
		)
		if err := lease.takeover(humanA, false); err != nil {
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

	t.Run("Should refine the provisional operator identity without confirmation [UT-018]", func(t *testing.T) {
		t.Parallel()
		operator := Actor{Kind: ActorKindHuman, ID: OperatorActorID, ProfileID: "profile-a"}
		transitions := make(chan string, 1)
		lease := newLeaseMachine(
			operator,
			io.Discard,
			time.Second,
			func(_ LeaseState, _ LeaseState, _ string, _ Actor, controller *Actor) {
				if controller != nil {
					transitions <- controller.ID
				}
			},
		)

		if err := lease.takeover(humanA, false); err != nil {
			t.Fatalf("takeover() error = %v", err)
		}
		state, controller := lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || !sameActor(*controller, humanA) {
			t.Fatalf("lease = %s %#v, want refined human identity", state, controller)
		}
		select {
		case got := <-transitions:
			if got != humanA.ID {
				t.Fatalf("transition controller = %q, want %q", got, humanA.ID)
			}
		default:
			t.Fatal("identity refinement did not emit an ownership transition")
		}
	})

	t.Run("Should require confirmation before displacing another human [UT-018][UT-118]", func(t *testing.T) {
		t.Parallel()
		lease := newLeaseMachine(humanA, io.Discard, time.Second, nil)
		if err := lease.takeover(humanB, false); !errors.Is(err, ErrWriteOwnerHeld) {
			t.Fatalf("takeover(without confirmation) error = %v, want ErrWriteOwnerHeld", err)
		}
		state, controller := lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || controller.ID != humanA.ID {
			t.Fatalf("lease after refusal = %s %#v, want human-a", state, controller)
		}
		if err := lease.takeover(humanB, true); err != nil {
			t.Fatalf("takeover(after confirmation) error = %v", err)
		}
		state, controller = lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || controller.ID != humanB.ID {
			t.Fatalf("lease after confirmation = %s %#v, want human-b", state, controller)
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
		takeoverStarted := make(chan struct{})
		go func() {
			close(takeoverStarted)
			takeoverDone <- lease.takeover(humanA, true)
		}()
		<-takeoverStarted
		select {
		case err := <-takeoverDone:
			t.Fatalf("takeover completed before write boundary: %v", err)
		default:
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

	t.Run("Should select one winner for simultaneous human takeovers [UT-020]", func(t *testing.T) {
		t.Parallel()
		lease := newLeaseMachine(agent, io.Discard, time.Second, nil)
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, actor := range []Actor{humanA, humanB} {
			go func() {
				<-start
				results <- lease.takeover(actor, false)
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

	t.Run("Should publish rapid transitions in committed order with truthful controllers", func(t *testing.T) {
		t.Parallel()
		type observedTransition struct {
			to         LeaseState
			reason     string
			controller *Actor
		}
		firstStarted := make(chan struct{})
		releaseFirst := make(chan struct{})
		observed := make(chan observedTransition, 2)
		var transitionCount atomic.Int64
		lease := newLeaseMachine(agent, io.Discard, time.Second, func(
			_ LeaseState,
			to LeaseState,
			reason string,
			_ Actor,
			controller *Actor,
		) {
			if transitionCount.Add(1) == 1 {
				close(firstStarted)
				<-releaseFirst
			}
			observed <- observedTransition{to: to, reason: reason, controller: controller}
		})
		takeoverDone := make(chan error, 1)
		go func() { takeoverDone <- lease.takeover(humanA, false) }()
		<-firstStarted
		if err := lease.yield(humanA); err != nil {
			t.Fatalf("yield() error = %v", err)
		}
		select {
		case got := <-observed:
			t.Fatalf("transition overtook blocked predecessor: %#v", got)
		default:
		}
		close(releaseFirst)
		if err := <-takeoverDone; err != nil {
			t.Fatalf("takeover() error = %v", err)
		}
		first, second := <-observed, <-observed
		if first.to != LeaseHumanOwned || first.reason != "takeover" || first.controller == nil ||
			!sameActor(*first.controller, humanA) {
			t.Fatalf("first transition = %#v, want takeover/human-a", first)
		}
		if second.to != LeaseAgentOwned || second.reason != "yield" || second.controller == nil ||
			!sameActor(*second.controller, agent) {
			t.Fatalf("second transition = %#v, want yield/original agent", second)
		}
	})

	t.Run("Should drain the active transition and discard queued ownership changes on close", func(t *testing.T) {
		t.Parallel()
		firstStarted := make(chan struct{})
		releaseFirst := make(chan struct{})
		observed := make(chan string, 2)
		lease := newLeaseMachine(agent, io.Discard, time.Second, func(
			_ LeaseState,
			_ LeaseState,
			reason string,
			_ Actor,
			_ *Actor,
		) {
			if reason == "takeover" {
				close(firstStarted)
				<-releaseFirst
			}
			observed <- reason
		})
		takeoverDone := make(chan error, 1)
		go func() { takeoverDone <- lease.takeover(humanA, false) }()
		<-firstStarted
		if err := lease.yield(humanA); err != nil {
			t.Fatalf("yield() error = %v", err)
		}
		closeDone := make(chan struct{})
		go func() {
			lease.close()
			close(closeDone)
		}()
		deadline := time.Now().Add(time.Second)
		for {
			lease.mu.Lock()
			closed := lease.closed
			lease.mu.Unlock()
			if closed {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("lease close did not seal transitions")
			}
			runtime.Gosched()
		}
		select {
		case <-closeDone:
			t.Fatal("lease close returned before the active transition drained")
		default:
		}
		close(releaseFirst)
		if err := <-takeoverDone; err != nil {
			t.Fatalf("takeover() error = %v", err)
		}
		select {
		case <-closeDone:
		case <-time.After(time.Second):
			t.Fatal("lease close did not finish after the active transition drained")
		}
		if reason := <-observed; reason != "takeover" {
			t.Fatalf("published transition = %q, want takeover", reason)
		}
		select {
		case reason := <-observed:
			t.Fatalf("queued transition published after close: %q", reason)
		default:
		}
		lease.runEnded(agent)
		lease.runtimeRecovered(agent, agent)
		if err := lease.takeover(humanB, true); !errors.Is(err, ErrExited) {
			t.Fatalf("takeover(after close) error = %v, want ErrExited", err)
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
		lease.runEnded(agent)
		lease.runEnded(agent)
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

	t.Run("Should return an active displaced agent only on explicit human release [UT-023]", func(t *testing.T) {
		t.Parallel()
		transitions := make(chan LeaseState, 2)
		lease := newLeaseMachine(
			agent,
			io.Discard,
			time.Second,
			func(_ LeaseState, to LeaseState, _ string, _ Actor, _ *Actor) {
				transitions <- to
			},
		)
		human := Actor{Kind: ActorKindHuman, ID: "client:web", ProfileID: agent.ProfileID}
		if err := lease.takeover(human, false); err != nil {
			t.Fatalf("takeover error = %v", err)
		}
		<-transitions
		if err := lease.yield(human); err != nil {
			t.Fatalf("release error = %v", err)
		}
		state, controller := lease.snapshot()
		if state != LeaseAgentOwned || controller == nil || !sameActor(*controller, agent) {
			t.Fatalf("lease after release = %s %#v, want active agent", state, controller)
		}
		if got := <-transitions; got != LeaseAgentOwned {
			t.Fatalf("release transition = %s, want agent_owned", got)
		}
	})

	t.Run("Should keep human control when the displaced agent has ended [UT-023]", func(t *testing.T) {
		t.Parallel()
		lease := newLeaseMachine(agent, io.Discard, time.Second, nil)
		human := Actor{Kind: ActorKindHuman, ID: "client:web", ProfileID: agent.ProfileID}
		if err := lease.takeover(human, false); err != nil {
			t.Fatalf("takeover error = %v", err)
		}
		lease.runEnded(agent)
		if err := lease.yield(human); err != nil {
			t.Fatalf("release error = %v", err)
		}
		state, controller := lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || !sameActor(*controller, human) {
			t.Fatalf("lease after ended-agent release = %s %#v, want human", state, controller)
		}
	})

	t.Run("Should start grace only after the final controller attachment closes [UT-024]", func(t *testing.T) {
		t.Parallel()
		transition := make(chan LeaseState, 2)
		lease := newLeaseMachine(
			humanA,
			io.Discard,
			35*time.Millisecond,
			func(_ LeaseState, to LeaseState, _ string, _ Actor, _ *Actor) {
				transition <- to
			},
		)
		first := lease.attachWriter(humanA)
		second := lease.attachWriter(humanA)
		lease.detachWriter(first)
		lease.mu.Lock()
		graceStartedEarly := lease.timer != nil
		lease.mu.Unlock()
		if graceStartedEarly {
			t.Fatal("grace timer started while one controller attachment remained")
		}
		state, controller := lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || controller.ID != humanA.ID {
			t.Fatalf("lease after one detach = %s %#v, want human-a held", state, controller)
		}
		lease.detachWriter(second)
		lease.mu.Lock()
		graceStarted := lease.timer != nil
		lease.mu.Unlock()
		if !graceStarted {
			t.Fatal("grace timer did not start after the final controller attachment closed")
		}
		resumed := lease.attachWriter(humanA)
		lease.mu.Lock()
		graceCanceled := lease.timer == nil
		lease.mu.Unlock()
		if !graceCanceled {
			t.Fatal("reattachment did not cancel the controller grace timer")
		}
		state, controller = lease.snapshot()
		if state != LeaseHumanOwned || controller == nil || controller.ID != humanA.ID {
			t.Fatalf("lease after grace cancellation = %s %#v, want human-a held", state, controller)
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
	count := min(len(input), 2)
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
