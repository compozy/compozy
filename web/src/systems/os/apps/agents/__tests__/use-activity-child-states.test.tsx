// Suite: Activity child-state catalog reads
// Invariant: Activity does not probe N session catalogs or invent parked/gone
// from stop reasons. Until the daemon projects `child_state`, the map stays
// empty. Owning layer: `useActivityChildStates`. Canonical suite: this file.
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { AgentCommsScope } from "@/systems/agent-comms";

import { useActivityChildStates } from "../use-activity-child-states";

const SCOPE: AgentCommsScope = {
  workspaceId: "ws_main",
  profileKey: "default",
  params: { profile: "default" },
  actingProfile: "default",
};

describe("useActivityChildStates", () => {
  it("Should claim nothing until the daemon projects child_state", () => {
    const { result } = renderHook(() =>
      useActivityChildStates(
        SCOPE,
        [{ rootSessionId: "ses_root", childSessionIds: ["ses_child", "ses_reaped"] }],
        false
      )
    );
    expect(result.current.size).toBe(0);
  });

  it("Should ask nothing of the session catalog", () => {
    const { result } = renderHook(() =>
      useActivityChildStates(
        { ...SCOPE, workspaceId: "" },
        [{ rootSessionId: "ses_root", childSessionIds: ["ses_child"] }],
        true
      )
    );
    expect(result.current.size).toBe(0);
  });
});
