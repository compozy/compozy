import type { LoopRunEventFrame, LoopRunEventKind } from "../../types";

/**
 * Shared SSE-envelope factory for the loop lib suites (reducer + story). Keeps
 * the `LoopRunEventFrame` payload/kind wiring in one place so both suites stay
 * aligned when the envelope gains fields.
 */
export function loopEventFrame(
  kind: LoopRunEventKind,
  payload: unknown,
  seq = 1,
  overrides: Partial<LoopRunEventFrame> = {}
): LoopRunEventFrame {
  return {
    id: `ev-${seq}`,
    seq,
    kind,
    loop_run_id: "looprun_running",
    workspace_id: "ws_default",
    at: "2026-07-06T14:38:00Z",
    payload,
    ...overrides,
  };
}
