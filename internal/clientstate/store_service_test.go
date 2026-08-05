package clientstate

// Invariant: values, revisions, sequences, tombstones, limits, and purge remain atomic and durable.
// Owning layer: client-state service over real bbolt. Canonical suite: store_service_test.go.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestEngineShouldHonorStoreContract(t *testing.T) {
	t.Parallel()

	t.Run("Should create and read an identical first revision (UT-001)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		value := objectValue("first")
		created := applyPut(t, engine, "w1", "os_shell", "desktop", value, 0, ApplyOptions{})
		got, err := engine.Get(context.Background(), "w1", "os_shell", "desktop")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if created.Rev != 1 || created.Seq != 1 || created.Deleted {
			t.Fatalf("created = %#v, want live rev=1 seq=1", created)
		}
		if !bytes.Equal(got.Value, value) || got.Rev != created.Rev ||
			got.Seq != created.Seq || !got.UpdatedAt.Equal(created.UpdatedAt) {
			t.Fatalf("Get() = %#v, want identical envelope to %#v", got, created)
		}
	})

	t.Run("Should increment revision and replace the payload (UT-002)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("one"), 0, ApplyOptions{})
		updated := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("two"), 0, ApplyOptions{})
		if updated.Rev != 2 || updated.Seq != 2 || !bytes.Equal(updated.Value, objectValue("two")) {
			t.Fatalf("updated = %#v, want rev=2 seq=2 replacement", updated)
		}
	})

	t.Run("Should report an absent key without creating state (UT-003)", func(t *testing.T) {
		t.Parallel()
		engine, _, path := newTestEngine(t, testLimits())
		_, err := engine.Get(context.Background(), "w1", "os_shell", "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get() error = %v, want ErrNotFound", err)
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("Stat(database) error = %v", statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("database mode = %o, want 600", info.Mode().Perm())
		}
	})

	t.Run("Should accept a matching compare and swap revision (UT-004)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		first := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("one"), 0, ApplyOptions{})
		second := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("two"), first.Rev, ApplyOptions{})
		if second.Rev != first.Rev+1 {
			t.Fatalf("second.Rev = %d, want %d", second.Rev, first.Rev+1)
		}
	})

	t.Run("Should reject stale compare and swap without changing state (UT-005)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		first := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("one"), 0, ApplyOptions{})
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("two"), first.Rev, ApplyOptions{})
		_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpPut, Key: "desktop", Value: objectValue("stale"), IfRev: first.Rev,
		}}, ApplyOptions{})
		if !errors.Is(err, ErrRevConflict) {
			t.Fatalf("Apply() error = %v, want ErrRevConflict", err)
		}
		got, getErr := engine.Get(context.Background(), "w1", "os_shell", "desktop")
		if getErr != nil {
			t.Fatalf("Get() error = %v", getErr)
		}
		if got.Rev != 2 || !bytes.Equal(got.Value, objectValue("two")) {
			t.Fatalf("Get() = %#v, want unchanged rev=2 value=two", got)
		}
		if engine.Metrics().RevConflicts != 1 {
			t.Fatalf("RevConflicts = %d, want 1", engine.Metrics().RevConflicts)
		}
	})

	t.Run("Should accept the exact payload cap and reject one byte over (UT-006)", func(t *testing.T) {
		t.Parallel()
		const capBytes = 256 * 1024
		engine, _, _ := newTestEngine(t, Limits{MaxValueBytes: capBytes, MaxKeysPerWorkspace: 4})
		exact := []byte(`{"v":"` + strings.Repeat("x", capBytes-len(`{"v":""}`)) + `"}`)
		if len(exact) != capBytes {
			t.Fatalf("test payload size = %d, want %d", len(exact), capBytes)
		}
		applyPut(t, engine, "w1", "os_shell", "exact", exact, 0, ApplyOptions{})
		tooLarge := append(append([]byte(nil), exact[:len(exact)-2]...), 'x', '"', '}')
		_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpPut, Key: "large", Value: tooLarge,
		}}, ApplyOptions{})
		if !errors.Is(err, ErrValueTooLarge) {
			t.Fatalf("Apply() error = %v, want ErrValueTooLarge", err)
		}
	})

	t.Run("Should count retained key identities across domains and tombstones (UT-007)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, Limits{MaxValueBytes: 1024, MaxKeysPerWorkspace: 512})
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("one"), 0, ApplyOptions{})
		deleted := applyDelete(t, engine, "w1", "os_shell", "desktop", 0)
		ops := make([]Op, 511)
		for index := range ops {
			ops[index] = Op{
				Kind:  OpPut,
				Key:   fmt.Sprintf("key:%03d", index),
				Value: objectValue("retained"),
			}
		}
		if _, err := engine.Apply(context.Background(), "w1", "other", ops, ApplyOptions{}); err != nil {
			t.Fatalf("Apply(511 retained identities) error = %v", err)
		}
		_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpPut, Key: "key:512", Value: objectValue("over-quota"),
		}}, ApplyOptions{})
		if !errors.Is(err, ErrKeyQuota) {
			t.Fatalf("Apply(513th identity) error = %v, want ErrKeyQuota", err)
		}
		recreated := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("again"), deleted.Rev, ApplyOptions{})
		if recreated.Rev != deleted.Rev+1 {
			t.Fatalf("recreated.Rev = %d, want %d", recreated.Rev, deleted.Rev+1)
		}
		applyPut(t, engine, "w1", "other", "key:000", objectValue("updated"), 0, ApplyOptions{})
	})

	t.Run("Should reject malformed domains (UT-008)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		_, err := engine.Apply(context.Background(), "w1", "OS Shell", []Op{{
			Kind: OpPut, Key: "desktop", Value: objectValue("value"),
		}}, ApplyOptions{})
		if !errors.Is(err, ErrInvalidDomain) {
			t.Fatalf("Apply() error = %v, want ErrInvalidDomain", err)
		}
	})

	t.Run("Should reject slash and overlength keys (UT-009)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		for _, key := range []string{"bad/key", strings.Repeat("k", 129)} {
			_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
				Kind: OpPut, Key: key, Value: objectValue("value"),
			}}, ApplyOptions{})
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Apply(key=%q) error = %v, want ErrInvalidKey", key, err)
			}
		}
	})

	t.Run("Should list one domain in stable key order (UT-010)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		applyPut(t, engine, "w1", "os_shell", "win:b", objectValue("b"), 0, ApplyOptions{})
		applyPut(t, engine, "w1", "other", "hidden", objectValue("other"), 0, ApplyOptions{})
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("desktop"), 0, ApplyOptions{})
		entries, err := engine.List(context.Background(), "w1", "os_shell")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(entries) != 2 || entries[0].Key != "desktop" || entries[1].Key != "win:b" {
			t.Fatalf("List() = %#v, want desktop then win:b", entries)
		}
		if entries[0].Rev != 1 || !bytes.Equal(entries[0].Value, objectValue("desktop")) ||
			entries[1].Rev != 1 || !bytes.Equal(entries[1].Value, objectValue("b")) {
			t.Fatalf("List() values/revisions = %#v, want both first revisions and original values", entries)
		}
	})

	t.Run("Should log apply envelopes without payload contents", func(t *testing.T) {
		t.Parallel()
		resolver := newFakeWorkspaceResolver()
		resolver.register("w1", "generation-1")
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		engine, err := Open(context.Background(), DatabasePath(t.TempDir()), resolver, testLimits(), WithLogger(logger))
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		t.Cleanup(func() {
			if err := engine.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		const secret = "payload-must-not-appear-in-logs"
		applyPut(
			t,
			engine,
			"w1",
			"os_shell",
			"desktop",
			objectValue(secret),
			0,
			ApplyOptions{Origin: "conn-a"},
		)
		output := logs.String()
		for _, field := range []string{
			`"component":"clientstate"`,
			`"msg":"clientstate.apply"`,
			`"workspace":"w1"`,
			`"key":"desktop"`,
			`"rev":1`,
			`"seq":1`,
			`"conn_id":"conn-a"`,
		} {
			if !strings.Contains(output, field) {
				t.Fatalf("apply log = %s, want field %s", output, field)
			}
		}
		if strings.Contains(output, secret) {
			t.Fatalf("apply log exposed payload contents: %s", output)
		}
	})

	t.Run("Should preserve tombstone revision across delete and recreate (UT-072)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		first := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("first"), 0, ApplyOptions{})
		deleted := applyDelete(t, engine, "w1", "os_shell", "desktop", first.Rev)
		if !deleted.Deleted || deleted.Rev != 2 {
			t.Fatalf("deleted = %#v, want tombstone rev=2", deleted)
		}
		if entries, err := engine.List(context.Background(), "w1", "os_shell"); err != nil || len(entries) != 0 {
			t.Fatalf("List() = %#v, %v; want no tombstones", entries, err)
		}
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		if subscription.AsOfSeq() != deleted.Seq || len(subscription.Snapshot()) != 0 {
			t.Fatalf(
				"Watch() fence/snapshot = %d/%#v, want tombstone fence %d and empty snapshot",
				subscription.AsOfSeq(),
				subscription.Snapshot(),
				deleted.Seq,
			)
		}
		if err := subscription.Close(); err != nil {
			t.Fatalf("Subscription.Close() error = %v", err)
		}
		_, err = engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpPut, Key: "desktop", Value: objectValue("stale"), IfRev: first.Rev,
		}}, ApplyOptions{})
		if !errors.Is(err, ErrRevConflict) {
			t.Fatalf("Apply(stale recreate) error = %v, want ErrRevConflict", err)
		}
		recreated := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("again"), deleted.Rev, ApplyOptions{})
		if recreated.Rev != 3 {
			t.Fatalf("recreated.Rev = %d, want 3", recreated.Rev)
		}
	})

	t.Run("Should atomically roll back an invalid batch and sequence a valid batch (UT-073)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		seed := applyPut(t, engine, "w1", "os_shell", "seed", objectValue("seed"), 0, ApplyOptions{})
		_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{
			{Kind: OpPut, Key: "one", Value: objectValue("one")},
			{Kind: OpPut, Key: "two", Value: objectValue("two")},
			{Kind: OpPut, Key: "seed", Value: objectValue("stale"), IfRev: seed.Rev + 1},
		}, ApplyOptions{})
		if !errors.Is(err, ErrRevConflict) {
			t.Fatalf("Apply(invalid batch) error = %v, want ErrRevConflict", err)
		}
		for _, key := range []string{"one", "two"} {
			if _, getErr := engine.Get(context.Background(), "w1", "os_shell", key); !errors.Is(getErr, ErrNotFound) {
				t.Fatalf("Get(%s) error = %v, want ErrNotFound after rollback", key, getErr)
			}
		}
		results, applyErr := engine.Apply(context.Background(), "w1", "os_shell", []Op{
			{Kind: OpPut, Key: "one", Value: objectValue("one")},
			{Kind: OpPut, Key: "two", Value: objectValue("two")},
			{Kind: OpPut, Key: "seed", Value: objectValue("updated"), IfRev: seed.Rev},
		}, ApplyOptions{})
		if applyErr != nil {
			t.Fatalf("Apply(valid batch) error = %v", applyErr)
		}
		if len(results) != 3 || results[0].Seq != 2 || results[1].Seq != 3 || results[2].Seq != 4 {
			t.Fatalf("Apply(valid batch) = %#v, want seqs 2,3,4", results)
		}
	})

	t.Run("Should reject invalid JSON objects atomically (UT-087)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		for _, value := range [][]byte{[]byte(`{"broken":`), []byte(`[]`), []byte(`"scalar"`), []byte(`null`)} {
			_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{
				{Kind: OpPut, Key: "valid", Value: objectValue("valid")},
				{Kind: OpPut, Key: "invalid", Value: value},
			}, ApplyOptions{})
			if !errors.Is(err, ErrInvalidValue) {
				t.Fatalf("Apply(value=%q) error = %v, want ErrInvalidValue", value, err)
			}
			_, getErr := engine.Get(context.Background(), "w1", "os_shell", "valid")
			if !errors.Is(getErr, ErrNotFound) {
				t.Fatalf("Get(valid) error = %v, want rollback", getErr)
			}
		}
	})

	t.Run("Should reject an empty apply without advancing state (UT-088)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		_, err := engine.Apply(context.Background(), "w1", "os_shell", nil, ApplyOptions{})
		if !errors.Is(err, ErrEmptyApply) {
			t.Fatalf("Apply() error = %v, want ErrEmptyApply", err)
		}
		subscription, watchErr := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if watchErr != nil {
			t.Fatalf("Watch() error = %v", watchErr)
		}
		if subscription.AsOfSeq() != 0 || len(subscription.Snapshot()) != 0 {
			t.Fatalf("Watch() fence/snapshot = %d/%#v, want 0/empty", subscription.AsOfSeq(), subscription.Snapshot())
		}
	})

	t.Run("Should reject invalid delete and operation shapes without state", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpDelete, Key: "missing",
		}}, ApplyOptions{})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Apply(delete missing) error = %v, want ErrNotFound", err)
		}
		_, err = engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpDelete, Key: "missing", IfRev: 7,
		}}, ApplyOptions{})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Apply(delete missing with CAS) error = %v, want ErrNotFound", err)
		}
		_, err = engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpDelete, Key: "missing", Value: objectValue("invalid"),
		}}, ApplyOptions{})
		if !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("Apply(delete with value) error = %v, want ErrInvalidValue", err)
		}
		_, err = engine.Apply(context.Background(), "w1", "os_shell", []Op{{
			Kind: OpKind(99), Key: "missing",
		}}, ApplyOptions{})
		if err == nil {
			t.Fatal("Apply(unknown kind) error = nil, want validation failure")
		}
		entries, listErr := engine.List(context.Background(), "w1", "os_shell")
		if listErr != nil || len(entries) != 0 {
			t.Fatalf("List() = %#v, %v; want empty after rejected mutations", entries, listErr)
		}
	})
}

