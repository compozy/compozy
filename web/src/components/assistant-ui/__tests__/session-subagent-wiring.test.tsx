import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  AssistantRuntimeProvider,
  MessageByIndexProvider,
  useExternalStoreRuntime,
  fromThreadMessageLike,
  type ThreadMessageLike,
} from "@assistant-ui/react";
import { fireEvent, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import { act, createElement, Profiler, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import { SubagentDrawerContext, subagentDrawerLogic } from "@/systems/session";

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
import { useSessionSubagents } from "@/systems/session/hooks/use-session-subagents";
import { normalizeTranscriptMessages } from "@/systems/session/lib/message-schemas";
import { sessionKeys } from "@/systems/session/lib/query-keys";
import { toThreadMessageLikes } from "@/systems/session/lib/session-thread-repository";
import { toTimelineParts } from "@/systems/session/lib/timeline-message-parts";
import { SessionActiveTurnContext } from "@/systems/session/lib/session-turn-outcomes-context-value";
import type { SubagentRoster } from "@/systems/session/lib/subagent-roster";
import { primarySessionFixture } from "@/systems/session/mocks/fixtures";
import type { SessionMessage, SessionPayload } from "@/systems/session/types";

import { SessionNativeSubagentActivity } from "../session-native-subagent-activity";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";
import { subagentViewFromPayload } from "@/systems/session/lib/subagent-payload";
import { SessionSubagentBanner } from "../session-subagent-banner";
import { useAssistantMessageTimeline } from "../hooks/use-assistant-message-timeline";
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
  queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, SESSION_ID), {
    profile_name: "design",
  });
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

