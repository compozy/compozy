import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { SettingsMCPServerEntry } from "@/systems/settings";

import {
  sessionMCPAllReadyFixture,
  sessionMCPReloadFailedFixture,
  sessionMCPRowFixture,
  sessionMCPSummaryFixture,
} from "../../mocks/session-mcp-fixtures";
import { primarySessionFixture } from "../../mocks/fixtures";

vi.mock("../../adapters/session-mcp-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-mcp-api")>()),
  fetchSessionMCP: vi.fn(),
}));

// No `importOriginal`: the Settings barrel reaches back into the session system, so loading it here
// would bind the hook under test to the real Settings module before this mock exists.
vi.mock("@/systems/settings", () => ({
  SettingsApiError: class SettingsApiError extends Error {},
  deriveMCPAuthFilter: vi.fn(),
  useSettingsMCPServers: vi.fn(),
  useAllowMCPServer: vi.fn(),
  useMCPDefinitionAuthorization: vi.fn(),
  mcpApprovalTarget: vi.fn(),
  MCPSignInPopover: () => null,
  ProviderLogo: () => null,
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => vi.fn() }));

import {
  mcpApprovalTarget,
  useAllowMCPServer,
  useMCPDefinitionAuthorization,
  useSettingsMCPServers,
} from "@/systems/settings";

import { fetchSessionMCP } from "../../adapters/session-mcp-api";
import { SessionComposerMCP } from "../../components/session-composer-mcp";
import { useSessionMCP } from "../use-session-mcp";
import { useSessionMCPActions } from "../use-session-mcp-actions";

// Suite: the composer's session MCP data hooks and the container that wires them (spec
// mcp-extension-hot-activation, `_uiux.md` S3).
// Invariant: the popover reads a session's MCP servers in that session's own lens — a User
// session (empty workspace ID) included — the closed control reads them only while a change is
// pending (where a failed reload shows), and its row actions never turn an in-flight Allow into
// a trip to Settings.
// Owning layer: session hooks (and `SessionComposerMCP`) over the session MCP adapter and the
// Settings MCP hooks.
// Canonical suite: this file.

const WAITING_ROW = sessionMCPRowFixture({ name: "notion", state: "waiting_approval" });
const WAITING_ENTRY = {
  name: "notion",
  owner: "",
  approval: { definition_digest: "sha256:notion" },
} as unknown as SettingsMCPServerEntry;

function wrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function stubSettings({ allowPending = false }: { allowPending?: boolean } = {}) {
  const mutate = vi.fn();
  vi.mocked(useSettingsMCPServers).mockReturnValue({
    isPending: false,
    data: { mcp_servers: [WAITING_ENTRY] },
  } as unknown as ReturnType<typeof useSettingsMCPServers>);
  vi.mocked(useAllowMCPServer).mockReturnValue({
    isPending: allowPending,
    mutate,
  } as unknown as ReturnType<typeof useAllowMCPServer>);
  vi.mocked(useMCPDefinitionAuthorization).mockReturnValue({
    authorize: { requestAuthorize: vi.fn() },
  } as unknown as ReturnType<typeof useMCPDefinitionAuthorization>);
  vi.mocked(mcpApprovalTarget).mockReturnValue({ workspace_id: "ws_project" });
  return { mutate };
}

describe("useSessionMCP", () => {
  beforeEach(() => {
    vi.mocked(fetchSessionMCP).mockReset();
  });

  it("Should read a User session's MCP servers when the popover opens", async () => {
    vi.mocked(fetchSessionMCP).mockResolvedValue(sessionMCPAllReadyFixture);

    const { result } = renderHook(
      () => useSessionMCP("", "sess-user", undefined, { open: true, pendingReload: false }),
      { wrapper: wrapper() }
    );

    await waitFor(() => expect(result.current.data).toEqual(sessionMCPAllReadyFixture));
    expect(fetchSessionMCP).toHaveBeenCalledWith("", "sess-user", undefined, expect.anything());
  });

  it("Should read a failed reload for the closed control while a change is pending", async () => {
    vi.mocked(fetchSessionMCP).mockResolvedValue(sessionMCPReloadFailedFixture);

    const { result } = renderHook(
      () =>
        useSessionMCP("ws_project", "sess-project", undefined, {
          open: false,
          pendingReload: true,
        }),
      { wrapper: wrapper() }
    );

    await waitFor(() => expect(result.current.data?.last_reload?.outcome).toBe("failed"));
  });

  it("Should not read the servers while the control is closed with nothing pending", () => {
    renderHook(
      () =>
        useSessionMCP("ws_project", "sess-project", undefined, {
          open: false,
          pendingReload: false,
        }),
      { wrapper: wrapper() }
    );

    expect(fetchSessionMCP).not.toHaveBeenCalled();
  });
});

describe("SessionComposerMCP", () => {
  beforeEach(() => {
    vi.mocked(fetchSessionMCP).mockReset();
  });

  it("Should name a failed reload on the closed control without opening it", async () => {
    stubSettings();
    vi.mocked(fetchSessionMCP).mockResolvedValue(sessionMCPReloadFailedFixture);
    const Wrapper = wrapper();

    render(
      <Wrapper>
        <UIProvider>
          <SessionComposerMCP
            session={{
              ...primarySessionFixture,
              mcp_summary: sessionMCPSummaryFixture(sessionMCPReloadFailedFixture),
            }}
          />
        </UIProvider>
      </Wrapper>
    );

    expect(
      await screen.findByRole("button", { name: /couldn't start with the new servers/ })
    ).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-attention")).toBeInTheDocument();
  });
});

describe("useSessionMCPActions", () => {
  it("Should resolve a User session's rows in the User lens", () => {
    stubSettings();

    renderHook(
      () =>
        useSessionMCPActions({
          workspaceId: "",
          sessionId: "sess-user",
          profile: undefined,
          enabled: true,
          onUnresolved: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    expect(useSettingsMCPServers).toHaveBeenLastCalledWith({ scope: "user" }, { enabled: true });
  });

  it("Should resolve a named profile's User session without a project", () => {
    stubSettings();

    renderHook(
      () =>
        useSessionMCPActions({
          workspaceId: "",
          sessionId: "sess-user",
          profile: "work",
          enabled: true,
          onUnresolved: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    expect(useSettingsMCPServers).toHaveBeenLastCalledWith(
      { scope: "profile", profile: "work" },
      { enabled: true }
    );
  });

  it("Should ignore a second Allow while one is in flight instead of opening Settings", () => {
    const { mutate } = stubSettings({ allowPending: true });
    const onUnresolved = vi.fn();

    const { result } = renderHook(
      () =>
        useSessionMCPActions({
          workspaceId: "ws_project",
          sessionId: "sess-project",
          profile: undefined,
          enabled: true,
          onUnresolved,
        }),
      { wrapper: wrapper() }
    );

    expect(result.current.allowing).toBe(true);
    act(() => result.current.act(WAITING_ROW, "allow"));
    expect(onUnresolved).not.toHaveBeenCalled();
    expect(mutate).not.toHaveBeenCalled();
  });
});
