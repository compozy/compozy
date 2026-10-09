import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/subagent-api", () => ({ fetchSessionSubagents: vi.fn() }));

import { fetchSessionSubagents } from "../../adapters/subagent-api";
import type { SubagentPayload } from "../../adapters/subagent-api";
import { sessionKeys } from "../../lib/query-keys";
import type { SubagentRoster } from "../../lib/subagent-roster";
import { useSubagentRosterControlPoll } from "../use-subagent-roster-control-poll";

// Suite: mid-prompt subagent roster control poll.
// Invariant: while a prompt POST streams (the session SSE is closed) the roster still reads the
// parent's live subagents; outside a prompt it never polls. Owning layer: session hooks (roster
// feed). Boundary OUT: the subagent list adapter. Canonical suite: this file (new hook).
const WORKSPACE = "ws-01";
const SESSION = "sess-parent";

function row(id: string, status: string): SubagentPayload {
  return {
    subagent_id: id,
    workspace_id: WORKSPACE,
    parent_session_id: SESSION,
    parent_turn_id: "turn-1",
    child_session_id: null,
    origin: "provider_native",
    provider_tool_call_id: "toolu",
    title: "Review the diff",
    role: "general",
    status,
    work_state: "working",
    runtime: { agent: "", provider: "claude", model: "", reasoning_effort: "", speed: "" },
    depth: 1,
    progress: "",
    result: null,
    result_preview: "",
    result_truncated: false,
    error: null,
    wait_timed_out: false,
    delivery: "none",
    started_at: "2026-10-09T12:00:00Z",
    settled_at: null,
    created_at: "2026-10-09T12:00:00Z",
    updated_at: "2026-10-09T12:00:00Z",
  };
}

function setup(enabled: boolean) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  renderHook(
    () =>
      useSubagentRosterControlPoll({
        workspaceId: WORKSPACE,
        sessionId: SESSION,
        enabled,
        intervalMs: 1_000,
      }),
    { wrapper }
  );
  return queryClient;
}

beforeEach(() => {
  vi.mocked(fetchSessionSubagents).mockReset();
});

describe("subagent roster control poll", () => {
  it("Should feed the roster from the list route while a prompt streams", async () => {
    vi.mocked(fetchSessionSubagents).mockResolvedValue({
      subagents: [row("sub-native", "running")],
      next_cursor: null,
    });
    const queryClient = setup(true);
    await waitFor(() =>
      expect(
        queryClient
          .getQueryData<SubagentRoster>(sessionKeys.subagentRoster(WORKSPACE, SESSION))
          ?.rows.map(view => [view.id, view.status])
      ).toEqual([["sub-native", "running"]])
    );
    expect(fetchSessionSubagents).toHaveBeenCalledWith(
      WORKSPACE,
      SESSION,
      { limit: 200 },
      expect.anything()
    );
  });

  it("Should read every page so older cards keep their state", async () => {
    vi.mocked(fetchSessionSubagents)
      .mockResolvedValueOnce({ subagents: [row("sub-new", "running")], next_cursor: "page-2" })
      .mockResolvedValueOnce({ subagents: [row("sub-old", "completed")], next_cursor: null });
    const queryClient = setup(true);
    await waitFor(() =>
      expect(
        queryClient
          .getQueryData<SubagentRoster>(sessionKeys.subagentRoster(WORKSPACE, SESSION))
          ?.rows.map(view => [view.id, view.status])
      ).toEqual([
        ["sub-new", "running"],
        ["sub-old", "completed"],
      ])
    );
    expect(fetchSessionSubagents).toHaveBeenLastCalledWith(
      WORKSPACE,
      SESSION,
      { limit: 200, cursor: "page-2" },
      expect.anything()
    );
  });

  it("Should not poll outside a prompt", async () => {
    const queryClient = setup(false);
    await new Promise(resolve => setTimeout(resolve, 20));
    expect(fetchSessionSubagents).not.toHaveBeenCalled();
    expect(
      queryClient.getQueryData(sessionKeys.subagentRoster(WORKSPACE, SESSION))
    ).toBeUndefined();
  });
});