describe("parent timeline activity across subagent updates (#835)", () => {
  it("Should keep a daemon-owned turn open between tools before and after delegation", () => {
    // Invariant: the active parent turn never folds or publishes final file counts
    // between tool calls. Owner: timeline hook wiring; canonical suite: this file.
    const Host = harness([view("sub-finished", { status: "completed", settled_at: T0 })]);
    let activeTurnId: string | undefined = "turn-1";
    let calls = 1;
    function Wrapper({ children }: { children: ReactNode }) {
      const runtime = useExternalStoreRuntime({
        messages: [
          {
            id: calls >= 3 ? "turn-1-4" : "turn-1",
            role: "assistant",
            content: [
              ...Array.from({ length: calls }, (_, index) => ({
                type: "tool-call",
                toolCallId: `edit-${index}`,
                toolName: "Edit",
                argsText: "{}",
                args: { file_path: `/src/${index}.ts`, old_string: "a", new_string: "b" },
                result: { ok: true },
                turnId: "turn-1",
              })),
              ...(calls >= 2
                ? [
                    {
                      type: "data",
                      name: "compozy-subagent",
                      data: {
                        subagent_id: "sub-finished",
                        tool_call_id: "delegate-finished",
                        turn_id: "turn-1",
                      },
                    },
                  ]
                : []),
              { type: "text", text: "Continuing the parent work", turnId: "turn-1" },
            ],
          } as ThreadMessageLike,
        ],
        convertMessage: value =>
          fromThreadMessageLike(value, "turn-1", { type: "complete", reason: "stop" }),
        onNew: async () => {},
      });
      return (
        <Host>
          <AssistantRuntimeProvider runtime={runtime}>
            <SessionActiveTurnContext value={activeTurnId}>
              <MessageByIndexProvider index={0}>{children}</MessageByIndexProvider>
            </SessionActiveTurnContext>
          </AssistantRuntimeProvider>
        </Host>
      );
    }
    const { result, rerender } = renderHook(useAssistantMessageTimeline, { wrapper: Wrapper });
    for (calls = 1; calls <= 4; calls++) {
      rerender();
      expect(result.current.rows.some(row => row.kind === "working")).toBe(true);
      expect(
        result.current.rows.some(row => row.kind === "turn-fold" || row.kind === "changed-files")
      ).toBe(false);
    }
    activeTurnId = undefined;
    rerender();
    expect(result.current.rows.some(row => row.kind === "working")).toBe(false);
    expect(result.current.rows.some(row => row.kind === "turn-fold")).toBe(true);
  });

  // Invariant: child/progress updates cannot move the parent's active identity;
  // roster settlement reveals its edited files once. Owner: timeline hook wiring.
  it.each(["progress", "child"])(
    "Should retain the parent turn across alternating %s turn IDs",
    async tail => {
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const Host = harness([view("sub-live")], null, client);
      const edit = {
        type: "tool-call",
        toolCallId: "edit-1",
        toolName: "Edit",
        argsText: "{}",
        args: { file_path: "/src/a.ts", old_string: "a", new_string: "a\nb" },
        result: { ok: true },
        turnId: "turn-1",
      };
      let message: ThreadMessageLike;
      const update = (trailingTurnId: string, running = true): ThreadMessageLike => ({
        id: "turn-1",
        role: "assistant",
        status: running ? { type: "running" } : { type: "complete", reason: "stop" },
        content: [
          edit,
          {
            type: "data",
            name: "compozy-subagent",
            data: {
              subagent_id: "sub-live",
              tool_call_id: "delegate-1",
              turn_id: "turn-1",
              origin: "provider_native",
            },
          },
          tail === "progress"
            ? {
                type: "data",
                name: "compozy-event",
                state: "streaming",
                data: { type: "runtime_progress", turn_id: trailingTurnId },
              }
            : {
                type: "text",
                text: "Reviewing the change",
                state: "streaming",
                turnId: trailingTurnId,
                parentToolCallId: "delegate-1",
              },
        ] as ThreadMessageLike["content"],
      });
      message = update("turn-1");
      function Wrapper({ children }: { children: ReactNode }) {
        const runtime = useExternalStoreRuntime({
          messages: [message],
          convertMessage: value =>
            fromThreadMessageLike(value, "turn-1", { type: "complete", reason: "stop" }),
          onNew: async () => {},
        });
        return (
          <Host>
            <AssistantRuntimeProvider runtime={runtime}>
              <MessageByIndexProvider index={0}>{children}</MessageByIndexProvider>
            </AssistantRuntimeProvider>
          </Host>
        );
      }
      const { result, rerender } = renderHook(useAssistantMessageTimeline, { wrapper: Wrapper });
      for (const turnId of ["child-turn", "turn-1", "another-turn", "turn-1"]) {
        message = update(turnId);
        rerender();
        expect(result.current.rows.find(row => row.kind === "working")?.turnId).toBe("turn-1");
        expect(result.current.rows.some(row => row.kind === "changed-files")).toBe(false);
      }
      message = update("child-turn", false);
      rerender();
      expect(result.current.rows.some(row => row.kind === "working")).toBe(false);
      expect(result.current.rows.some(row => row.kind === "changed-files")).toBe(false);
      act(() => {
        client.setQueryData<SubagentRoster>(sessionKeys.subagentRoster(WORKSPACE_ID, SESSION_ID), {
          rows: [view("sub-live", { status: "completed", settled_at: T0 })],
          staleIds: new Set(),
        });
      });
      await waitFor(() =>
        expect(result.current.rows.filter(row => row.kind === "changed-files")).toHaveLength(1)
      );
    }
  );
});

