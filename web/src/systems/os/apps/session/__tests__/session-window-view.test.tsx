// Suite: OS session-window view runtime selection
// Invariant: a User-scope session (no workspace, scope "user") is live work and binds the chat
// runtime to the `@user` selector; only a legacy session without any scope falls back to the
// read-only history. A window whose session has not loaded holds the frames the loaded window
// fills (rail, transcript, composer) under one busy status, never a bare spinner.
// Owning layer: the OS session-window view.
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SessionWindowView } from "../session-window-view";

const instances = vi.hoisted(() => ({ current: [] as { id: string; main_session_id: string }[] }));
vi.mock("@/systems/agent", () => ({ useAgentInstances: () => instances.current }));

const runtimeProviderSpy = vi.hoisted(() => vi.fn());
const readOnlyTranscriptSpy = vi.hoisted(() => vi.fn());

vi.mock("@/systems/session", () => ({
  canPromptSession: () => true,
  useMarkSeenOnLeave: () => undefined,
  SessionSidebarLayout: ({ sidebar, children }: { sidebar: ReactNode; children: ReactNode }) => (
    <>
      {sidebar}
      {children}
    </>
  ),
  SessionSidebarSkeleton: () => <aside data-testid="session-sidebar-skeleton" />,
  SessionPromptRuntimeProvider: ({ children }: { children: ReactNode }) => children,
  SessionChatRuntimeProvider: (props: { workspaceId: string; children: ReactNode }) => {
    runtimeProviderSpy(props.workspaceId);
    return null;
  },
}));

vi.mock("../session-read-only-transcript", () => ({
  SessionReadOnlyTranscript: () => {
    readOnlyTranscriptSpy();
    return null;
  },
}));

vi.mock("../session-window-module-loader", () => ({
  loadSessionWindowContent: () => new Promise(() => {}),
}));

function renderView(session: Record<string, unknown> | undefined, isLoading = false) {
  return render(
    <SessionWindowView
      windowId="session:sess-1"
      name="general"
      id="sess-1"
      workspaceId=""
      session={session as never}
      liveTailEnabled
      isLoading={isLoading}
      error={null}
    />
  );
}

describe("SessionWindowView", () => {
  it("Should keep a superseded instance session read-only and link to its current main", () => {
    instances.current = [{ id: "instance", main_session_id: "sess-current" }];
    renderView({ id: "sess-1", agent_name: "general", scope: "user", instance_id: "instance" });
    expect(readOnlyTranscriptSpy).toHaveBeenCalled();
    expect(runtimeProviderSpy).not.toHaveBeenCalled();
    expect(screen.getByRole("link", { name: "Open the current one" })).toHaveAttribute(
      "href",
      "/session/sess-current"
    );
  });

  beforeEach(() => {
    instances.current = [];
    runtimeProviderSpy.mockClear();
    readOnlyTranscriptSpy.mockClear();
  });

  it("Should bind a User-scope session to the live runtime through @user", () => {
    renderView({ id: "sess-1", agent_name: "general", scope: "user" });

    expect(runtimeProviderSpy).toHaveBeenCalledWith("@user");
    expect(readOnlyTranscriptSpy).not.toHaveBeenCalled();
  });

  it("Should keep a legacy session without a scope in the read-only history", () => {
    renderView({ id: "sess-1", agent_name: "general" });

    expect(readOnlyTranscriptSpy).toHaveBeenCalled();
    expect(runtimeProviderSpy).not.toHaveBeenCalled();
  });

  it("Should hold the window's frames under one busy status while its session loads", () => {
    renderView(undefined, true);

    const region = screen.getByTestId("session-route-loading");
    expect(region).toHaveAttribute("role", "status");
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading session");
    // The rail, the transcript rows and the composer card are each in the place they will fill.
    expect(screen.getByTestId("session-sidebar-skeleton")).toBeInTheDocument();
    expect(region.querySelector('[data-slot="skeleton-text"]')).not.toBeNull();
    expect(screen.getByTestId("composer-skeleton")).toBeInTheDocument();
    // One announcement for the whole window: the placeholders are never statuses of their own.
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(runtimeProviderSpy).not.toHaveBeenCalled();
  });
});
