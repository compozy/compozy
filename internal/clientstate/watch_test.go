package clientstate

// Invariant: watch snapshots and events form one gap-free, ordered workspace stream without blocking writers.
// Owning layer: client-state sequencer and hub. Canonical suite: watch_test.go.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestEngineShouldPublishOrderedWorkspaceEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should publish a tombstone and hide it from reads (UT-011)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("live"), 0, ApplyOptions{})
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		deleted := applyDelete(t, engine, "w1", "os_shell", "desktop", 0)
		if _, err := engine.Get(context.Background(), "w1", "os_shell", "desktop"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get() error = %v, want ErrNotFound", err)
		}
		event := receiveEvent(t, subscription)
		if !event.Entry.Deleted || event.Entry.Rev != deleted.Rev || event.Entry.Seq != deleted.Seq {
			t.Fatalf("delete event = %#v, want tombstone %#v", event, deleted)
		}
	})

	t.Run("Should reject a stale delete and keep the live entry (UT-012)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		first := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("one"), 0, ApplyOptions{})
		second := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("two"), first.Rev, ApplyOptions{})
		_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpDelete, Key: "desktop", IfRev: first.Rev,
		}}, ApplyOptions{})
		if !errors.Is(err, ErrRevConflict) {
			t.Fatalf("Apply(delete) error = %v, want ErrRevConflict", err)
		}
		got, getErr := engine.Get(context.Background(), "w1", "os_shell", "desktop")
		if getErr != nil {
			t.Fatalf("Get() error = %v", getErr)
		}
		if got.Rev != second.Rev || !bytes.Equal(got.Value, second.Value) {
			t.Fatalf("Get() = %#v, want unchanged %#v", got, second)
		}
	})

	t.Run("Should deliver put put delete in increasing revision order (UT-013)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("one"), 0, ApplyOptions{})
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("two"), 0, ApplyOptions{})
		applyDelete(t, engine, "w1", "os_shell", "desktop", 0)
		for wantRev := uint64(1); wantRev <= 3; wantRev++ {
			event := receiveEvent(t, subscription)
			if event.Entry.Rev != wantRev || event.Entry.Seq != wantRev {
				t.Fatalf("event = %#v, want rev/seq %d", event, wantRev)
			}
		}
	})

	t.Run("Should fan out the same commit to three subscribers (UT-014)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		subscriptions := make([]Subscription, 3)
		for index := range subscriptions {
			var err error
			subscriptions[index], err = engine.Watch(context.Background(), "w1", []string{"os_shell"})
			if err != nil {
				t.Fatalf("Watch(%d) error = %v", index, err)
			}
		}
		want := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("fanout"), 0, ApplyOptions{})
		for index, subscription := range subscriptions {
			event := receiveEvent(t, subscription)
			if event.Entry.Seq != want.Seq || !bytes.Equal(event.Entry.Value, want.Value) {
				t.Fatalf("subscriber %d event = %#v, want %#v", index, event, want)
			}
		}
	})

	t.Run("Should evict only the slow subscriber without blocking writes (UT-015)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, Limits{MaxValueBytes: 1024, MaxKeysPerWorkspace: 4})
		slow, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(slow) error = %v", err)
		}
		healthy, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(healthy) error = %v", err)
		}
		healthyEvents := make([]Event, 0, SubscriptionBufferSize+1)
		for index := range SubscriptionBufferSize + 1 {
			applyPut(t, engine, "w1", "os_shell", "desktop", objectValue(fmt.Sprintf("%d", index)), 0, ApplyOptions{})
			healthyEvents = append(healthyEvents, receiveEvent(t, healthy))
		}
		waitForSubscriptionError(t, slow, ErrSlowConsumer)
		if len(healthyEvents) != SubscriptionBufferSize+1 {
			t.Fatalf("healthy subscriber received %d events, want %d", len(healthyEvents), SubscriptionBufferSize+1)
		}
		if healthyEvents[len(healthyEvents)-1].Entry.Seq != SubscriptionBufferSize+1 {
			t.Fatalf(
				"healthy last seq = %d, want %d",
				healthyEvents[len(healthyEvents)-1].Entry.Seq,
				SubscriptionBufferSize+1,
			)
		}
		metrics := engine.Metrics()
		if metrics.SlowConsumerEvictions != 1 || metrics.ActiveSubscriptions != 1 {
			t.Fatalf("metrics = %#v, want one eviction and one active subscription", metrics)
		}
	})

	t.Run("Should preserve mutation origin on published events (UT-016)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("origin"), 0, ApplyOptions{Origin: "conn-a"})
		if event := receiveEvent(t, subscription); event.Origin != "conn-a" {
			t.Fatalf("event.Origin = %q, want conn-a", event.Origin)
		}
	})

	t.Run("Should never leak events across workspaces (UT-021)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		w1, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(w1) error = %v", err)
		}
		w2, err := engine.Watch(context.Background(), "w2", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(w2) error = %v", err)
		}
		applyPut(t, engine, "w2", "os_shell", "desktop", objectValue("w2"), 0, ApplyOptions{})
		if event := receiveEvent(t, w2); event.Workspace != "w2" {
			t.Fatalf("w2 event = %#v, want w2", event)
		}
		select {
		case event := <-w1.Events():
			t.Fatalf("w1 received cross-workspace event %#v", event)
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("Should filter selected domains and include all domains when omitted", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("desktop"), 0, ApplyOptions{})
		applyPut(t, engine, "w1", "other", "other", objectValue("other"), 0, ApplyOptions{})
		filtered, err := engine.Watch(context.Background(), "w1", []string{"os_shell", "os_shell"})
		if err != nil {
			t.Fatalf("Watch(filtered) error = %v", err)
		}
		if snapshot := filtered.Snapshot(); len(snapshot) != 1 || snapshot[0].Domain != "os_shell" {
			t.Fatalf("filtered snapshot = %#v, want os_shell only", snapshot)
		}
		all, err := engine.Watch(context.Background(), "w1", nil)
		if err != nil {
			t.Fatalf("Watch(all) error = %v", err)
		}
		snapshot := all.Snapshot()
		if len(snapshot) != 2 || snapshot[0].Domain != "os_shell" || snapshot[1].Domain != "other" {
			t.Fatalf("all-domain snapshot = %#v, want two domains in stable order", snapshot)
		}
		if err := filtered.Close(); err != nil {
			t.Fatalf("filtered.Close() error = %v", err)
		}
		if metrics := engine.Metrics(); metrics.ActiveSubscriptions != 1 {
			t.Fatalf("ActiveSubscriptions = %d, want 1 after explicit close", metrics.ActiveSubscriptions)
		}
	})
}

