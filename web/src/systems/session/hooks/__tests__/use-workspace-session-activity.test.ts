import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../mocks/fixtures";
import type { SessionsResponse } from "../../types";
import { workspaceSessionActivityFromResults } from "../use-workspace-session-activity";

function response(sessions: SessionsResponse["sessions"], total: number): SessionsResponse {
  return {
    sessions,
    page: { has_more: false, limit: 1, total },
  };
}

describe("workspaceSessionActivityFromResults", () => {
  it("Should derive exact isolated counts and remove a deleted return target", () => {
    const alphaSession = {
      ...primarySessionFixture,
      id: "sess_alpha",
      workspace_id: "ws_alpha",
      name: "Review payment retries",
    };
    const betaSession = {
      ...primarySessionFixture,
      id: "sess_beta",
      workspace_id: "ws_beta",
      name: "Reconcile payout ledger",
    };

    const active = workspaceSessionActivityFromResults(
      ["ws_alpha", "ws_beta"],
      [{ data: response([alphaSession], 1) }, { data: response([betaSession], 3) }]
    );
    expect(active).toEqual({
      ws_alpha: {
        state: "ready",
        count: 1,
        returnTarget: {
          sessionId: "sess_alpha",
          agentName: alphaSession.agent_name,
          title: "Review payment retries",
        },
      },
      ws_beta: {
        state: "ready",
        count: 3,
        returnTarget: {
          sessionId: "sess_beta",
          agentName: betaSession.agent_name,
          title: "Reconcile payout ledger",
        },
      },
    });

    const afterDelete = workspaceSessionActivityFromResults(
      ["ws_alpha", "ws_beta"],
      [{ data: response([alphaSession], 1) }, { data: response([], 0) }]
    );
    expect(afterDelete).toEqual({
      ws_alpha: active.ws_alpha,
      ws_beta: { state: "ready", count: 0 },
    });
  });

  it("Should preserve loading and error states instead of reporting idle activity", () => {
    const activity = workspaceSessionActivityFromResults(
      ["ws_loading", "ws_error"],
      [
        { data: undefined },
        { data: undefined, isError: true, error: new Error("catalog stream unavailable") },
      ]
    );

    expect(activity).toEqual({
      ws_loading: { state: "loading" },
      ws_error: { state: "error", message: "catalog stream unavailable" },
    });
  });
});