func TestEngineShouldPersistAndPurgeWorkspaceState(t *testing.T) {
	t.Parallel()

	t.Run("Should restore a staged purge when workspace deletion rolls back", func(t *testing.T) {
		t.Parallel()
		engine, resolver, _ := newTestEngine(t, testLimits())
		created := applyPut(
			t,
			engine,
			"w1",
			"os_shell",
			"desktop",
			objectValue("preserved"),
			0,
			ApplyOptions{},
		)
		resolver.markDeleting("w1")
		preparation, err := engine.PrepareWorkspacePurge(context.Background(), "w1")
		if err != nil {
			t.Fatalf("PrepareWorkspacePurge() error = %v", err)
		}
		if err := preparation.Rollback(context.Background()); err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}
		resolver.register("w1", "w1-generation-1")
		preserved, err := engine.Get(context.Background(), "w1", "os_shell", "desktop")
		if err != nil {
			t.Fatalf("Get(after rollback) error = %v", err)
		}
		if preserved.Rev != created.Rev || preserved.Seq != created.Seq ||
			!bytes.Equal(preserved.Value, created.Value) {
			t.Fatalf("Get(after rollback) = %#v, want %#v", preserved, created)
		}
		updated := applyPut(
			t,
			engine,
			"w1",
			"os_shell",
			"desktop",
			objectValue("continued"),
			created.Rev,
			ApplyOptions{},
		)
		if updated.Rev != created.Rev+1 || updated.Seq != created.Seq+1 {
			t.Fatalf("updated = %#v, want revision and sequence continuity", updated)
		}
	})

	t.Run("Should recover an interrupted staged purge from workspace row truth", func(t *testing.T) {
		t.Parallel()
		resolver := newFakeWorkspaceResolver()
		resolver.register("w1", "generation-1")
		path := DatabasePath(t.TempDir())
		engine, err := Open(context.Background(), path, resolver, testLimits())
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		want := applyPut(
			t,
			engine,
			"w1",
			"os_shell",
			"desktop",
			objectValue("recover"),
			0,
			ApplyOptions{},
		)
		resolver.markDeleting("w1")
		if _, err := engine.PrepareWorkspacePurge(context.Background(), "w1"); err != nil {
			t.Fatalf("PrepareWorkspacePurge() error = %v", err)
		}
		if err := engine.Close(); err != nil {
			t.Fatalf("Close(staged) error = %v", err)
		}

		resolver.register("w1", "generation-1")
		recovered, err := Open(context.Background(), path, resolver, testLimits())
		if err != nil {
			t.Fatalf("Open(recover existing workspace) error = %v", err)
		}
		got, err := recovered.Get(context.Background(), "w1", "os_shell", "desktop")
		if err != nil {
			t.Fatalf("Get(recovered) error = %v", err)
		}
		if got.Rev != want.Rev || got.Seq != want.Seq || !bytes.Equal(got.Value, want.Value) {
			t.Fatalf("Get(recovered) = %#v, want %#v", got, want)
		}

		resolver.markDeleting("w1")
		if _, err := recovered.PrepareWorkspacePurge(context.Background(), "w1"); err != nil {
			t.Fatalf("PrepareWorkspacePurge(second) error = %v", err)
		}
		if err := recovered.Close(); err != nil {
			t.Fatalf("Close(second staged) error = %v", err)
		}
		resolver.remove("w1")
		committed, err := Open(context.Background(), path, resolver, testLimits())
		if err != nil {
			t.Fatalf("Open(recover deleted workspace) error = %v", err)
		}
		t.Cleanup(func() {
			if err := committed.Close(); err != nil {
				t.Errorf("Close(committed recovery) error = %v", err)
			}
		})
		resolver.register("w1", "generation-2")
		if entries, err := committed.List(context.Background(), "w1", "os_shell"); err != nil || len(entries) != 0 {
			t.Fatalf("List(recreated) = %#v, %v; want empty", entries, err)
		}
	})

	t.Run("Should cancel open-time purge recovery and close the store", func(t *testing.T) {
		t.Parallel()
		base := newFakeWorkspaceResolver()
		base.register("w1", "generation-1")
		path := DatabasePath(t.TempDir())
		engine, err := Open(context.Background(), path, base, testLimits())
		if err != nil {
			t.Fatalf("Open(seed) error = %v", err)
		}
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("recover"), 0, ApplyOptions{})
		base.markDeleting("w1")
		if _, err := engine.PrepareWorkspacePurge(context.Background(), "w1"); err != nil {
			t.Fatalf("PrepareWorkspacePurge() error = %v", err)
		}
		if err := engine.Close(); err != nil {
			t.Fatalf("Close(seed) error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		resolver := &cancelableRecoveryResolver{
			base:    base,
			entered: make(chan struct{}),
		}
		result := make(chan error, 1)
		go func() {
			recovered, openErr := Open(ctx, path, resolver, testLimits())
			if recovered != nil {
				openErr = errors.Join(openErr, recovered.Close())
			}
			result <- openErr
		}()
		select {
		case <-resolver.entered:
		case <-time.After(2 * time.Second):
			t.Fatal("Open() did not reach staged-purge recovery")
		}
		cancel()
		select {
		case openErr := <-result:
			if !errors.Is(openErr, context.Canceled) {
				t.Fatalf("Open(canceled recovery) error = %v, want context.Canceled", openErr)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Open() did not return after recovery cancellation")
		}

		db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
		if err != nil {
			t.Fatalf("bolt.Open(after canceled recovery) error = %v, want released store", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("DB.Close(after canceled recovery) error = %v", err)
		}
	})

	t.Run("Should purge one deleting workspace and close only its subscriptions (UT-017)", func(t *testing.T) {
		t.Parallel()
		engine, resolver, _ := newTestEngine(t, testLimits())
		applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("w1"), 0, ApplyOptions{})
		applyPut(t, engine, "w1", "other", "hidden", objectValue("w1-other"), 0, ApplyOptions{})
		applyPut(t, engine, "w2", "os_shell", "desktop", objectValue("w2"), 0, ApplyOptions{})
		w1Subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(w1) error = %v", err)
		}
		w2Subscription, err := engine.Watch(context.Background(), "w2", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch(w2) error = %v", err)
		}
		resolver.markDeleting("w1")
		if err := engine.PurgeWorkspace(context.Background(), "w1"); err != nil {
			t.Fatalf("PurgeWorkspace() error = %v", err)
		}
		select {
		case _, ok := <-w1Subscription.Events():
			if ok {
				t.Fatal("w1 subscription remained open after purge")
			}
		case <-time.After(time.Second):
			t.Fatal("w1 subscription did not close after purge")
		}
		applyPut(t, engine, "w2", "os_shell", "desktop", objectValue("w2-next"), 0, ApplyOptions{})
		if event := receiveEvent(t, w2Subscription); event.Workspace != "w2" {
			t.Fatalf("w2 event workspace = %q, want w2", event.Workspace)
		}
		if err := engine.PurgeWorkspace(context.Background(), "w1"); err != nil {
			t.Fatalf("second PurgeWorkspace() error = %v", err)
		}
		resolver.register("w1", "w1-generation-2")
		for _, domain := range []string{"os_shell", "other"} {
			entries, listErr := engine.List(context.Background(), "w1", domain)
			if listErr != nil || len(entries) != 0 {
				t.Fatalf("List(recreated %s) = %#v, %v; want empty", domain, entries, listErr)
			}
		}
		w2Entry, err := engine.Get(context.Background(), "w2", "os_shell", "desktop")
		if err != nil || !bytes.Equal(w2Entry.Value, objectValue("w2-next")) {
			t.Fatalf("Get(w2) = %#v, %v; want untouched w2-next", w2Entry, err)
		}
	})

	t.Run("Should preserve entries and revisions across reopen (UT-018)", func(t *testing.T) {
		t.Parallel()
		resolver := newFakeWorkspaceResolver()
		resolver.register("w1", "generation-1")
		path := filepath.Join(t.TempDir(), "state", DatabaseName)
		engine, err := Open(context.Background(), path, resolver, testLimits())
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		want := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("durable"), 0, ApplyOptions{})
		if err := engine.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		reopened, err := Open(context.Background(), path, resolver, testLimits())
		if err != nil {
			t.Fatalf("Open(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		got, err := reopened.Get(context.Background(), "w1", "os_shell", "desktop")
		if err != nil {
			t.Fatalf("Get(reopened) error = %v", err)
		}
		if got.Rev != want.Rev || got.Seq != want.Seq || !bytes.Equal(got.Value, want.Value) {
			t.Fatalf("Get(reopened) = %#v, want %#v", got, want)
		}
		updated := applyPut(
			t,
			reopened,
			"w1",
			"os_shell",
			"desktop",
			objectValue("durable-next"),
			want.Rev,
			ApplyOptions{},
		)
		if updated.Rev != 2 || updated.Seq != 2 {
			t.Fatalf("Apply(reopened) = %#v, want durable rev=2 seq=2", updated)
		}
	})

	t.Run("Should hard-cut an unversioned store and preserve only current-contract writes", func(t *testing.T) {
		t.Parallel()
		resolver := newFakeWorkspaceResolver()
		resolver.register("w1", "generation-1")
		path := filepath.Join(t.TempDir(), "state", DatabaseName)
		seedUnversionedStore(t, path)
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))

		engine, err := Open(context.Background(), path, resolver, testLimits(), WithLogger(logger))
		if err != nil {
			t.Fatalf("Open(unversioned) error = %v", err)
		}
		_, err = engine.Get(
			context.Background(),
			"w1",
			"window_manager",
			"snapshot",
		)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(pre-contract snapshot) error = %v, want ErrNotFound", err)
		}
		created := applyPut(
			t,
			engine,
			"w1",
			"window_manager",
			"snapshot",
			objectValue("current"),
			0,
			ApplyOptions{},
		)
		if created.Rev != 1 || created.Seq != 1 {
			t.Fatalf("Apply(after hard cut) = %#v, want rev=1 seq=1", created)
		}
		if err := engine.Close(); err != nil {
			t.Fatalf("Close(after hard cut) error = %v", err)
		}

		reopened, err := Open(context.Background(), path, resolver, testLimits())
		if err != nil {
			t.Fatalf("Open(current contract) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(); err != nil {
				t.Errorf("Close(reopened current contract) error = %v", err)
			}
		})
		got, err := reopened.Get(context.Background(), "w1", "window_manager", "snapshot")
		if err != nil {
			t.Fatalf("Get(reopened current contract) error = %v", err)
		}
		if got.Rev != created.Rev || got.Seq != created.Seq || !bytes.Equal(got.Value, created.Value) {
			t.Fatalf("Get(reopened current contract) = %#v, want %#v", got, created)
		}
		if !strings.Contains(logs.String(), `"msg":"clientstate.store.hard_cut"`) {
			t.Fatalf("Open(unversioned) logs = %s, want hard-cut event", logs.String())
		}
	})

	t.Run("Should reject an ahead store format without discarding its contents", func(t *testing.T) {
		t.Parallel()
		resolver := newFakeWorkspaceResolver()
		resolver.register("w1", "generation-1")
		path := filepath.Join(t.TempDir(), "state", DatabaseName)
		seedAheadStore(t, path)

		engine, err := Open(context.Background(), path, resolver, testLimits())
		if engine != nil || !errors.Is(err, ErrStoreFormatTooNew) {
			t.Fatalf("Open(ahead format) = %v, %v; want nil engine and ahead-version error", engine, err)
		}
		formatErr, ok := errors.AsType[*StoreFormatTooNewError](err)
		if !ok || formatErr.Version != stateStoreFormatVersion+1 ||
			formatErr.SupportedVersion != stateStoreFormatVersion {
			t.Fatalf("Open(ahead format) error = %#v, want incompatible version details", formatErr)
		}
		db, err := bolt.Open(path, 0o600, nil)
		if err != nil {
			t.Fatalf("bolt.Open(ahead verification) error = %v", err)
		}
		if err := db.View(func(tx *bolt.Tx) error {
			if tx.Bucket(workspaceBucketName("w1")) == nil {
				return errors.New("ahead workspace bucket was discarded")
			}
			return nil
		}); err != nil {
			closeErr := db.Close()
			t.Fatalf("verify ahead store error = %v; close error = %v", err, closeErr)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close(ahead verification) error = %v", err)
		}
	})
}

