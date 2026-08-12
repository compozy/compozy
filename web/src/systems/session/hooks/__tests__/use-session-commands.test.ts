// Suite: session command menu projection and refresh
// Invariant: the web groups only daemon-projected lanes and an open-trigger refresh rereads the
// current workspace/session catalog before the menu presents commands.
// Owning layer: session command query hook.
// Canonical suite: this hook's existing session-command test suite.
// Boundary IN: typed session command response items and the TanStack Query hook.
// Boundary OUT: assistant-ui trigger detection/rendering and HTTP transport.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const sessionCommandApiMocks = vi.hoisted(() => ({
  fetchSessionCommands: vi.fn(),
}));

vi.mock("../../adapters/session-command-api", () => ({
  fetchSessionCommands: sessionCommandApiMocks.fetchSessionCommands,
}));

import type { SessionCommandPayload } from "../../types";
import { sessionCommandMenuCatalog, useSessionCommands } from "../use-session-commands";

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function command(
  id: string,
  token: string,
  lane: string,
  placements: string[]
): SessionCommandPayload {
  return {
    id,
    canonical_token: token,
    display_name: token,
    description: `${token} description`,
    available: true,
    lane,
    placements,
    source: { kind: lane, scope: "session" },
  };
}

describe("sessionCommandMenuCatalog", () => {
  it("Should show controls only at prompt start and skills in both placements", () => {
    const catalog = sessionCommandMenuCatalog([
      command("builtin:goal", "/goal", "builtin", ["standalone"]),
      command("agent:compact", "/compact", "agent", ["standalone"]),
      command("skill:workspace::review", "/review", "skill", ["standalone", "inline"]),
    ]);

    expect(catalog.standaloneSections.map(section => section.label)).toEqual([
      "Built-in",
      "Agent",
      "Skills",
    ]);
    expect(
      catalog.standaloneSections.flatMap(section => section.commands.map(item => item.token))
    ).toEqual(["/goal", "/compact", "/review"]);
    expect(catalog.inlineSkills.map(item => item.token)).toEqual(["/review"]);
  });

  it("Should humanize builtin/agent titles, keep skill identity, and carry lane + scope", () => {
    const catalog = sessionCommandMenuCatalog([
      command("builtin:goal", "/goal", "builtin", ["standalone"]),
      command("agent:compact-context", "/compact-context", "agent", ["standalone"]),
      command("skill:workspace::frontend-qa", "/frontend-qa", "skill", ["standalone", "inline"]),
    ]);

    expect(
      catalog.standaloneSections.flatMap(section => section.commands.map(item => item.label))
    ).toEqual(["Goal", "Compact Context", "frontend-qa"]);
    expect(
      catalog.standaloneSections.flatMap(section => section.commands.map(item => item.lane))
    ).toEqual(["builtin", "agent", "skill"]);
    expect(catalog.inlineSkills[0]).toMatchObject({
      lane: "skill",
      scope: "session",
      label: "frontend-qa",
    });
  });
});

describe("useSessionCommands", () => {
  it("Should reread the current catalog when the command-open refresh is requested", async () => {
    const firstCatalog = {
      commands: [command("skill:review", "/review", "skill", ["standalone", "inline"])],
      revision: "before",
    };
    const refreshedCatalog = {
      commands: [command("skill:ship", "/ship", "skill", ["standalone", "inline"])],
      revision: "after",
    };
    sessionCommandApiMocks.fetchSessionCommands
      .mockResolvedValueOnce(firstCatalog)
      .mockResolvedValueOnce(refreshedCatalog);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useSessionCommands(WORKSPACE_ID, SESSION_ID), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => expect(result.current.catalog?.inlineSkills[0]?.token).toBe("/review"));

    await act(async () => {
      await result.current.refetch();
    });

    await waitFor(() => expect(result.current.catalog?.inlineSkills[0]?.token).toBe("/ship"));
    expect(sessionCommandApiMocks.fetchSessionCommands).toHaveBeenNthCalledWith(
      2,
      WORKSPACE_ID,
      SESSION_ID,
      expect.any(AbortSignal)
    );
  });
});
