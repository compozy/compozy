import { describe, expect, it } from "vitest";

import { parseBootstrapEvent } from "../bootstrap-event";

// Invariant: the local boot page receives only the closed JSONL vocabulary emitted by Go bootstrap.
describe("bootstrap event parser", () => {
  it("Should accept a typed bootstrap phase", () => {
    expect(
      parseBootstrapEvent(
        '{"type":"bootstrap","phase":"start","status":"retrying","message":"Retrying","attempt":2}'
      )
    ).toMatchObject({ phase: "start", status: "retrying", attempt: 2 });
  });

  it.each([
    "not-json",
    "{}",
    '{"type":"bootstrap","phase":"invented","status":"started","message":"x"}',
  ])("Should reject invalid event %s", input => expect(parseBootstrapEvent(input)).toBeNull());
});
