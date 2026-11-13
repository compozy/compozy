import { describe, expect, it } from "vitest";

import { bootstrapLogRecord, parseBootstrapEvent } from "../bootstrap-event";

// Invariant: the local boot page receives only the closed JSONL vocabulary emitted by Go bootstrap.
// Owner: desktop bootstrap boundary. Canonical suite: bootstrap-event.test.ts.
describe("bootstrap event parser", () => {
  it("Should accept a typed bootstrap phase", () => {
    expect(
      parseBootstrapEvent(
        '{"type":"bootstrap","phase":"start","status":"retrying","message":"Retrying","attempt":2}'
      )
    ).toMatchObject({ phase: "start", status: "retrying", attempt: 2 });
  });

  it("Should accept typed compatibility evidence", () => {
    const event = {
      type: "bootstrap",
      phase: "ready",
      status: "failed",
      message: "The runtime is incompatible.",
      compatibility: {
        reason: "runtime_below_minimum",
        runtime: "0.2.0",
        needed: ">=0.3.0",
      },
    };
    expect(parseBootstrapEvent(JSON.stringify(event))).toMatchObject(event);
  });

  it("Should accept typed config failure evidence", () => {
    const event = {
      type: "bootstrap",
      phase: "resolve",
      status: "failed",
      message: "cli: CompozyOS configuration could not be loaded",
      classification: "config-invalid",
      config: { path: "/home/dora/.compozy/config.toml", unknown_keys: ["permissions.bogus"] },
    };
    expect(parseBootstrapEvent(JSON.stringify(event))).toMatchObject(event);
    expect(
      parseBootstrapEvent(JSON.stringify({ ...event, config: { line: 3, column: 9 } }))
    ).toMatchObject({ config: { line: 3, column: 9 } });
  });

  it.each([0, 42, 100])("Should accept replacement data-migration progress at %i percent", pct => {
    const event = {
      type: "bootstrap",
      phase: "provision",
      status: "started",
      message: `Upgrading CompozyOS data (${pct}%).`,
      resolution: "provision",
      stage: "migrate",
      pct,
    };
    expect(parseBootstrapEvent(JSON.stringify(event))).toMatchObject(event);
  });

  it("Should project data-migration progress into the bootstrap log record", () => {
    const event = parseBootstrapEvent(
      '{"type":"bootstrap","phase":"provision","status":"started","message":"Upgrading","resolution":"provision","stage":"migrate","pct":0}'
    );
    if (!event) throw new Error("expected a migration event");
    expect(bootstrapLogRecord(event, "boot-1", new Date("2026-11-04T12:00:00Z"))).toEqual({
      timestamp: "2026-11-04T12:00:00.000Z",
      level: "INFO",
      boot_id: "boot-1",
      message: "Upgrading",
      phase: "provision",
      status: "started",
      resolution: "provision",
      stage: "migrate",
      pct: 0,
    });
  });

  it.each([
    "not-json",
    "{}",
    '{"type":"bootstrap","phase":"invented","status":"started","message":"x"}',
  ])("Should reject invalid event %s", input => expect(parseBootstrapEvent(input)).toBeNull());

  it.each(["http://127.0.0.1:2123", "https://localhost:2123", "http://[::1]:2123"])(
    "Should accept ready daemon origin %s",
    origin => {
      const event = {
        type: "bootstrap",
        phase: "ready",
        status: "completed",
        message: "Ready",
        resolution: "attach",
        daemon: { origin, version: "1.0.0", pid: 42, owned: false },
      };
      expect(parseBootstrapEvent(JSON.stringify(event))).toMatchObject(event);
    }
  );

  it.each([
    null,
    [],
    { type: "other", phase: "resolve", status: "started", message: "x" },
    { type: "bootstrap", phase: 1, status: "started", message: "x" },
    { type: "bootstrap", phase: "resolve", status: 1, message: "x" },
    { type: "bootstrap", phase: "resolve", status: "invented", message: "x" },
    { type: "bootstrap", phase: "resolve", status: "started", message: 1 },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      resolution: "invented",
    },
    { type: "bootstrap", phase: "provision", status: "started", message: "x", stage: "invented" },
    {
      type: "bootstrap",
      phase: "provision",
      status: "started",
      message: "x",
      stage: "migrate",
      pct: 101,
    },
    {
      type: "bootstrap",
      phase: "provision",
      status: "started",
      message: "x",
      stage: "migrate",
      pct: 4.2,
    },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      daemon: null,
    },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      daemon: { origin: "https://example.com", version: "1", pid: 1, owned: true },
    },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      daemon: { origin: "not a URL", version: "1", pid: 1, owned: true },
    },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      daemon: { origin: "http://127.0.0.1", version: 1, pid: 1, owned: true },
    },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      daemon: { origin: "http://127.0.0.1", version: "1", pid: 0, owned: true },
    },
    {
      type: "bootstrap",
      phase: "ready",
      status: "completed",
      message: "x",
      daemon: { origin: "http://127.0.0.1", version: "1", pid: 1, owned: "yes" },
    },
    { type: "bootstrap", phase: "resolve", status: "failed", message: "x", config: null },
    { type: "bootstrap", phase: "resolve", status: "failed", message: "x", config: { path: 1 } },
    {
      type: "bootstrap",
      phase: "resolve",
      status: "failed",
      message: "x",
      config: { unknown_keys: [1] },
    },
    { type: "bootstrap", phase: "resolve", status: "failed", message: "x", config: { line: 0 } },
    { type: "bootstrap", phase: "start", status: "started", message: "x", attempt: 0 },
    { type: "bootstrap", phase: "start", status: "retrying", message: "x", backoff_ms: -1 },
    {
      type: "bootstrap",
      phase: "ready",
      status: "failed",
      message: "x",
      compatibility: { reason: "invented", runtime: "1.0.0", needed: "2.0.0" },
    },
  ])("Should reject malformed structured event %#", value => {
    expect(parseBootstrapEvent(JSON.stringify(value))).toBeNull();
  });

  // Invariant: lifecycle and wait evidence is additive. Events carrying it parse, events without
  // it keep parsing, and it reaches the bootstrap log.
  it("Should accept runtime lifecycle and mutation-wait evidence and log it", () => {
    const waiting = {
      type: "bootstrap",
      phase: "start",
      status: "started",
      message: "Another CompozyOS update or restart is in progress (pid 4242); waiting.",
      attempt: 1,
      classification: "mutation-in-progress",
      wait: { reason: "runtime_mutation", owner_pid: 4242 },
    };
    expect(parseBootstrapEvent(JSON.stringify(waiting))).toMatchObject(waiting);
    const serving = {
      type: "bootstrap",
      phase: "start",
      status: "started",
      message: "The CompozyOS runtime is starting.",
      attempt: 1,
      readiness: { phase: "serving", step: "resumeBootInterruptedSessions" },
    };
    const event = parseBootstrapEvent(JSON.stringify(serving));
    if (!event) throw new Error("expected a lifecycle event");
    expect(bootstrapLogRecord(event, "boot-1", new Date("2026-11-12T12:00:00Z"))).toMatchObject({
      phase: "start",
      readiness: { phase: "serving", step: "resumeBootInterruptedSessions" },
    });
    // A lifecycle phase this app does not know yet is still a valid event.
    const unknown = JSON.stringify({ ...serving, readiness: { phase: "warming" } });
    expect(parseBootstrapEvent(unknown)).toMatchObject({ readiness: { phase: "warming" } });
  });

  it.each([
    { readiness: "serving" },
    { readiness: { phase: "" } },
    { readiness: { phase: "serving", step: 1 } },
    { wait: { owner_pid: 4242 } },
    { wait: { reason: "runtime_mutation", owner_pid: 0 } },
    { classification: 1 },
  ])("Should reject malformed lifecycle evidence %#", extra => {
    const base = { type: "bootstrap", phase: "start", status: "started", message: "x" };
    expect(parseBootstrapEvent(JSON.stringify({ ...base, ...extra }))).toBeNull();
  });
});
