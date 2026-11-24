// Suite: ComputerDetailsPanel live tabs and placement (S8 / U7)
// Invariant: renders right-docked at top-0 in pane mode, shows 4-hop diagnosis + Retry + Copy,
// routes with mismatch removal, admin vs read-only permissions, and conditional Update tab only
// when incompatible.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "@/systems/os";
import { takePendingTerminalCommand } from "@/systems/terminal/parts";

import { useComputerProviders } from "../../hooks/use-computer-providers";
import { ComputerDetailsPanel } from "../computer-details-panel";
import {
  DETAILS_DEVBOX,
  DETAILS_GPU_VM,
  DETAILS_INCOMPATIBLE_MANUAL,
  DETAILS_WORKSTATION,
} from "../../mocks/remote-computers-fixtures-lane-b";

describe("ComputerDetailsPanel", () => {
  it("hides Update tab for healthy computers and renders pane placement at top-0", () => {
    render(
      <ComputerDetailsPanel
        computer={DETAILS_DEVBOX}
        open
        placement="pane"
        showRouteReorder={false}
      />
    );

    const panel = screen.getByLabelText("devbox details");
    expect(panel.className).toContain("top-0");
    expect(screen.getByRole("tab", { name: "Status" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Routes/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Permissions" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Providers" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /Update/i })).not.toBeInTheDocument();
  });

  it("renders 4-hop diagnosis and removes mismatched learned routes via live callback", () => {
    const onRetry = vi.fn();
    const onRemoveRoute = vi.fn();

    render(
      <ComputerDetailsPanel
        computer={DETAILS_WORKSTATION}
        onRemoveRoute={onRemoveRoute}
        onRetryConnection={onRetry}
        open
        placement="pane"
        showRouteReorder={false}
      />
    );

    expect(screen.getByTestId("computer-details-diagnosis-steps")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Retry now/i }));
    expect(onRetry).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("tab", { name: /Routes/i }));
    expect(screen.getByText("Different computer at this address")).toBeInTheDocument();
    const removeButtons = screen.getAllByRole("button", { name: /^Remove$/i });
    fireEvent.click(removeButtons[0]!);
    expect(onRemoveRoute).toHaveBeenCalledTimes(1);
  });

  it("shows Update tab when incompatible and supports SSH live update and manual compozy update", () => {
    const onStartUpdate = vi.fn();
    const { rerender } = render(
      <ComputerDetailsPanel
        computer={DETAILS_GPU_VM}
        initialTab="update"
        liveUpdateState="idle"
        onStartUpdate={onStartUpdate}
        open
        placement="pane"
      />
    );

    expect(screen.getByRole("tab", { name: /Update/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Update gpu-vm/i }));
    expect(onStartUpdate).toHaveBeenCalledTimes(1);

    const onCheckVersion = vi.fn();
    rerender(
      <ComputerDetailsPanel
        computer={DETAILS_INCOMPATIBLE_MANUAL}
        initialTab="update"
        onCheckVersion={onCheckVersion}
        open
        placement="pane"
      />
    );
    expect(screen.getByText("compozy update")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Check version again/i }));
    expect(onCheckVersion).toHaveBeenCalledTimes(1);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // UT-091 (remote-computers US-013): Providers tab reads `/c/{id}/api/providers` and clicking
  // `Sign in on devbox` opens a terminal on `cmp_7f3k2q9x` running `claude login`.
  it("loads live provider state and launches claude login on devbox from Providers tab (UT-091)", async () => {
    const DEVBOX_ID = "cmp_7f3k2q9x";
    const calls: { method: string; url: string }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const request = input instanceof Request ? input : new Request(String(input), init);
        const url = new URL(request.url);
        calls.push({ method: request.method, url: url.pathname });
        if (url.pathname === `/c/${DEVBOX_ID}/api/providers`) {
          return Response.json({
            providers: [
              {
                name: "claude",
                display_name: "Claude",
                enabled: true,
                auth_status: {
                  state: "needs_login",
                  message: "Sign in required",
                  login: { executable: "claude", configured: true, presence: "present" },
                },
              },
              {
                name: "codex",
                display_name: "Codex",
                enabled: true,
                auth_status: {
                  state: "authenticated",
                  message: "dev@example.com",
                  login: { executable: "codex", configured: true, presence: "present" },
                },
              },
            ],
          });
        }
        if (
          request.method === "POST" &&
          url.pathname === `/c/${DEVBOX_ID}/api/workspaces/%40user/terminals`
        ) {
          return Response.json({
            terminal: {
              id: "term_login_1",
              workspace_id: "@user",
              profile_id: "prof_default",
              profile_name: "default",
              title: "Claude sign-in",
              shell: "/bin/bash",
              cwd: "/home/dev",
              mode: "pty",
              state: "running",
              viewers: 0,
              bound_run: null,
              capabilities: { interactive: true },
              created_at: "2026-11-24T10:00:00Z",
            },
          });
        }
        return new Response("not found", { status: 404 });
      })
    );

    const userOpen = vi.fn(async () => ({
      accepted: true,
      completion: Promise.resolve(true),
      windowId: "win-term-1",
    }));
    const shell = {
      coordinator: { userOpen },
      manager: { getState: () => ({ windows: {}, focusedId: null }) },
    } as unknown as OsShellHandle;
    const devbox = { ...DETAILS_DEVBOX, id: DEVBOX_ID, name: "devbox", providers: [] };

    function LiveProvidersHarness() {
      const live = useComputerProviders(devbox, {
        openTerminalWindow: spec => shell.coordinator.userOpen(spec),
      });
      return (
        <ComputerDetailsPanel
          computer={{ ...devbox, providers: live.providers }}
          initialTab="providers"
          onRetryProviders={live.refetch}
          onSignInProvider={(provider, command) => {
            void live.signInProvider(provider, command);
          }}
          open
          placement="pane"
          providersError={live.error}
          providersLoading={live.isLoading}
          signingInProviderId={live.signingInProviderId}
        />
      );
    }

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <OsShellContext value={shell}>
          <LiveProvidersHarness />
        </OsShellContext>
      </QueryClientProvider>
    );

    const signInButton = await screen.findByRole("button", { name: "Sign in on devbox" });
    const claudeRow = signInButton.closest('[data-provider-id="claude"]');
    expect(claudeRow).not.toBeNull();
    expect(claudeRow).toHaveTextContent("Claude");
    expect(claudeRow).toHaveTextContent("Signed out");

    fireEvent.click(signInButton);

    await waitFor(() =>
      expect(userOpen).toHaveBeenCalledWith(
        expect.objectContaining({
          app: "terminal",
          instanceKey: "term_login_1",
          route: expect.objectContaining({
            pathname: "/terminal/term_login_1",
            search: expect.objectContaining({ computer: DEVBOX_ID, workspace: "@user" }),
          }),
        })
      )
    );
    expect(calls).toContainEqual({
      method: "POST",
      url: `/c/${DEVBOX_ID}/api/workspaces/%40user/terminals`,
    });
    expect(takePendingTerminalCommand(DEVBOX_ID, "term_login_1")).toBe("claude login");
  });

  it("hides Sign in on <name> when terminal:operate is missing", () => {
    const devboxWithoutTerminal = {
      ...DETAILS_DEVBOX,
      scopes: ["sessions:operate", "files:read"] as const,
      providers: [{ id: "claude", name: "Claude", status: "signed_out" as const }],
    };
    render(
      <ComputerDetailsPanel
        computer={devboxWithoutTerminal}
        initialTab="providers"
        open
        placement="pane"
      />
    );
    expect(screen.queryByRole("button", { name: /Sign in on devbox/i })).toBeNull();
    expect(
      screen.getByText(/Turn on Terminals permission on devbox to sign in remotely/i)
    ).toBeInTheDocument();
  });
});