func seedUnversionedStore(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(legacy store) error = %v", err)
	}
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatalf("bolt.Open(legacy store) error = %v", err)
	}
	encoded, err := encodeEntry(Entry{
		Domain: "window_manager", Key: "snapshot", Value: objectValue("obsolete"),
		Rev: 19, Seq: 19, UpdatedAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		closeErr := db.Close()
		t.Fatalf("encodeEntry(legacy store) error = %v; close error = %v", err, closeErr)
	}
	sequence, err := encodeSequence(19)
	if err != nil {
		closeErr := db.Close()
		t.Fatalf("encodeSequence(legacy store) error = %v; close error = %v", err, closeErr)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		workspace, err := tx.CreateBucket(workspaceBucketName("w1"))
		if err != nil {
			return fmt.Errorf("create legacy workspace bucket: %w", err)
		}
		if err := workspace.Put(sequenceKey, sequence); err != nil {
			return fmt.Errorf("put legacy sequence: %w", err)
		}
		domain, err := workspace.CreateBucket([]byte("window_manager"))
		if err != nil {
			return fmt.Errorf("create legacy domain bucket: %w", err)
		}
		if err := domain.Put([]byte("snapshot"), encoded); err != nil {
			return fmt.Errorf("put legacy snapshot: %w", err)
		}
		return nil
	}); err != nil {
		closeErr := db.Close()
		t.Fatalf("seed legacy store error = %v; close error = %v", err, closeErr)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(legacy store) error = %v", err)
	}
}

