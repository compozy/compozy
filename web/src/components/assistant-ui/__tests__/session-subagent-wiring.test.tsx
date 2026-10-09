import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock("@/systems/session/adapters/subagent-api", () => ({
  cancelSubagent: vi.fn(),
}));
vi.mock("@/systems/session/adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session/adapters/session-api")>()),
  fetchSession: vi.fn(),
}));

import { fetchSession } from "@/systems/session/adapters/session-api";
import { cancelSubagent } from "@/systems/session/adapters/subagent-api";
import type { SubagentView } from "@/systems/session/components/subagents/types";
import {
  SubagentNavigationContext,
  type SubagentNavigation,
} from "@/systems/session/contexts/session-subagents-context-value";
import { SessionSubagentsProvider } from "@/systems/session/contexts/session-subagents-provider";
import { useSubagentOrigin } from "@/systems/session/hooks/use-subagent-origin";
import { sessionKeys } from "@/systems/session/lib/query-keys";
import type { SubagentRoster } from "@/systems/session/lib/subagent-roster";
import { primarySessionFixture } from "@/systems/session/mocks/fixtures";
import type { SessionPayload } from "@/systems/session/types";

import { SessionSubagentBanner } from "../session-subagent-banner";
import { SessionSubagentRowView } from "../session-subagent-row";
import { deriveSessionRows, type SessionTimelinePart } from "../session-timeline.logic";

// Suite: subagent wiring between the parent roster, the transcript card, the composer banner,
// the session window's navigation and the child's "Subagent of" origin.
// Invariant: cards and banner read state from the roster by id and act through the host's
// navigation and the cancel route; the origin names the parent only for delegated subagents.
// Owning layer: session view wiring (roster context → connected views). Boundary OUT: the
// cancel and session-detail adapters. Canonical suite: this file (no existing suite owns it).
const WORKSPACE_ID = "ws-01";
const SESSION_ID = "sess-parent";
const T0 = "2026-10-08T21:00:00.000Z";

function view(id: string, overrides: Partial<SubagentView> = {}): SubagentView {
  return {
    id,
    parent_session_id: SESSION_ID,
    child_session_id: `sess-${id}`,
    origin: "delegated",
    title: `Task ${id}`,
    status: "running",
    progress: "",
    result_preview: "",
    error: null,
    runtime: { agent: "codex", provider: "codex", model: "", reasoning_effort: "", speed: "" },
    started_at: T0,
    settled_at: null,
    created_at: T0,
    updated_at: T0,
    ...overrides,
  };
}

function cardPart(id: string, origin = "delegated"): SessionTimelinePart {
  return {
    kind: "data",
    id,
    name: "data-compozy-subagent",
    data: { subagent_id: id, tool_call_id: `call-${id}`, origin },
    turnId: "turn-1",
  };
}

function harness(rows: SubagentView[], navigation: SubagentNavigation | null = null) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData<SubagentRoster>(sessionKeys.subagentRoster(WORKSPACE_ID, SESSION_ID), {
    rows,
    staleIds: new Set(),
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <UIProvider>
        <SubagentNavigationContext value={navigation}>
          <SessionSubagentsProvider workspaceId={WORKSPACE_ID} sessionId={SESSION_ID}>
            {children}
          </SessionSubagentsProvider>
        </SubagentNavigationContext>
      </UIProvider>
    </QueryClientProvider>
  );
}

function renderRow(parts: SessionTimelinePart[], wrapper: ReturnType<typeof harness>) {
  const [row] = deriveSessionRows(parts);
  if (row?.kind !== "subagents") throw new Error("expected a subagent row");
  return render(
    <SessionSubagentRowView row={row} renderNested={() => null} onGroupOpenChange={vi.fn()} />,
    { wrapper }
  );
}

beforeEach(() => {
  vi.mocked(cancelSubagent).mockReset();
  vi.mocked(fetchSession).mockReset();
});

