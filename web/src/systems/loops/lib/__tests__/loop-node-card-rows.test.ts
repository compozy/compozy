import { describe, expect, it } from "vitest";

import type { RawLoopNode } from "../codec";
import { loopNodeCardRows } from "../loop-node-card-rows";

function raw(
  partial: Partial<RawLoopNode> & Pick<RawLoopNode, "id" | "class" | "kind">
): RawLoopNode {
  return partial;
}

describe("loopNodeCardRows", () => {
  it("Should project only declared fields and never invent joins or quarantine", () => {
    expect(loopNodeCardRows(raw({ id: "collect", class: "control", kind: "collect" }))).toEqual([
      { key: "kind", label: "kind", value: "collect" },
    ]);
    expect(
      loopNodeCardRows(
        raw({
          id: "execute",
          class: "action",
          kind: "run-agent",
          params: { agent: "implementer" },
          retry: { max_attempts: 2, backoff: { base: "30s", max: "5m" } },
          deadline: "2h",
        })
      )
    ).toEqual([
      { key: "agent", label: "agent", value: "implementer" },
      { key: "retry", label: "retry", value: "2 · 30s→5m" },
      { key: "deadline", label: "deadline", value: "2h" },
    ]);
  });

  it("Should render on_error only when route or allow_fail is declared", () => {
    expect(
      loopNodeCardRows(
        raw({ id: "run", class: "action", kind: "run-agent", on_error: { effects: [] } })
      )
    ).toEqual([]);
    expect(
      loopNodeCardRows(
        raw({
          id: "run",
          class: "action",
          kind: "run-agent",
          on_error: { route: "quarantine_path" },
        })
      )
    ).toEqual([
      {
        key: "on_error",
        label: "on_error",
        value: "route → quarantine_path",
        danger: true,
      },
    ]);
    expect(
      loopNodeCardRows(
        raw({ id: "run", class: "action", kind: "run-agent", on_error: { allow_fail: true } })
      )
    ).toEqual([{ key: "on_error", label: "on_error", value: "allow_fail", danger: true }]);
  });

  it("Should summarize wait mode, expect.required, and expiry from declared params", () => {
    expect(
      loopNodeCardRows(
        raw({
          id: "await_ack",
          class: "control",
          kind: "wait",
          params: {
            event: { kind: "hook" },
            expect: { required: ["deploy_id"] },
            expires: { after: "24h" },
          },
        })
      )
    ).toEqual([
      { key: "kind", label: "kind", value: "wait · event" },
      { key: "expect", label: "expect", value: "deploy_id" },
      { key: "expires", label: "expires", value: "24h" },
    ]);
  });
});