type cancelableRecoveryResolver struct {
	base    *fakeWorkspaceResolver
	entered chan struct{}
}

func (r *cancelableRecoveryResolver) ResolveWorkspace(
	ctx context.Context,
	_ WorkspaceID,
) (WorkspaceGeneration, error) {
	close(r.entered)
	<-ctx.Done()
	return "", ctx.Err()
}

func (r *cancelableRecoveryResolver) ResolveWorkspaceForPurge(
	ctx context.Context,
	workspaceID WorkspaceID,
) (WorkspaceGeneration, error) {
	return r.base.ResolveWorkspaceForPurge(ctx, workspaceID)
}

func seedAheadStore(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(ahead store) error = %v", err)
	}
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatalf("bolt.Open(ahead store) error = %v", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		metadata, err := tx.CreateBucket(stateStoreMetadataBucket)
		if err != nil {
			return fmt.Errorf("create ahead metadata: %w", err)
		}
		version := make([]byte, 4)
		binary.BigEndian.PutUint32(version, stateStoreFormatVersion+1)
		if err := metadata.Put(stateStoreFormatKey, version); err != nil {
			return fmt.Errorf("put ahead format version: %w", err)
		}
		if _, err := tx.CreateBucket(workspaceBucketName("w1")); err != nil {
			return fmt.Errorf("create ahead workspace bucket: %w", err)
		}
		return nil
	}); err != nil {
		closeErr := db.Close()
		t.Fatalf("seed ahead store error = %v; close error = %v", err, closeErr)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(ahead store) error = %v", err)
	}
}