func TestEngineShouldLinearizeConcurrentCommitsAndWatch(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver identical total order to independent subscribers (UT-070)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, Limits{MaxValueBytes: 1024, MaxKeysPerWorkspace: 16})
		first, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(first) error = %v", err)
		}
		second, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(second) error = %v", err)
		}
		const writers = 4
		const writesPerWriter = 25
		const total = writers * writesPerWriter
		committed := make([]Entry, total)
		var waitGroup sync.WaitGroup
		for writer := range writers {
			waitGroup.Go(func() {
				for write := range writesPerWriter {
					results, applyErr := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
						Kind:  OpPut,
						Key:   fmt.Sprintf("writer:%d", writer),
						Value: objectValue(fmt.Sprintf("%d", write)),
					}}, ApplyOptions{})
					if applyErr != nil {
						t.Errorf("Apply(writer=%d write=%d) error = %v", writer, write, applyErr)
						return
					}
					if len(results) != 1 || results[0].Seq == 0 || results[0].Seq > total {
						t.Errorf(
							"Apply(writer=%d write=%d) results = %#v, want one bounded sequence",
							writer,
							write,
							results,
						)
						return
					}
					committed[results[0].Seq-1] = results[0]
				}
			})
		}
		waitGroup.Wait()
		if t.Failed() {
			t.FailNow()
		}
		firstEvents := receiveEvents(t, first, total)
		secondEvents := receiveEvents(t, second, total)
		for index := range total {
			wantSeq := uint64(index + 1)
			if firstEvents[index].Entry.Seq != wantSeq || secondEvents[index].Entry.Seq != wantSeq {
				t.Fatalf(
					"event %d seqs = %d/%d, want %d",
					index,
					firstEvents[index].Entry.Seq,
					secondEvents[index].Entry.Seq,
					wantSeq,
				)
			}
			if firstEvents[index].Entry.Key != secondEvents[index].Entry.Key ||
				!bytes.Equal(firstEvents[index].Entry.Value, secondEvents[index].Entry.Value) {
				t.Fatalf("event %d streams differ: %#v / %#v", index, firstEvents[index], secondEvents[index])
			}
			if firstEvents[index].Entry.Key != committed[index].Key ||
				!bytes.Equal(firstEvents[index].Entry.Value, committed[index].Value) {
				t.Fatalf(
					"event %d = %#v, want commit-order entry %#v",
					index,
					firstEvents[index].Entry,
					committed[index],
				)
			}
		}
	})

	t.Run("Should fence a subscription racing a write burst without gaps (UT-071)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, Limits{MaxValueBytes: 1024, MaxKeysPerWorkspace: 128})
		for index := range 10 {
			applyPut(
				t,
				engine,
				"w1",
				"os_shell",
				fmt.Sprintf("seed:%02d", index),
				objectValue("seed"),
				0,
				ApplyOptions{},
			)
		}
		start := make(chan struct{})
		writerDone := make(chan error, 1)
		go func() {
			<-start
			for index := range 50 {
				_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
					Kind: OpPut, Key: fmt.Sprintf("burst:%02d", index), Value: objectValue("burst"),
				}}, ApplyOptions{})
				if err != nil {
					writerDone <- err
					return
				}
			}
			writerDone <- nil
		}()
		close(start)
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		if err := <-writerDone; err != nil {
			t.Fatalf("writer error = %v", err)
		}
		marker := applyPut(t, engine, "w1", "os_shell", "after-snapshot", objectValue("marker"), 0, ApplyOptions{})
		asOfSeq := subscription.AsOfSeq()
		if asOfSeq < 10 || asOfSeq > 60 {
			t.Fatalf("AsOfSeq() = %d, want within [10,60]", asOfSeq)
		}
		for _, entry := range subscription.Snapshot() {
			if entry.Key == marker.Key {
				t.Fatalf("snapshot contains key %q created after its fence", marker.Key)
			}
			if entry.Seq > asOfSeq {
				t.Fatalf("snapshot entry seq = %d above fence %d", entry.Seq, asOfSeq)
			}
		}
		events := receiveEvents(t, subscription, int(marker.Seq-asOfSeq))
		markerDelivered := false
		for index, event := range events {
			wantSeq := asOfSeq + uint64(index) + 1
			if event.Entry.Seq != wantSeq {
				t.Fatalf("event %d seq = %d, want %d", index, event.Entry.Seq, wantSeq)
			}
			if event.Entry.Key == marker.Key {
				markerDelivered = true
			}
		}
		if !markerDelivered {
			t.Fatalf("events = %#v, want key %q created after snapshot", events, marker.Key)
		}
	})
}

func receiveEvents(t *testing.T, subscription Subscription, count int) []Event {
	t.Helper()
	events := make([]Event, 0, count)
	for len(events) < count {
		events = append(events, receiveEvent(t, subscription))
	}
	return events
}
