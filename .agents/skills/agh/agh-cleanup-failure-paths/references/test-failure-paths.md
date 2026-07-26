# Testing Failure Paths

Every behaviorally distinct cleanup invariant needs coverage in its canonical
suite. Use `consolidate-test-suites` first; multiple returns that exercise the
same ownership failure mode share one test. Patterns below.

## Pattern 1: Inject a failing dependency

```go
t.Run("Should release lease when registry registration fails", func(t *testing.T) {
    t.Parallel()
    fakeRegistry := &registryStub{registerErr: errors.New("boom")}
    leaseStore := newTestLeaseStore(t)
    svc := newServiceForTest(t, fakeRegistry, leaseStore)

    err := svc.StartTaskRun(ctx, taskID)
    if !errors.Is(err, errRegistry) {
        t.Fatalf("expected wrapped registry error, got %v", err)
    }
    if got := leaseStore.heldByTask(taskID); got {
        t.Fatalf("lease still held after registry failure")
    }
})
```

## Pattern 2: Cancel mid-flight and assert subprocess stops

```go
t.Run("Should stop ACP subprocess when parent context cancels", func(t *testing.T) {
    t.Parallel()
    ctx, cancel := context.WithCancel(context.Background())
    proc := startTestACPProcess(t)
    errCh := make(chan error, 1)
    go func() { errCh <- proc.Wait(ctx) }()

    cancel()
    select {
    case err := <-errCh:
        if err != nil && !errors.Is(err, context.Canceled) {
            t.Fatalf("wait: %v", err)
        }
    case <-time.After(2 * time.Second):
        t.Fatalf("process did not exit within timeout after ctx cancel")
    }
    if !proc.IsExited() {
        t.Fatalf("process group still alive")
    }
})
```

## Pattern 3: Force HTTP body to fail and assert drain

```go
t.Run("Should drain response body even on 5xx", func(t *testing.T) {
    t.Parallel()
    body := &readerStub{bytes: bytes.Repeat([]byte("x"), 8192)}
    resp := &http.Response{StatusCode: 500, Body: body}
    err := consume(resp)
    if err == nil {
        t.Fatalf("expected error")
    }
    if !body.closed {
        t.Fatalf("body not closed")
    }
    if body.bytesRead < len(body.bytes) {
        t.Fatalf("body not drained: read %d of %d", body.bytesRead, len(body.bytes))
    }
})
```

## Pattern 4: Restart and assert no resource leaks

For long-lived components (Manager, Scheduler, Coordinator), the canonical regression test starts the component, exercises a failure, stops it, restarts it, and asserts the second instance can claim/listen/run cleanly. Leaked locks, ports, or sockets fail this test.

## Anti-patterns (do not do this)

- Asserting only the error type without checking that resources released.
- Using `time.Sleep` to "give cleanup a chance" — race-flake guaranteed. Use synchronization or assertion-with-timeout helpers.
- Asserting only that a fake cleanup method was called instead of verifying the resource is actually released.
- Skipping cleanup tests because "make verify already runs the leak detector" — `make verify` does NOT detect lease leaks or in-memory registry leaks; only `-race` catches certain classes.
