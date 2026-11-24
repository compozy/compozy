// Suite: Remote session terminal scope hiding (remote-computers US-011.EC-1; UT-089)
// Invariant: a principal without `terminal:operate` on `devbox` sees NO Terminal control on
// `devbox` sessions (neither in the session header nor on supervised terminal tool rows), while a
// principal WITH `terminal:operate` sees `Terminal on devbox` and opening it targets `/c/cmp_devbox`.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Topbar, TopbarSlotProvider, UIProvider } from "@compozy/ui";

import { ComputerScopeContext } from "@/lib/computer-scope-context";
import { OsShellContext, type OsShellHandle } from "@/systems/os";

import { SessionTerminalToolRow } from "../session-terminal-tool-row";
import { useSessionTopbarSlot } from "../../hooks/use-session-topbar-slot";
import type { SessionPayload, UIMessage } from "../../types";

const DEVBOX_ID = "cmp_7f3k2q9x";

function computerPayload(scopes: string[], status = "connected") {
  return {
    id: DEVBOX_ID,
    name: "devbox",
    platform: "linux_x86_64",
    version: "0.9.0",
    protocol: { min: 1, max: 1 },
    status,
    enabled: true,
    plan_state: "ok",
    route_in_use: { id: "r1", kind: "direct", label: "via LAN 10.0.0.5" },
    routes: [
      {
        id: "r1",
        kind: "direct",
        address: "10.0.0.5:7777",
        in_use: true,
        learned: true,
        last_error: null,
      },
    ],
    scopes,
    load_balancing_weight: 50,
    diagnosis: null,
    added_at: "2026-11-01T00:00:00Z",
    last_connected_at: "2026-11-24T10:00:00Z",
  };
}

const REMOTE_SESSION: SessionPayload = {
  id: "sess_remote_1",
  name: "Fix remote build",
  agent_name: "claude",
  workspace_id: "ws_remote_1",
  workspace_path: "/srv/compozy",
  working_dir: "/srv/compozy",
  worktree_id: null,
  worktree_binding: null,
  profile_id: "prof_default",
  profile_name: "default",
  state: "active",
  archived_at: null,
  created_at: "2026-11-24T09:00:00Z",
  updated_at: "2026-11-24T10:00:00Z",
  runtime: {
    status: "running",
    acp_session_id: "acp_1",
    model_id: "claude-sonnet-4-6",
    mode_id: "default",
    recovery: null,
  },
  failure: null,
  activity: null,
  linked_work: [],
  processes: [],
} as unknown as SessionPayload;

const TERMINAL_TOOL_MESSAGE: UIMessage = {
  id: "msg-term-1",
  role: "assistant",
  content: "",
  timestamp: "2026-11-24T10:00:00Z",
  toolCallId: "tc-1",
  toolName: "compozy__terminal_exec",
  toolState: "completed",
  toolInput: { title: "uname -n" },
  toolResult: {
    rawOutput: {
      terminal_id: "term_remote_1",
      output: "devbox\n",
      exit_code: 0,
    },
  },
} as unknown as UIMessage;

function stubRemoteComputer(scopes: string[], status = "connected") {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const request = input instanceof Request ? input : new Request(String(input));
      const url = new URL(request.url);
      if (url.pathname === "/api/computers") {
        return Response.json({
          computers: [computerPayload(scopes, status)],
          local: { run_agents: true },
        });
      }
      if (url.pathname === "/api/workspaces") {
        return Response.json({ workspaces: [] });
      }
      if (url.pathname === "/api/agents/instances") {
        return Response.json({ instances: [] });
      }
      return Response.json({});
    })
  );
}

function makeShell() {
  const userOpen = vi.fn(async () => ({
    accepted: true,
    completion: Promise.resolve(true),
    windowId: "w-new",
  }));
  const shell = {
    coordinator: { userOpen },
    manager: { getState: () => ({ windows: {}, focusedId: null }) },
  } as unknown as OsShellHandle;
  return { shell, userOpen };
}

