// Suite: Web root sign-in gate, /sign-in route, 4401 transition, and grace mode (S15 / U0)
// Test IDs: UT-120, UT-138
// Invariant: Root gate runs before _app's beforeLoad; enforced signed-out installations
//            redirect to /sign-in?redirect=<href> without calling gated routes; dev builds
//            (gate:"off") never render U0; hosted origins never read /api/account; grace mode
//            renders the app shell with GateGraceBanner and transitions on grace end; 4401
//            moves the app to /sign-in without a reload loop.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  ACCOUNT_SIGN_IN_CLOSE_CODE,
  isAccountSignInCloseCode,
  reportAccountGateSignal,
  resetAccountGateSignalForTest,
} from "@/lib/account-gate-signal";
import { resetHostedBrowserAuth, setHostedBrowserMode } from "@/lib/hosted-browser-auth";
import {
  GateGraceBanner,
  accountKeys,
  useAccountGateWatcher,
  type AccountSignInStreamHandlers,
  type AccountStatusPayload,
} from "@/systems/account";
import { guardRootSignInGate } from "../-root-gate";

const mocks = vi.hoisted(() => ({
  fetchAccountStatus: vi.fn(),
  prepareAppProfile: vi.fn(),
  openAccountSignInStream: vi.fn(),
}));

vi.mock("@/systems/account/adapters/account-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/account/adapters/account-api")>();
  return {
    ...actual,
    fetchAccountStatus: mocks.fetchAccountStatus,
    openAccountSignInStream: mocks.openAccountSignInStream,
  };
});

function makeStatus(overrides: Partial<AccountStatusPayload> = {}): AccountStatusPayload {
  return {
    gate: "enforced",
    grace: null,
    state: "signed_out",
    source: null,
    signed_out_reason: null,
    pre_account_install: false,
    installation_linked: false,
    token_name: null,
    signed_in: false,
    email: "",
    name: "",
    provider: "",
    device: null,
    marketing_opt_in: false,
    plan: "free",
    subscription: null,
    remote_computers_used: 0,
    remote_computers_limit: 1,
    cloud_routes_used: 0,
    cloud_routes_limit: 0,
    payment_issue: false,
    recent_sign_ins: [],
    ...overrides,
  };
}

function buildTestRouter(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  function RootView() {
    useAccountGateWatcher();
    return (
      <div data-testid="test-root">
        <Outlet />
      </div>
    );
  }

  const rootRoute = createRootRouteWithContext<{ queryClient: QueryClient }>()({
    beforeLoad: ({ context, location }) => guardRootSignInGate(context.queryClient, location),
    component: RootView,
  });

  const signInRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/sign-in",
    validateSearch: (search: Record<string, unknown>) => ({
      redirect: typeof search.redirect === "string" ? search.redirect : undefined,
    }),
    component: () => <div data-testid="u0-sign-in-page">Sign in screen</div>,
  });

  const appRoute = createRoute({
    getParentRoute: () => rootRoute,
    id: "_app",
    beforeLoad: () => mocks.prepareAppProfile(),
    component: () => (
      <div data-testid="app-shell">
        <GateGraceBanner />
        <Outlet />
      </div>
    ),
  });

  const sessionsRoute = createRoute({
    getParentRoute: () => appRoute,
    path: "/sessions",
    component: () => <div data-testid="sessions-page">Sessions</div>,
  });

  const sessionDetailRoute = createRoute({
    getParentRoute: () => appRoute,
    path: "/sessions/$id",
    component: () => <div data-testid="session-detail-page">Session detail</div>,
  });

  const routeTree = rootRoute.addChildren([
    signInRoute,
    appRoute.addChildren([sessionsRoute, sessionDetailRoute]),
  ]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
    context: { queryClient },
  });

  return { queryClient, router };
}

