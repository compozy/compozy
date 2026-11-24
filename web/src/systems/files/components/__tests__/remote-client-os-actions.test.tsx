// Suite: Hide client-OS actions for remote sessions and computers (UT-090, US-012.AC-2)
// Invariant: Open in editor, Reveal in Finder, and the native folder picker are absent for
//            remote sessions/computers and present for local ones.
// Boundary IN: OpenInButton, OpenRootInMenu, useOpenFileInEditor, useDirectoryPathField.
// Boundary OUT: Desktop shell IPC bridge execution.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ComputerScopeContext } from "@/lib/computer-scope-context";
import * as osSystem from "@/systems/os";
import * as openInTargets from "../../lib/open-in-targets";
import * as openInHook from "../../hooks/use-open-in";
import { OpenInButton, OpenRootInMenu } from "../open-in-button";

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));

describe("Remote client-OS action suppression (UT-090)", () => {
  beforeEach(() => {
    vi.spyOn(osSystem, "desktopShellBridge").mockReturnValue({
      files: {
        listOpenTargets: vi.fn().mockResolvedValue([]),
        openInEditor: vi.fn().mockResolvedValue({ status: "opened" }),
        openInTarget: vi.fn().mockResolvedValue({ status: "opened" }),
      },
    } as never);
    vi.spyOn(openInTargets, "canOpenInTargets").mockReturnValue(true);
    vi.spyOn(openInHook, "useOpenIn").mockReturnValue({
      busy: false,
      primary: { id: "vscode", kind: "editor", label: "VS Code" },
      targets: [
        { id: "vscode", kind: "editor", label: "VS Code" },
        { id: "finder", kind: "reveal", label: "Reveal in Finder" },
      ],
      open: vi.fn(),
    } as never);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("UT-090: Should render Open in editor and Reveal in Finder controls for local sessions and hide them for remote sessions", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const localRoot = {
      kind: "session" as const,
      workspaceId: "ws_1",
      sessionId: "sess_1",
    };

    const { rerender } = render(
      <QueryClientProvider client={queryClient}>
        <ComputerScopeContext value="local">
          <OpenInButton path="README.md" root={localRoot} />
          <OpenRootInMenu root={localRoot} />
        </ComputerScopeContext>
      </QueryClientProvider>
    );

    expect(screen.getByTestId("open-in-split-button")).toBeInTheDocument();
    expect(screen.getByTestId("files-root-open-in")).toBeInTheDocument();

    rerender(
      <QueryClientProvider client={queryClient}>
        <ComputerScopeContext value="cmp_7f3k2q9x">
          <OpenInButton path="README.md" root={localRoot} />
          <OpenRootInMenu root={localRoot} />
        </ComputerScopeContext>
      </QueryClientProvider>
    );

    expect(screen.queryByTestId("open-in-split-button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("files-root-open-in")).not.toBeInTheDocument();
  });

  it("UT-090: Should hide Open in editor hook on remote sessions while keeping it available locally", () => {
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <ComputerScopeContext value="local">{children}</ComputerScopeContext>
    );
    const remoteWrapper = ({ children }: { children: ReactNode }) => (
      <ComputerScopeContext value="cmp_7f3k2q9x">{children}</ComputerScopeContext>
    );

    const localResult = renderHook(() => osSystem.useOpenFileInEditor(), {
      wrapper: localWrapper,
    });
    const remoteResult = renderHook(() => osSystem.useOpenFileInEditor(), {
      wrapper: remoteWrapper,
    });

    expect(localResult.result.current).toBeTypeOf("function");
    expect(remoteResult.result.current).toBeUndefined();
  });
});
