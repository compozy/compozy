import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { workspaceKeys } from "@/systems/workspace";
import { workspaceDetailFixture } from "@/systems/workspace/mocks";
import { providerKeys } from "@/systems/providers";
import { modelCatalogKeys } from "@/systems/model-catalog";
import { modelCatalogAllModelsFixture } from "@/systems/model-catalog/mocks";
// Suite: empty desktop composer
// Invariant: an empty desktop takes a first message: sending it starts a session with the chosen
// agent in this scope's worktree, queues the message as that session's first prompt, and opens the
// session here; the words stay in the composer until a session owns them; in Global the composer
// stays closed and leads to picking a project.
// Owning layer: OsEmptyDesktop with its launcher hook, the prompt-first create path, and the real
// session composer. Only the create request, the session jump, and toasts are faked.
import type { AssistantRuntime } from "@assistant-ui/react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import {
  createSessionCreateStore,
  SessionCreateProvider,
  sessionStore,
  type SessionCreateStore,
} from "@/systems/session";

import { OsEmptyDesktop } from "../os-empty-desktop";

const workspace = vi.hoisted(() => ({
  runtimeWorkspaceId: "ws_alpha" as string | null,
  runtimeWorkspace: { default_agent: "general" } as { default_agent?: string } | undefined,
  scope: "workspace" as "global" | "workspace",
}));
const agents = vi.hoisted(() => ({
  data: [] as Array<{ name: string; provider: string }>,
  isSuccess: true,
  isLoading: false,
  error: null,
}));
const scopedWorktree = vi.hoisted(() => ({ worktreeId: "wt-feature" as string | undefined }));
const createSessionAsync = vi.hoisted(() => vi.fn());
const jumpToSession = vi.hoisted(() => vi.fn());
const notifyUser = vi.hoisted(() => vi.fn());
const runtimes = vi.hoisted(() => ({ latest: null as AssistantRuntime | null }));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: workspace.runtimeWorkspaceId, ...workspace }),
}));
vi.mock("@/systems/workspace/hooks/use-active-worktree", async importOriginal => ({
  ...(await importOriginal<object>()),
  useScopedWorktreeFilter: () => ({ worktreeId: scopedWorktree.worktreeId, resolved: true }),
}));
vi.mock("@/systems/agent/hooks/use-agents", async importOriginal => ({
  ...(await importOriginal<object>()),
  useAgents: () => agents,
}));
vi.mock("@/systems/session/hooks/use-session-actions", async importOriginal => ({
  ...(await importOriginal<object>()),
  useCreateSession: () => ({ mutateAsync: createSessionAsync }),
}));
vi.mock("../../hooks/use-attention-jump", () => ({ useAttentionJump: () => jumpToSession }));
const shellState = vi.hoisted(() => ({ activeDesktopId: "desktop-origin" as string | null }));
vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ manager: { getState: () => shellState } }),
}));
vi.mock("@/lib/user-feedback", () => ({ notifyUser }));
// The composer runtime has no transcript to type into from outside; the test
// drives it through the same runtime the composer renders.
vi.mock("../../hooks/use-empty-desktop-prompt-runtime", async importOriginal => {
  const actual =
    await importOriginal<typeof import("../../hooks/use-empty-desktop-prompt-runtime")>();
  return {
    ...actual,
    useEmptyDesktopPromptRuntime: (
      options: Parameters<typeof actual.useEmptyDesktopPromptRuntime>[0]
    ) => {
      const runtime = actual.useEmptyDesktopPromptRuntime(options);
      runtimes.latest = runtime;
      return runtime;
    },
  };
});

function composer() {
  if (!runtimes.latest) throw new Error("the empty desktop composer is not mounted");
  return runtimes.latest.thread.composer;
}