describe("subagent card drill-in (UT-W16)", () => {
  // Invariant: inspection selects a queued delegation before a child session exists.
  // Owner/canonical suite: provider → card → window drawer store wiring, UT-W16.
  it("Should select a queued subagent in the drawer before its child session exists", () => {
    const queued = view("sub-1", { status: "queued", child_session_id: null });
    const store = subagentDrawerLogic.createStore();
    const openSession = vi.fn();
    const Host = harness([queued], { openSession });
    function Wrapper({ children }: { children: ReactNode }) {
      return (
        <SubagentDrawerContext
          value={{
            store,
            open: (subagent, _options, workspaceId) =>
              store.trigger.opened({ subagent, workspaceId, trigger: null }),
          }}
        >
          <Host>{children}</Host>
        </SubagentDrawerContext>
      );
    }
    renderRow([cardPart("sub-1")], Wrapper);
    const card = screen.getByRole("button", { name: "Open Task sub-1" });
    fireEvent.click(card);
    expect(store.getSnapshot().context.selected).toEqual({
      subagent: queued,
      workspaceId: WORKSPACE_ID,
    });
    expect(card).toHaveAttribute("aria-current", "true");
    expect(openSession).not.toHaveBeenCalled();
  });

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
    fireEvent.click(screen.getByRole("button", { name: "Open Task sub-1" }));
    expect(openSession).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Open Task sub-1" })).toBeEnabled();

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

  it("Should reveal native work in its parent and keep a roster-less card unconfirmed", () => {
    const native = view("sub-native", { origin: "provider_native", child_session_id: null });
    const openSession = vi.fn();
    const { container } = renderRow(
      [cardPart("sub-native", "provider_native")],
      harness([native], { openSession })
    );
    fireEvent.click(screen.getByRole("button", { name: "Open Task sub-native" }));
    expect(openSession).toHaveBeenCalledWith(
      { sessionId: SESSION_ID, agentName: "", workspaceId: WORKSPACE_ID, subagentId: "sub-native" },
      { newWindow: false }
    );
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
    vi.mocked(fetchSession).mockResolvedValue({
      ...primarySessionFixture,
      profile_name: "design-renamed",
    });
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
    expect(vi.mocked(cancelSubagent).mock.calls.map(call => call.slice(0, 3))).toEqual([
      [WORKSPACE_ID, "sub-a", "design-renamed"],
      [WORKSPACE_ID, "sub-b", "design-renamed"],
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
  it("Should refresh a renamed parent owner before stopping a roster row", async () => {
    // Invariant: the immutable owner may have a new name; cancellation must resolve it at dispatch.
    // Owner: roster action hook; canonical suite: this session/subagent wiring suite.
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(sessionKeys.detail(WORKSPACE_ID, SESSION_ID), {
      ...primarySessionFixture,
      profile_name: "old-name",
    });
    vi.mocked(fetchSession).mockResolvedValue({
      ...primarySessionFixture,
      profile_name: "new-name",
    });
    vi.mocked(cancelSubagent).mockResolvedValue({
      subagent_id: "sub-a",
      status: "cancel_requested",
    });
    const { result } = renderHook(
      () => useSessionSubagents(WORKSPACE_ID, SESSION_ID, { coldRead: false }),
      {
        wrapper: ({ children }) => (
          <QueryClientProvider client={client}>{children}</QueryClientProvider>
        ),
      }
    );
    await act(async () => {
      await result.current.stop(view("sub-a"));
    });
    expect(cancelSubagent).toHaveBeenCalledExactlyOnceWith(WORKSPACE_ID, "sub-a", "new-name");
    client.clear();
  });

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

  it("Should hold the opened popover's place while the roster is read, never claim nothing was delegated", async () => {
    // Invariant: until the roster answers, the popover reads as loading rather than empty.
    // Owner: the composer control's cold read, reusing this suite's adapter boundary.
    vi.mocked(fetchSessionSubagents)
      .mockReset()
      .mockReturnValue(new Promise(() => undefined));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <UIProvider>
          <SessionComposerSubagents session={parent()} />
        </UIProvider>
      </QueryClientProvider>
    );

    fireEvent.click(screen.getByRole("button", { name: "Subagents, none yet" }));

    const popover = await screen.findByRole("dialog");
    await waitFor(() => expect(fetchSessionSubagents).toHaveBeenCalled());
    expect(within(popover).getByRole("status")).toHaveTextContent("Loading subagents");
    expect(within(popover).queryByText("Nothing delegated yet.")).not.toBeInTheDocument();
    queryClient.clear();
  });

  it("Should stop a failed cold page walk until the operator retries", async () => {
    // Invariant: a failed continuation cannot spin requests; explicit retry can recover.
    // Owner: cold roster wiring, reusing this suite's real QueryClient and adapter boundary.
    vi.mocked(fetchSessionSubagents).mockReset().mockRejectedValue(new Error("Page unavailable"));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const firstPage = { subagents: [subagentPayload("sub-a")], next_cursor: "page-2" };
    client.setQueryData(sessionKeys.subagents(WORKSPACE_ID, SESSION_ID), {
      pages: [firstPage],
      pageParams: [""],
    });
    const { result } = renderHook(() => useSessionSubagents(WORKSPACE_ID, SESSION_ID), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    });
    await waitFor(() => expect(result.current.failed).toBe(true));
    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 50));
    });
    expect(fetchSessionSubagents).toHaveBeenCalledTimes(1);
    expect(result.current.complete).toBe(false);
    vi.mocked(fetchSessionSubagents)
      .mockResolvedValueOnce(firstPage)
      .mockResolvedValueOnce({ subagents: [subagentPayload("sub-b")], next_cursor: null });
    act(() => result.current.retry());
    await waitFor(() => expect(result.current.complete).toBe(true));
    expect(result.current.subagents.map(row => row.id)).toEqual(["sub-a", "sub-b"]);
    client.clear();
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

// Invariant: native background answers/errors survive the wire-to-view mapping even with no transcript parts.
// Owner: session drawer wiring; canonical suite: this file. No mocked internal hooks.
it("Should update native background results and errors independently of transcript tool parts", () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const renderActivity = (payload: SubagentPayload) => (
    <QueryClientProvider client={client}>
      <UIProvider>
        <SessionTranscriptThreadProvider
          messages={[]}
          status="success"
          error={null}
          retry={() => {}}
        >
          <SessionNativeSubagentActivity
            subagent={subagentViewFromPayload(payload)}
            workspaceId={WORKSPACE_ID}
          />
        </SessionTranscriptThreadProvider>
      </UIProvider>
    </QueryClientProvider>
  );
  const payload = {
    ...subagentPayload("native-background"),
    origin: "provider_native",
    child_session_id: null,
  };
  const rendered = render(renderActivity(payload));
  rendered.rerender(
    renderActivity({
      ...payload,
      status: "completed",
      result: "Complete background answer",
      result_preview: "Preview only",
    })
  );
  expect(screen.getByText("Complete background answer")).toBeVisible();
  expect(screen.queryByText("Preview only")).not.toBeInTheDocument();
  rendered.rerender(
    renderActivity({ ...payload, status: "failed", error: "Background task failed" })
  );
  expect(screen.getByRole("alert")).toHaveTextContent("Background task failed");
});

// Invariant: nested native rows read and navigate through the owning parent roster.
// Owner/canonical suite: session-subagent wiring, using the real provider and query cache.
it("Should resolve nested native activity rows from the parent roster", () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const nested = view("nested", {
    title: "Nested review",
    status: "failed",
    error: "Review failed",
  });
  client.setQueryData<SubagentRoster>(sessionKeys.subagentRoster(WORKSPACE_ID, SESSION_ID), {
    rows: [nested],
    staleIds: new Set(),
  });
  const openSession = vi.fn();
  // Provider attribution extends assistant-ui's generic data-part shape.
  const nestedPart = {
    type: "data" as const,
    name: "compozy-subagent",
    data: { subagent_id: "nested", tool_call_id: "call-nested", origin: "delegated" },
    providerMetadata: { compozy: { parentToolCallId: "call-root" } },
  };
  const message = fromThreadMessageLike(
    {
      role: "assistant",
      content: [
        {
          type: "data",
          name: "compozy-subagent",
          data: {
            subagent_id: "root",
            tool_call_id: "call-root",
            origin: "provider_native",
          },
        },
        nestedPart,
      ],
    },
    "nested-activity",
    { type: "complete", reason: "stop" }
  );
  render(
    <QueryClientProvider client={client}>
      <UIProvider>
        <SubagentNavigationContext value={{ openSession }}>
          <SessionTranscriptThreadProvider
            messages={[message]}
            status="success"
            error={null}
            retry={() => {}}
          >
            <SessionNativeSubagentActivity
              workspaceId={WORKSPACE_ID}
              subagent={view("root", { origin: "provider_native", child_session_id: null })}
            />
          </SessionTranscriptThreadProvider>
        </SubagentNavigationContext>
      </UIProvider>
    </QueryClientProvider>
  );
  expect(screen.getByText("Nested review")).toBeVisible();
  expect(screen.getByText("Failed")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Open Nested review" }));
  expect(openSession).toHaveBeenCalledWith(
    expect.objectContaining({ workspaceId: WORKSPACE_ID, sessionId: "sess-nested" }),
    { newWindow: false }
  );
});

// Invariant: native activity includes deep descendants but never sibling or cyclic branches.
// Owner/canonical suite: session drawer wiring, rendered through the actual timeline.
it("Should include deep native descendants and exclude unrelated activity", () => {
  const attributed = (parentToolCallId: string) => ({ compozy: { parentToolCallId } });
  const messages: ThreadMessageLike[] = [
    {
      role: "assistant",
      content: [
        {
          type: "data",
          name: "compozy-subagent",
          data: {
            subagent_id: "root",
            tool_call_id: "root-call",
            origin: "provider_native",
          },
        },
        {
          type: "tool-call",
          toolCallId: "child-call",
          toolName: "Read",
          args: {},
          result: "Child result",
          providerMetadata: attributed("root-call"),
        },
        {
          type: "tool-call",
          toolCallId: "deep-call",
          toolName: "Read",
          args: {},
          result: "Deep result",
          providerMetadata: attributed("child-call"),
        },
        {
          type: "text",
          text: "Deep inspection completed",
          providerMetadata: attributed("deep-call"),
        },
        {
          type: "text",
          text: "Sibling private activity",
          providerMetadata: attributed("sibling-call"),
        },
        {
          type: "tool-call",
          toolCallId: "cycle-call",
          toolName: "Read",
          args: {},
          result: "Cycle result",
          providerMetadata: attributed("cycle-call"),
        },
        {
          type: "text",
          text: "Cyclic unrelated activity",
          providerMetadata: attributed("cycle-call"),
        },
      ],
    },
  ];
  function Activity() {
    const runtime = useExternalStoreRuntime({
      messages,
      convertMessage: message => message,
      isRunning: false,
      onNew: async () => {},
    });
    return (
      <AssistantRuntimeProvider runtime={runtime}>
        <SessionTranscriptThreadProvider
          messages={messages.map((message, index) =>
            fromThreadMessageLike(message, `activity-${index}`, {
              type: "complete",
              reason: "stop",
            })
          )}
          status="success"
          error={null}
          retry={() => {}}
        >
          <SessionNativeSubagentActivity
            workspaceId={WORKSPACE_ID}
            subagent={view("root", { origin: "provider_native", child_session_id: null })}
          />
        </SessionTranscriptThreadProvider>
      </AssistantRuntimeProvider>
    );
  }
  render(<Activity />, { wrapper: harness([]) });
  expect(screen.getByText("Deep inspection completed")).toBeVisible();
  expect(screen.queryByText("Sibling private activity")).not.toBeInTheDocument();
  expect(screen.queryByText("Cyclic unrelated activity")).not.toBeInTheDocument();
});
