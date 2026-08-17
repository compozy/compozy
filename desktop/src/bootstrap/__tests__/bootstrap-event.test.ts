import { describe, expect, it } from "vitest";

import { parseBootstrapEvent } from "../bootstrap-event";

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
});