describe("Root sign-in gate and 4401 watcher (UT-120, UT-138)", () => {
  let signInStreamHandlers: AccountSignInStreamHandlers | null = null;

  beforeEach(() => {
    resetHostedBrowserAuth();
    resetAccountGateSignalForTest();
    signInStreamHandlers = null;
    mocks.fetchAccountStatus.mockReset();
    mocks.prepareAppProfile.mockReset();
    mocks.prepareAppProfile.mockResolvedValue(undefined);
    mocks.openAccountSignInStream.mockReset();
    mocks.openAccountSignInStream.mockImplementation((handlers: AccountSignInStreamHandlers) => {
      signInStreamHandlers = handlers;
      return () => {
        signInStreamHandlers = null;
      };
    });
  });

  afterEach(() => {
    resetHostedBrowserAuth();
    resetAccountGateSignalForTest();
  });

  it("UT-120: redirects /sessions/sess-1 to /sign-in?redirect=/sessions/sess-1 before _app runs, then returns after sign-in", async () => {
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({ gate: "enforced", state: "signed_out", signed_in: false })
    );

    const { queryClient, router } = buildTestRouter("/sessions/sess-1");
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );

    expect(await screen.findByTestId("u0-sign-in-page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/sign-in");
    expect(router.state.location.search).toEqual({ redirect: "/sessions/sess-1" });
    expect(mocks.prepareAppProfile).not.toHaveBeenCalled();

    // After sign-in completes, navigating back to the saved redirect enters _app and prepares the profile.
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({
        gate: "enforced",
        state: "signed_in",
        signed_in: true,
        source: "session",
        email: "pedro@example.com",
      })
    );
    queryClient.setQueryData(
      accountKeys.status(),
      makeStatus({
        gate: "enforced",
        state: "signed_in",
        signed_in: true,
        source: "session",
        email: "pedro@example.com",
      })
    );

    await act(async () => {
      await router.navigate({ href: "/sessions/sess-1", replace: true });
    });

    expect(await screen.findByTestId("session-detail-page")).toBeInTheDocument();
    expect(mocks.prepareAppProfile).toHaveBeenCalledTimes(1);
  });

  it("UT-120: skips U0 completely when gate is off (dev build)", async () => {
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({ gate: "off", state: "signed_out", signed_in: false })
    );

    const { queryClient, router } = buildTestRouter("/sessions/sess-1");
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );

    expect(await screen.findByTestId("session-detail-page")).toBeInTheDocument();
    expect(screen.queryByTestId("u0-sign-in-page")).not.toBeInTheDocument();
    expect(mocks.prepareAppProfile).toHaveBeenCalledTimes(1);
  });

  it("UT-120: never reads /api/account on hosted origins (owned by U8)", async () => {
    setHostedBrowserMode(true);
    const { queryClient, router } = buildTestRouter("/sessions");

    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );

    expect(await screen.findByTestId("sessions-page")).toBeInTheDocument();
    expect(mocks.fetchAccountStatus).not.toHaveBeenCalled();
  });

  it("UT-120: 4401 stream close transitions the web app to U0 once without a reload loop", async () => {
    expect(isAccountSignInCloseCode(ACCOUNT_SIGN_IN_CLOSE_CODE)).toBe(true);
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({ gate: "enforced", state: "signed_in", signed_in: true, source: "session" })
    );

    const { queryClient, router } = buildTestRouter("/sessions");
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );

    expect(await screen.findByTestId("sessions-page")).toBeInTheDocument();
    const navigateSpy = vi.spyOn(router, "navigate");

    // Daemon signs out and closes operator streams with 4401.
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({
        gate: "enforced",
        state: "reauth_required",
        signed_in: false,
        signed_out_reason: "device_revoked",
      })
    );

    await act(async () => {
      reportAccountGateSignal({ kind: "sign_in_required", code: "sign_in_required" });
      reportAccountGateSignal({ kind: "sign_in_required", code: "sign_in_required" });
    });

    expect(await screen.findByTestId("u0-sign-in-page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/sign-in");
    expect(navigateSpy).toHaveBeenCalledTimes(1);

    // Additional 4401 signals while already on /sign-in do not loop navigation.
    await act(async () => {
      reportAccountGateSignal({ kind: "sign_in_required", code: "sign_in_required" });
    });
    expect(navigateSpy).toHaveBeenCalledTimes(1);
  });

  it("UT-138: renders app shell with non-dismissible grace banner in grace mode and moves to U0 when grace ends", async () => {
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({
        gate: "grace",
        state: "signed_out",
        signed_in: false,
        grace: {
          source: "policy",
          until: "2026-12-03T14:00:00Z",
          message: "",
        },
      })
    );

    const { queryClient, router } = buildTestRouter("/sessions");
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );

    expect(await screen.findByTestId("sessions-page")).toBeInTheDocument();
    const banner = await screen.findByTestId("gate-grace-banner");
    expect(banner).toHaveTextContent(
      /Sign-in is having problems\. CompozyOS keeps working until Dec 3, 14:00\./
    );
    expect(screen.getByRole("button", { name: /Sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Dismiss/i })).not.toBeInTheDocument();

    // Grace ends: daemon switches gate to enforced and emits account.state.
    mocks.fetchAccountStatus.mockResolvedValue(
      makeStatus({ gate: "enforced", state: "signed_out", signed_in: false, grace: null })
    );

    await waitFor(() => expect(signInStreamHandlers).not.toBeNull());
    await act(async () => {
      signInStreamHandlers?.onState?.({ state: "signed_out", signed_out_reason: null });
    });

    expect(await screen.findByTestId("u0-sign-in-page")).toBeInTheDocument();
  });
});
