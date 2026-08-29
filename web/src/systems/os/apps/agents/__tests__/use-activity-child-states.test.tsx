// Suite: Activity child-state catalog reads
// Invariant: Activity reads one scoped session catalog and copies only daemon-
// projected child lifecycle. Owning layer: `useActivityChildStates`.
// Canonical suite: this file.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AgentCommsScope } from "@/systems/agent-comms";

import { useActivityChildStates } from "../use-activity-child-states";

const useSessionsMock = vi.hoisted(() => vi.fn());

vi.mock("@/systems/session", () => ({ useSessions: useSessionsMock }));

const SCOPE: AgentCommsScope = {
  workspaceId: "ws_main",
  profileKey: "default",
  params: { profile: "default" },
  actingProfile: "default",
};

describe("useActivityChildStates", () => {
  beforeEach(() => {
    useSessionsMock.mockReset();
    useSessionsMock.mockReturnValue({ data: undefined });
  });

  it("Should copy projected states for only the children named by Activity", () => {
    useSessionsMock.mockReturnValue({
      data: [
        { id: "ses_child", child_state: "parked" },
        { id: "ses_reaped", child_state: "gone" },
        { id: "ses_unrelated", child_state: "running" },
      ],
    });
    const { result } = renderHook(() =>
      useActivityChildStates(
        SCOPE,
        [{ rootSessionId: "ses_root", childSessionIds: ["ses_child", "ses_reaped"] }],
        true
      )
    );
    expect([...result.current]).toEqual([
      ["ses_child", "parked"],
      ["ses_reaped", "gone"],
    ]);
    expect(useSessionsMock).toHaveBeenCalledOnce();
    expect(useSessionsMock).toHaveBeenCalledWith("ws_main", { enabled: true, loadAll: true });
  });

  it("Should keep the catalog query disabled outside a live workspace", () => {
    const { result } = renderHook(() =>
      useActivityChildStates(
        { ...SCOPE, workspaceId: "" },
        [{ rootSessionId: "ses_root", childSessionIds: ["ses_child"] }],
        true
      )
    );
    expect(result.current.size).toBe(0);
    expect(useSessionsMock).toHaveBeenCalledWith("", { enabled: false, loadAll: true });
  });
});
