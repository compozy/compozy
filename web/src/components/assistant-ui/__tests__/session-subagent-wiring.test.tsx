import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { act, createElement, Profiler, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock("@/systems/session/adapters/subagent-api", () => ({
  cancelSubagent: vi.fn(),
  fetchSessionSubagents: vi.fn(),
}));
vi.mock("@/systems/session/adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session/adapters/session-api")>()),
  fetchSession: vi.fn(),
}));

import { fetchSession } from "@/systems/session/adapters/session-api";
import {
  cancelSubagent,
  fetchSessionSubagents,
  type SubagentPayload,
} from "@/systems/session/adapters/subagent-api";
import type { SubagentView } from "@/systems/session/components/subagents/types";
import {
  SubagentNavigationContext,
  type SubagentNavigation,
} from "@/systems/session/contexts/session-subagents-context-value";
import { SessionComposerSubagents } from "@/systems/session/components/session-composer-subagents";
import { SessionSubagentsProvider } from "@/systems/session/contexts/session-subagents-provider";
import { useSubagentOrigin } from "@/systems/session/hooks/use-subagent-origin";
import { normalizeTranscriptMessages } from "@/systems/session/lib/message-schemas";
import { sessionKeys } from "@/systems/session/lib/query-keys";
import { toThreadMessageLikes } from "@/systems/session/lib/session-thread-repository";
import { toTimelineParts } from "@/systems/session/lib/timeline-message-parts";
import type { SubagentRoster } from "@/systems/session/lib/subagent-roster";
import { primarySessionFixture } from "@/systems/session/mocks/fixtures";
import type { SessionMessage, SessionPayload } from "@/systems/session/types";

import { SessionSubagentBanner } from "../session-subagent-banner";
import { SessionSubagentRowView } from "../session-subagent-row";
import { deriveSessionRows, type SessionTimelinePart } from "../session-timeline.logic";

// Suite: subagent wiring between the parent roster, the transcript card, the composer banner and
// subagents control, the session window's navigation and the child's "Subagent of" origin.
// Invariant: cards, banner and composer control read state from the roster by id and act through
// the host's navigation and the cancel route; the origin names the parent only for delegated
// subagents.
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
    delivery: "none",
    ...overrides,
  };
}