function RemoteSessionTopbarHarness() {
  useSessionTopbarSlot({
    session: REMOTE_SESSION,
    isStopping: false,
    isUnarchiving: false,
    inspectorOpen: false,
    reviewOpen: false,
    onReviewToggle: vi.fn(),
    sidebarOpen: false,
    linkedWork: {
      items: [],
      lead: null,
      count: 0,
      clipped: false,
      refreshing: false,
      setClipped: vi.fn(),
      unlink: vi.fn(),
      refresh: vi.fn(),
    } as never,
    onInspectorToggle: vi.fn(),
    onSidebarToggle: vi.fn(),
    onStop: vi.fn(),
    onUnarchive: vi.fn(),
  });
  return null;
}

describe("Remote session terminal & files scope gating (UT-089)", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("hides every Terminal control on devbox sessions when terminal:operate is absent (UT-089)", async () => {
    stubRemoteComputer(["sessions:operate", "files:read"]);
    const { shell } = makeShell();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    render(
      <QueryClientProvider client={client}>
        <UIProvider>
          <OsShellContext value={shell}>
            <ComputerScopeContext value={DEVBOX_ID}>
              <TopbarSlotProvider>
                <Topbar title="Session" />
                <RemoteSessionTopbarHarness />
              </TopbarSlotProvider>
              <SessionTerminalToolRow message={TERMINAL_TOOL_MESSAGE} />
            </ComputerScopeContext>
          </OsShellContext>
        </UIProvider>
      </QueryClientProvider>
    );

    // Wait for the remote computer chip & Files control to appear from /api/computers.
    await waitFor(() =>
      expect(screen.getByTestId(`session-computer-chip-${DEVBOX_ID}`)).toBeInTheDocument()
    );
    expect(screen.getByTestId("session-remote-files-action")).toBeInTheDocument();
    // No Terminal control anywhere for a principal without `terminal:operate`.
    expect(screen.queryByTestId("session-remote-terminal-action")).toBeNull();
    expect(screen.queryByRole("button", { name: /open terminal|terminal on devbox/i })).toBeNull();
    expect(screen.queryByTestId("session-terminal-open-term_remote_1")).toBeNull();
  });

  it("shows Terminal on devbox and opens a remote terminal when terminal:operate is granted", async () => {
    stubRemoteComputer(["sessions:operate", "terminal:operate", "files:read"]);
    const { shell, userOpen } = makeShell();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    render(
      <QueryClientProvider client={client}>
        <UIProvider>
          <OsShellContext value={shell}>
            <ComputerScopeContext value={DEVBOX_ID}>
              <TopbarSlotProvider>
                <Topbar title="Session" />
                <RemoteSessionTopbarHarness />
              </TopbarSlotProvider>
              <SessionTerminalToolRow message={TERMINAL_TOOL_MESSAGE} />
            </ComputerScopeContext>
          </OsShellContext>
        </UIProvider>
      </QueryClientProvider>
    );

    const terminalButton = await screen.findByRole("button", { name: "Terminal on devbox" });
    fireEvent.click(terminalButton);
    expect(userOpen).toHaveBeenCalledWith(
      expect.objectContaining({
        app: "terminal",
        forceNewInstance: true,
        route: expect.objectContaining({
          pathname: "/terminal",
          search: expect.objectContaining({
            computer: DEVBOX_ID,
            new: "1",
          }),
        }),
      })
    );

    const openToolButton = screen.getByTestId("session-terminal-open-term_remote_1");
    fireEvent.click(openToolButton);
    expect(userOpen).toHaveBeenCalledWith(
      expect.objectContaining({
        app: "terminal",
        instanceKey: "term_remote_1",
        route: expect.objectContaining({
          pathname: "/terminal/term_remote_1",
          search: expect.objectContaining({ computer: DEVBOX_ID }),
        }),
      })
    );
  });
});