function renderDesktop(options: { canStart?: boolean; store?: SessionCreateStore } = {}) {
  const store = options.store ?? createSessionCreateStore();
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  for (const id of ["ws_alpha", "ws_beta"])
    client.setQueryData(workspaceKeys.detail(id), workspaceDetailFixture);
  client.setQueryData(providerKeys.lists(), {
    providers: [
      { name: "claude", display_name: "Claude", auth_status: { state: "authenticated" } },
      { name: "codex", display_name: "Codex", auth_status: { state: "authenticated" } },
    ],
  });
  client.setQueryData(modelCatalogKeys.allModels("all", undefined, undefined, true), {
    models: modelCatalogAllModelsFixture,
  });
  client.setQueryData(modelCatalogKeys.initialReadiness, true);
  const tree = () => (
    <QueryClientProvider client={client}>
      <TooltipProvider delay={0}>
        <SessionCreateProvider store={store}>
          <OsEmptyDesktop
            desktopName="Desktop 2"
            paletteShortcutLabel="⌘K"
            canStart={options.canStart ?? true}
          />
        </SessionCreateProvider>
      </TooltipProvider>
    </QueryClientProvider>
  );
  const view = render(tree());
  return { store, unmount: view.unmount, rerender: () => view.rerender(tree()) };
}

async function send(text: string, via: "button" | "enter" = "button") {
  const user = userEvent.setup();
  await act(async () => composer().setText(text));
  if (via === "button") {
    await user.click(screen.getByTestId("composer-send-button"));
    return;
  }
  screen
    .getByTestId("composer-input")
    .querySelector<HTMLElement>('[contenteditable="true"]')
    ?.focus();
  await user.keyboard("{Enter}");
}

