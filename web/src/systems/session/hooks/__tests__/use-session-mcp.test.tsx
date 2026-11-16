import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import type { ReactNode } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { SettingsMCPServerEntry } from "@/systems/settings";

import { sessionKeys } from "../../lib/query-keys";
import {
  sessionMCPAllReadyFixture,
  sessionMCPFailedFixture,
  sessionMCPFixture,
  sessionMCPReloadFailedFixture,
  sessionMCPRowFixture,
  sessionMCPSummaryFixture,
  sessionMCPWaitingFixture,
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
  mcpPanelAnchor: (element: HTMLElement) => ({ element, virtual: element }),
  mcpAllowPanelLaunch: () => null,
  MCPAllowPanel: () => null,
  MCPAnchoredPanel: () => null,
  MCPIssuePanel: () => null,
  MCPSignInPopover: () => null,
  ProviderLogo: () => null,
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => vi.fn() }));

import {
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
// pending (where a failed reload shows); Allow binds the row's own project and digest and never
// turns an in-flight Allow into a trip to Settings; Check again replaces the cached read with the
// daemon's answer and says plainly when the server still fails.
// Owning layer: session hooks (and `SessionComposerMCP`) over the session MCP adapter (HTTP mocked
// with MSW for the check) and the Settings MCP hooks.
// Canonical suite: this file.

const WAITING_ROW = sessionMCPWaitingFixture.compozy_servers[0]!;
const WAITING_ENTRY = {
  name: "sentry",
  owner: "",
  approval: { definition_digest: "sha256:sentry" },
} as unknown as SettingsMCPServerEntry;

const httpServer = setupServer();
beforeAll(() => httpServer.listen({ onUnhandledFrame: "error" }));
afterEach(() => httpServer.resetHandlers());
afterAll(() => httpServer.close());

function wrapper(queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
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
    act(() => result.current.allow(WAITING_ROW));
    expect(onUnresolved).not.toHaveBeenCalled();
    expect(mutate).not.toHaveBeenCalled();
  });

  it("Should review a waiting row beside its button and allow it with the row's project and digest", () => {
    const { mutate } = stubSettings();
    const onUnresolved = vi.fn();
    const trigger = document.createElement("button");

    const { result } = renderHook(
      () =>
        useSessionMCPActions({
          workspaceId: "ws_project",
          sessionId: "sess-project",
          profile: undefined,
          enabled: false,
          onUnresolved,
        }),
      { wrapper: wrapper() }
    );

    act(() => result.current.act(WAITING_ROW, "allow", trigger));
    expect(result.current.panel).toMatchObject({
      kind: "allow",
      target: { name: "sentry", owner: "" },
      anchor: { element: trigger },
    });
    act(() => result.current.allow(WAITING_ROW));
    expect(mutate).toHaveBeenCalledWith(
      {
        name: "sentry",
        body: { workspace_id: "ws_project", definition_digest: "sha256:sentry" },
      },
      expect.anything()
    );
    expect(onUnresolved).not.toHaveBeenCalled();
  });

  it("Should send a waiting row without a project or digest to Settings instead of allowing it", () => {
    const { mutate } = stubSettings();
    const onUnresolved = vi.fn();
    const { result } = renderHook(
      () =>
        useSessionMCPActions({
          workspaceId: "ws_project",
          sessionId: "sess-project",
          profile: undefined,
          enabled: false,
          onUnresolved,
        }),
      { wrapper: wrapper() }
    );

    act(() => result.current.allow(sessionMCPRowFixture({ name: "x", state: "waiting_approval" })));
    expect(onUnresolved).toHaveBeenCalledOnce();
    expect(mutate).not.toHaveBeenCalled();
  });

  it("Should check a failed server again, keep the panel open while it still fails, and close it once it works", async () => {
    stubSettings();
    const failedRow = sessionMCPFailedFixture.compozy_servers[0]!;
    const stillFailing = sessionMCPFixture({
      compozy: [{ ...failedRow, reason: "Connection refused" }],
    });
    const working = sessionMCPFixture({
      compozy: [{ ...failedRow, state: "applies_next_message", reason: "" }],
    });
    const answers = [stillFailing, working];
    const checks: URL[] = [];
    httpServer.use(
      http.post(
        "*/api/workspaces/:workspace/sessions/:session/mcp/servers/:name/check",
        ({ request }) => {
          checks.push(new URL(request.url));
          return HttpResponse.json(answers[checks.length - 1]);
        }
      )
    );
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(
      sessionKeys.mcp("ws_project", "sess-project"),
      sessionMCPFailedFixture
    );

    const { result } = renderHook(
      () =>
        useSessionMCPActions({
          workspaceId: "ws_project",
          sessionId: "sess-project",
          profile: "work",
          enabled: false,
          onUnresolved: vi.fn(),
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() => result.current.act(failedRow, "check", document.createElement("button")));
    expect(result.current.panel?.kind).toBe("check");
    act(() => result.current.check({ name: "pagerduty", owner: "" }));
    await waitFor(() => expect(result.current.stillFailing).toBe(true));
    expect(result.current.panel?.kind).toBe("check");
    expect(queryClient.getQueryData(sessionKeys.mcp("ws_project", "sess-project"))).toEqual(
      stillFailing
    );
    expect(checks[0]?.pathname).toMatch(
      /\/sessions\/sess-project\/mcp\/servers\/pagerduty\/check$/
    );
    expect(checks[0]?.searchParams.get("profile")).toBe("work");
    expect(checks[0]?.searchParams.has("owner")).toBe(false);

    act(() => result.current.check({ name: "pagerduty", owner: "" }));
    await waitFor(() => expect(result.current.panel).toBeNull());
    expect(result.current.stillFailing).toBe(false);
    expect(queryClient.getQueryData(sessionKeys.mcp("ws_project", "sess-project"))).toEqual(
      working
    );
  });
});
