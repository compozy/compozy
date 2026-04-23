import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useRunEventFeed } from "./use-run-event-feed";

describe("useRunEventFeed", () => {
  it("Should create a fresh event store when the run changes", () => {
    const { result, rerender } = renderHook(({ runId }) => useRunEventFeed(runId), {
      initialProps: { runId: "run-1" },
    });

    act(() => {
      result.current.append("cursor-1", {
        kind: "run.started",
        payload: { summary: "first run" },
        run_id: "run-1",
        seq: 1,
        ts: "2026-01-01T00:00:00Z",
      });
    });

    expect(result.current.events).toHaveLength(1);
    expect(result.current.events[0]?.runId).toBe("run-1");

    rerender({ runId: "run-2" });

    expect(result.current.events).toHaveLength(0);

    act(() => {
      result.current.append("cursor-2", {
        kind: "run.started",
        payload: { summary: "second run" },
        run_id: "run-2",
        seq: 1,
        ts: "2026-01-01T00:01:00Z",
      });
    });

    expect(result.current.events).toHaveLength(1);
    expect(result.current.events[0]?.runId).toBe("run-2");
  });
});