describe("OsEmptyDesktop", () => {
  beforeEach(() => {
    localStorage.clear();
    workspace.runtimeWorkspaceId = "ws_alpha";
    workspace.runtimeWorkspace = { default_agent: "general" };
    workspace.scope = "workspace";
    agents.data = [
      { name: "general", provider: "claude" },
      { name: "reviewer", provider: "codex" },
    ];
    scopedWorktree.worktreeId = "wt-feature";
    createSessionAsync.mockReset();
    jumpToSession.mockReset();
    notifyUser.mockReset();
    shellState.activeDesktopId = "desktop-origin";
    runtimes.latest = null;
    for (const slot of ["ws_alpha", "ws_beta"]) {
      sessionStore.trigger.composerDraftDiscarded({ sessionId: `desktop:new-session:${slot}` });
    }
  });

  // Invariant: a pre-session runtime choice survives remount and stays scoped to its project.
  // Owning layer and canonical suite: OsEmptyDesktop composer integration.
  it("Should restore a chosen model after remount without leaking it to another project", async () => {
    const user = userEvent.setup();
    const first = renderDesktop();
    await user.click(screen.getByTestId("session-prompt-runtime-select"));
    await user.click(await screen.findByRole("option", { name: /Claude Fable 5/ }));
    expect(screen.getByTestId("session-prompt-runtime-select")).toHaveTextContent("Claude Fable 5");
    first.unmount();
    const second = renderDesktop();
    expect(screen.getByTestId("session-prompt-runtime-select")).toHaveTextContent("Claude Fable 5");
    workspace.runtimeWorkspaceId = "ws_beta";
    second.rerender();
    expect(screen.getByTestId("session-prompt-runtime-select")).not.toHaveTextContent(
      "Claude Fable 5"
    );
    workspace.runtimeWorkspaceId = "ws_alpha";
    second.rerender();
    expect(screen.getByTestId("session-prompt-runtime-select")).toHaveTextContent("Claude Fable 5");
  });

  it("Should carry an attachment-only prompt and the agent runtime into the new session", async () => {
    createSessionAsync.mockResolvedValue({
      id: "session-files",
      workspace_id: "ws_alpha",
      agent_name: "general",
    });
    renderDesktop();
    const file = new File(["Please inspect this"], "notes.txt", { type: "text/plain" });
    await act(async () => composer().addAttachment(file));
    expect(screen.getByTestId("composer-attachment-strip")).toHaveTextContent("notes.txt");
    await userEvent.setup().click(screen.getByTestId("composer-send-button"));
    await waitFor(() => expect(jumpToSession).toHaveBeenCalled());
    expect(sessionStore.getSnapshot().context.firstPrompts["session-files"]).toMatchObject({
      text: "",
      files: [file],
      runtime: { provider: "claude" },
      claimed: false,
    });
    await waitFor(() => expect(composer().getState().attachments).toHaveLength(0));
  });

  it("Should preserve attached files when creating the session fails", async () => {
    createSessionAsync.mockRejectedValue(new Error("Unavailable"));
    renderDesktop();
    const file = new File(["draft"], "draft.txt", { type: "text/plain" });
    await act(async () => composer().addAttachment(file));
    await send("Keep this draft");
    await waitFor(() => expect(createSessionAsync).toHaveBeenCalled());
    expect(composer().getState().text).toBe("Keep this draft");
    expect(
      composer()
        .getState()
        .attachments.map(attachment => attachment.file)
    ).toEqual([file]);
  });

  it("Should start a session with the sent prompt and open it on this desktop", async () => {
    let resolveCreate: ((session: unknown) => void) | undefined;
    createSessionAsync.mockImplementation(
      () =>
        new Promise(resolve => {
          resolveCreate = resolve;
        })
    );
    renderDesktop();

    expect(screen.getByRole("region", { name: "Desktop 2" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "What should we work on?" })).toBeInTheDocument();
    expect(screen.getByTestId("os-desk-status")).toHaveTextContent("Press ⌘K to open anything");
    expect(screen.getByTestId("os-desk-agent-select")).toHaveTextContent("General");
    expect(screen.getByTestId("composer-attach-button")).toBeInTheDocument();
    expect(screen.getByTestId("session-prompt-runtime-select")).toBeInTheDocument();
    await send("Fix the flaky login test", "enter");

    expect(createSessionAsync).toHaveBeenCalledWith({
      agent_name: "general",
      workspace: "ws_alpha",
      worktree: "wt-feature",
    });
    // In flight: the words stay put, the composer rests, and the status says why.
    expect(screen.getByTestId("os-desk-status")).toHaveTextContent("Starting a session…");
    expect(screen.getByTestId("composer-input")).toHaveAttribute("inert");
    expect(composer().getState().text).toBe("Fix the flaky login test");

    // The operator moves to another desktop while the session is created.
    shellState.activeDesktopId = "desktop-other";
    await act(async () =>
      resolveCreate?.({ id: "session-desk-1", workspace_id: "ws_alpha", agent_name: "general" })
    );

    expect(sessionStore.getSnapshot().context.firstPrompts["session-desk-1"]?.text).toBe(
      "Fix the flaky login test"
    );
    // It still lands on the desktop the prompt was sent from.
    expect(jumpToSession).toHaveBeenCalledWith({
      sessionId: "session-desk-1",
      agentName: "general",
      workspaceId: "ws_alpha",
      desktopId: "desktop-origin",
      // A just-created session prefers an existing Session window (#825).
      created: true,
    });
    await waitFor(() => expect(composer().getState().text).toBe(""));
    expect(
      sessionStore.getSnapshot().context.drafts["desktop:new-session:ws_alpha"]
    ).toBeUndefined();
  });

  it("Should keep each project's unsent prompt on that project's empty desktop", async () => {
    const { rerender } = renderDesktop();
    await act(async () => composer().setText("Draft the alpha release notes"));
    await waitFor(() =>
      expect(sessionStore.getSnapshot().context.drafts["desktop:new-session:ws_alpha"]).toBe(
        "Draft the alpha release notes"
      )
    );

    workspace.runtimeWorkspaceId = "ws_beta";
    rerender();
    await waitFor(() => expect(composer().getState().text).toBe(""));
    expect(
      sessionStore.getSnapshot().context.drafts["desktop:new-session:ws_beta"]
    ).toBeUndefined();

    workspace.runtimeWorkspaceId = "ws_alpha";
    rerender();
    await waitFor(() => expect(composer().getState().text).toBe("Draft the alpha release notes"));
  });

  it("Should start with the agent picked in the composer", async () => {
    const user = userEvent.setup();
    createSessionAsync.mockResolvedValue({
      id: "session-desk-2",
      workspace_id: "ws_alpha",
      agent_name: "reviewer",
    });
    renderDesktop();

    await user.click(screen.getByTestId("os-desk-agent-select"));
    await user.click(await screen.findByRole("option", { name: /Reviewer/ }));
    expect(screen.getByTestId("os-desk-agent-select")).toHaveTextContent("Reviewer");

    await send("Review the open diff");

    await waitFor(() =>
      expect(createSessionAsync).toHaveBeenCalledWith(
        expect.objectContaining({ agent_name: "reviewer" })
      )
    );
  });

  it("Should keep the prompt in the composer when the session cannot start", async () => {
    createSessionAsync.mockRejectedValue(new Error("agent executable is unavailable"));
    renderDesktop();

    await send("Draft the release notes");

    await waitFor(() =>
      expect(notifyUser).toHaveBeenCalledWith({
        message: "Could not ask the agent: agent executable is unavailable",
        tone: "error",
      })
    );
    expect(jumpToSession).not.toHaveBeenCalled();
    expect(composer().getState().text).toBe("Draft the release notes");
    expect(screen.getByTestId("composer-input")).not.toHaveAttribute("inert");
    expect(screen.getByTestId("composer-send-button")).toBeEnabled();
  });

  it("Should keep the prompt until the agent dialog it moved to creates the session", async () => {
    workspace.runtimeWorkspace = { default_agent: "retired" };
    const { store } = renderDesktop();

    await send("Summarize yesterday's runs");

    expect(createSessionAsync).not.toHaveBeenCalled();
    expect(store.getSnapshot().context).toMatchObject({
      open: true,
      pendingPrompt: "Summarize yesterday's runs",
    });
    act(() => store.trigger.dialogOpenChanged({ open: false }));
    expect(composer().getState().text).toBe("Summarize yesterday's runs");

    await send("Summarize yesterday's runs");
    expect(store.getSnapshot().context.open).toBe(true);
    await act(async () =>
      store.trigger.submissionRequested({
        agentName: "general",
        workspaceId: "ws_alpha",
        execute: async () => ({ id: "session-desk-3", agent_name: "general" }) as never,
        navigate: async () => undefined,
      })
    );

    await waitFor(() => expect(composer().getState().text).toBe(""));
  });

  it("Should start the prompt when the operator types, leaving the shell's chords alone", () => {
    renderDesktop();
    const editable = screen
      .getByTestId("composer-input")
      .querySelector<HTMLElement>('[contenteditable="true"]');
    // Stand-in for the shell's document listener: a bound chord is claimed before the window sees it.
    const claimShortcut = (event: KeyboardEvent) => {
      if (event.key === "?") event.preventDefault();
    };
    document.addEventListener("keydown", claimShortcut);
    try {
      expect(document.activeElement).toBe(document.body);
      fireEvent.keyDown(document.body, { key: "?" });
      fireEvent.keyDown(document.body, { key: "2", ctrlKey: true });
      fireEvent.keyDown(document.body, { key: "ArrowRight" });
      expect(document.activeElement).toBe(document.body);

      fireEvent.keyDown(document.body, { key: "F" });
      expect(document.activeElement).toBe(editable);
    } finally {
      document.removeEventListener("keydown", claimShortcut);
    }
  });

  it("Should rest the composer without a project prompt while the destination settles", () => {
    workspace.runtimeWorkspaceId = null;
    workspace.runtimeWorkspace = undefined;
    renderDesktop({ canStart: false });

    const input = screen.getByTestId("composer-input");
    expect(input).toHaveAttribute("inert");
    expect(input).not.toHaveTextContent(/pick a project/i);
    expect(screen.getByTestId("composer-send-button")).toBeDisabled();
    expect(screen.queryByTestId("os-desk-agent-select")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /pick a project/i })).toBeNull();
    expect(createSessionAsync).not.toHaveBeenCalled();
  });
});