function subagentPayload(id: string): SubagentPayload {
  return {
    subagent_id: id,
    workspace_id: WORKSPACE_ID,
    parent_session_id: SESSION_ID,
    parent_turn_id: "turn-1",
    child_session_id: `sess-${id}`,
    origin: "delegated",
    provider_tool_call_id: null,
    title: `Task ${id}`,
    role: "general",
    status: "running",
    work_state: "working",
    runtime: { agent: "codex", provider: "codex", model: "", reasoning_effort: "", speed: "" },
    depth: 1,
    progress: "",
    result: null,
    result_preview: "",
    result_truncated: false,
    error: null,
    wait_timed_out: false,
    delivery: "none",
    isolation: "shared",
    created_at: T0,
    started_at: T0,
    settled_at: null,
    updated_at: T0,
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

function harness(
  rows: SubagentView[],
  navigation: SubagentNavigation | null = null,
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
) {
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
  it("Should become navigable from a live roster update, then open here or in a new window", async () => {
    // Invariant: a delegated card cannot navigate without a child; the live roster
    // supplies that child without a reload or remount. Owner: view wiring, UT-W16.
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const openSession = vi.fn();
    renderRow(
      [cardPart("sub-1")],
      harness(
        [view("sub-1", { status: "queued", child_session_id: null })],
        { openSession },
        queryClient
      )
    );
    fireEvent.click(screen.getByRole("group", { name: "Task sub-1, Queued" }));
    expect(openSession).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Open Task sub-1" })).toBeNull();

    act(() => {
      queryClient.setQueryData<SubagentRoster>(
        sessionKeys.subagentRoster(WORKSPACE_ID, SESSION_ID),
        { rows: [view("sub-1")], staleIds: new Set() }
      );
    });
    const card = await screen.findByRole("button", { name: "Open Task sub-1" });
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

describe("roster updates re-render only the card they change (m19)", () => {
  it("Should leave another row untouched when one subagent reports progress", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = sessionKeys.subagentRoster(WORKSPACE_ID, SESSION_ID);
    queryClient.setQueryData<SubagentRoster>(key, {
      rows: [view("sub-a"), view("sub-b")],
      staleIds: new Set(),
    });
    const renders = { a: 0, b: 0 };
    const rowFor = (id: string) => {
      const [row] = deriveSessionRows([cardPart(id)]);
      if (row?.kind !== "subagents") throw new Error("expected a subagent row");
      return row;
    };
    const rowA = rowFor("sub-a");
    const rowB = rowFor("sub-b");
    render(
      <QueryClientProvider client={queryClient}>
        <UIProvider>
          <SessionSubagentsProvider workspaceId={WORKSPACE_ID} sessionId={SESSION_ID}>
            <Profiler id="a" onRender={() => (renders.a += 1)}>
              <SessionSubagentRowView
                row={rowA}
                renderNested={() => null}
                onGroupOpenChange={vi.fn()}
              />
            </Profiler>
            <Profiler id="b" onRender={() => (renders.b += 1)}>
              <SessionSubagentRowView
                row={rowB}
                renderNested={() => null}
                onGroupOpenChange={vi.fn()}
              />
            </Profiler>
          </SessionSubagentsProvider>
        </UIProvider>
      </QueryClientProvider>
    );
    const before = { ...renders };
    act(() => {
      queryClient.setQueryData<SubagentRoster>(key, {
        rows: [view("sub-a", { progress: "Reading webhook.go" }), view("sub-b")],
        staleIds: new Set(),
      });
    });
    expect(await screen.findByText("Reading webhook.go")).toBeInTheDocument();
    expect(renders.a).toBeGreaterThan(before.a);
    expect(renders.b).toBe(before.b);
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

  it("Should breathe the delegated glyph only while a settled result waits to wake the parent", () => {
    const glyph = (container: HTMLElement) =>
      container.querySelector("[data-slot=subagent-waiting-banner] > :first-child");
    const { container: quiet } = render(<SessionSubagentBanner parentTurnRunning={false} />, {
      wrapper: harness([view("sub-a"), view("sub-done", { status: "completed", settled_at: T0 })]),
    });
    expect(glyph(quiet)?.getAttribute("class")).not.toContain("animate-pulse");
    const { container: waking } = render(<SessionSubagentBanner parentTurnRunning={false} />, {
      wrapper: harness([
        view("sub-a"),
        view("sub-done", { status: "completed", settled_at: T0, delivery: "pending" }),
      ]),
    });
    expect(glyph(waking)?.getAttribute("class")).toContain("animate-pulse");
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

describe("composer subagents control (issue #766 wiring)", () => {
  const parent = (summary: SessionPayload["subagent_summary"] = null): SessionPayload => ({
    ...primarySessionFixture,
    id: SESSION_ID,
    workspace_id: WORKSPACE_ID,
    subagent_summary: summary,
  });
  // The banner observes the roster as the open thread does, so the control reads it live.
  const host = (session: SessionPayload) => (
    <>
      <SessionSubagentBanner parentTurnRunning />
      <SessionComposerSubagents session={session} />
    </>
  );

  it("Should count the parent's live roster and let a loaded empty roster beat a stale summary", () => {
    render(host(parent()), {
      wrapper: harness([
        view("sub-a"),
        view("sub-native", { origin: "provider_native", child_session_id: null }),
        view("sub-done", { status: "completed", settled_at: T0 }),
      ]),
    });
    expect(screen.getByRole("button", { name: "Subagents, 2 running" })).toBeInTheDocument();
    const stale = { live: 1, total: 3, failed: 0, attention: 0, most_urgent: "running" as const };
    render(host(parent(stale)), { wrapper: harness([]) });
    expect(screen.getByRole("button", { name: "Subagents, none yet" })).toBeInTheDocument();
  });

  it("Should hold the session summary until every page of a cold read is in", async () => {
    // The next page never answers: the first page alone must not stand in for the total.
    vi.mocked(fetchSessionSubagents).mockReturnValue(new Promise(() => undefined));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(sessionKeys.subagents(WORKSPACE_ID, SESSION_ID), {
      pages: [
        {
          subagents: [subagentPayload("sub-a"), subagentPayload("sub-b")],
          next_cursor: "page-2",
        },
      ],
      pageParams: [""],
    });
    const summary = {
      live: 3,
      total: 250,
      failed: 0,
      attention: 0,
      most_urgent: "running" as const,
    };
    render(
      <QueryClientProvider client={queryClient}>
        <UIProvider>
          <SessionComposerSubagents session={parent(summary)} />
        </UIProvider>
      </QueryClientProvider>
    );
    await waitFor(() => expect(fetchSessionSubagents).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Subagents, 3 running" })).toBeInTheDocument();
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

describe("provider-native attribution through the transcript adapters (UT-W11)", () => {
  // Regression: AI SDK validation dropped `parentToolCallId` from text parts, so a native
  // subagent's inner text joined the parent's own reply.
  it("Should keep inner text and tools attributed from the wire message to the card", async () => {
    const wire = {
      id: "msg-1",
      role: "assistant",
      parts: [
        { type: "tool-Agent", toolCallId: "toolu_agent", state: "input-available", input: {} },
        {
          type: "data-compozy-subagent",
          id: "sub-native",
          data: {
            subagent_id: "sub-native",
            tool_call_id: "toolu_agent",
            origin: "provider_native",
          },
        },
        { type: "text", text: "Inspecting the diff.", parentToolCallId: "toolu_agent" },
        {
          type: "tool-Read",
          toolCallId: "toolu_read",
          state: "input-available",
          input: { file_path: "/workspace/README.md" },
          parentToolCallId: "toolu_agent",
        },
        { type: "text", text: "The reviewer found no issues." },
      ],
    } as unknown as SessionMessage;
    const [normalized] = await normalizeTranscriptMessages([wire]);
    const [like] = toThreadMessageLikes([normalized!]);
    const rows = deriveSessionRows(toTimelineParts(like!));
    expect(rows.map(row => row.kind)).toEqual(["subagents", "text"]);
    const card = rows[0];
    if (card?.kind !== "subagents") throw new Error("expected the native card");
    expect(card.nested.get("sub-native")?.map(part => part.kind)).toEqual(["text", "tool"]);
    expect(rows[1]).toMatchObject({ part: { text: "The reviewer found no issues." } });
  });

  it("Should nest live prompt-stream parts by their provider metadata attribution", () => {
    const attributed = { compozy: { parentToolCallId: "toolu_agent" } };
    // The shape assistant-ui gives a streaming turn: data parts normalized, metadata kept.
    const parts = toTimelineParts({
      id: "turn-live",
      content: [
        {
          type: "data",
          name: "compozy-subagent",
          data: {
            subagent_id: "sub-native",
            tool_call_id: "toolu_agent",
            origin: "provider_native",
            turn_id: "turn-live",
          },
        },
        { type: "text", text: "Inspecting the diff.", providerMetadata: attributed },
        {
          type: "tool-call",
          toolCallId: "toolu_read",
          toolName: "Read",
          args: { file_path: "/workspace/README.md" },
          providerMetadata: attributed,
        },
        { type: "text", text: "Still working." },
      ],
    });
    const rows = deriveSessionRows(parts);
    expect(rows.map(row => row.kind)).toEqual(["subagents", "text"]);
    const card = rows[0];
    if (card?.kind !== "subagents") throw new Error("expected the native card");
    expect(card.nested.get("sub-native")?.map(part => part.kind)).toEqual(["text", "tool"]);
  });

  it("Should place a card in its own turn_id and never in the message's turn", () => {
    const card = (data: Record<string, unknown>) =>
      toTimelineParts({
        id: "msg-turn",
        content: [{ type: "data", name: "compozy-subagent", data }],
      })[0];
    expect(card({ subagent_id: "sub-1", tool_call_id: "c1", turn_id: "turn-spawn" })?.turnId).toBe(
      "turn-spawn"
    );
    expect(card({ subagent_id: "sub-1", tool_call_id: "c1" })?.turnId).toBeUndefined();
    const text = toTimelineParts({ id: "msg-turn", content: [{ type: "text", text: "hi" }] })[0];
    expect(text?.turnId).toBe("msg-turn");
  });
});