describe("subagent card drill-in (UT-W16)", () => {
  it("Should open the child in this window on click and in a new window on ⌘/Ctrl-click", () => {
    const openSession = vi.fn();
    renderRow([cardPart("sub-1")], harness([view("sub-1")], { openSession }));
    const card = screen.getByRole("button", { name: "Open Task sub-1" });
    fireEvent.click(card);
    fireEvent.click(card, { metaKey: true });
    expect(openSession.mock.calls).toEqual([
      [
        { sessionId: "sess-sub-1", agentName: "codex", workspaceId: WORKSPACE_ID },
        { newWindow: false },
      ],
      [
        { sessionId: "sess-sub-1", agentName: "codex", workspaceId: WORKSPACE_ID },
        { newWindow: true },
      ],
    ]);
  });

  it("Should draw a provider-native card as a static row and a roster-less card as unconfirmed", () => {
    const native = view("sub-native", { origin: "provider_native", child_session_id: null });
    const { container } = renderRow(
      [cardPart("sub-native", "provider_native")],
      harness([native], { openSession: vi.fn() })
    );
    expect(screen.queryByRole("button")).toBeNull();
    expect(container.querySelector("[data-slot=subagent-card]")).toHaveAttribute(
      "data-origin",
      "provider_native"
    );

    const { container: unknown } = renderRow([cardPart("sub-unknown")], harness([]));
    const card = unknown.querySelector("[data-slot=subagent-card]");
    expect(card).toHaveAttribute("data-stale", "true");
    expect(card).toHaveAttribute("data-status", "running");
  });
});

describe("composer waiting banner (UT-W13 wiring)", () => {
  it("Should show live delegated subagents of an idle parent and stop each through the cancel route", async () => {
    vi.mocked(cancelSubagent).mockResolvedValue({ subagent_id: "x", status: "cancel_requested" });
    const rows = [
      view("sub-a"),
      view("sub-b", { status: "waiting" }),
      view("sub-native", { origin: "provider_native", child_session_id: null }),
      view("sub-done", { status: "completed", settled_at: T0 }),
    ];
    render(<SessionSubagentBanner parentTurnRunning={false} />, { wrapper: harness(rows) });
    expect(screen.getByText("Waiting on 2 subagents")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    await waitFor(() => expect(cancelSubagent).toHaveBeenCalledTimes(2));
    expect(vi.mocked(cancelSubagent).mock.calls.map(call => call.slice(0, 2))).toEqual([
      [WORKSPACE_ID, "sub-a"],
      [WORKSPACE_ID, "sub-b"],
    ]);
  });

  it("Should stay hidden while the parent turn runs or when only provider-native rows are live", () => {
    const native = view("sub-native", { origin: "provider_native", child_session_id: null });
    const { container: running } = render(<SessionSubagentBanner parentTurnRunning />, {
      wrapper: harness([view("sub-a")]),
    });
    expect(running.querySelector("[data-slot=subagent-waiting-banner]")).toBeNull();
    const { container: nativeOnly } = render(<SessionSubagentBanner parentTurnRunning={false} />, {
      wrapper: harness([native]),
    });
    expect(nativeOnly.querySelector("[data-slot=subagent-waiting-banner]")).toBeNull();
  });
});

describe("subagent origin (UT-W12 wiring)", () => {
  const child = (spawnRole: string): SessionPayload => ({
    ...primarySessionFixture,
    id: "sess-child",
    lineage: {
      ...primarySessionFixture.lineage!,
      kind: "spawn",
      spawn_role: spawnRole,
      parent_session_id: SESSION_ID,
    },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(
      QueryClientProvider,
      { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
      children
    );

  it("Should name the parent of a delegated subagent and report a deleted parent as gone", async () => {
    vi.mocked(fetchSession).mockResolvedValueOnce({
      ...primarySessionFixture,
      id: SESSION_ID,
      name: "Ship checkout v2",
    });
    const available = renderHook(() => useSubagentOrigin(child("subagent"), WORKSPACE_ID), {
      wrapper,
    });
    await waitFor(() =>
      expect(available.result.current?.parent).toMatchObject({
        id: SESSION_ID,
        title: "Ship checkout v2",
        agentName: primarySessionFixture.agent_name,
      })
    );

    vi.mocked(fetchSession).mockRejectedValueOnce(new Error("session_not_found"));
    const deleted = renderHook(() => useSubagentOrigin(child("subagent"), WORKSPACE_ID), {
      wrapper,
    });
    await waitFor(() => expect(deleted.result.current).toEqual({ parent: null }));
  });

  it("Should stay silent for a plain spawn child", () => {
    const plain = renderHook(() => useSubagentOrigin(child("worker"), WORKSPACE_ID), { wrapper });
    expect(plain.result.current).toBeNull();
    expect(fetchSession).not.toHaveBeenCalled();
  });
});
