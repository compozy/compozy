import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  prepareBrowserAuthorizeRedirect,
  reportHostedBrowserAccess,
  resetHostedBrowserAuth,
  savePendingApproval,
  setBrowserKeyStore,
  setHostedBrowserMode,
  type BrowserKeyStore,
} from "@/lib/hosted-browser-auth";

import { HostedBrowserAccessBoundary } from "../hosted-browser-access-boundary";
import { HostedComputerMenu } from "../hosted-computer-menu";
import { HostedPhoneApp } from "../hosted-phone-app";
import { useHostedBrowserContext } from "../../hooks/use-hosted-browser-context";

function createMemoryKeyStore(): BrowserKeyStore {
  let pair: CryptoKeyPair | null = null;
  return {
    async get() {
      return pair;
    },
    async set(next) {
      pair = next;
    },
    async delete() {
      pair = null;
    },
  };
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function setViewportWide(wide: boolean) {
  Object.defineProperty(window, "innerWidth", {
    configurable: true,
    value: wide ? 1280 : 390,
  });
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: (query: string) => ({
      matches: query.includes("min-width") ? wide : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }),
  });
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function DesktopHostedProbe() {
  const hosted = useHostedBrowserContext();
  if (!hosted) return <div data-testid="no-hosted-context" />;
  return (
    <div data-testid="desktop-hosted-shell">
      <HostedComputerMenu
        computer={hosted.computer}
        onSelectPeer={hosted.selectPeer}
        onSignOut={hosted.signOut}
        peers={hosted.peers}
      />
      <span data-testid="terminal-scope">{String(hosted.canOperateTerminal)}</span>
    </div>
  );
}

describe("HostedBrowserAccessBoundary & phone layout (UT-072, U8)", () => {
  beforeEach(() => {
    resetHostedBrowserAuth();
    sessionStorage.clear();
    localStorage.clear();
    setBrowserKeyStore(createMemoryKeyStore());
    setViewportWide(true);
    window.history.replaceState(null, "", "/");
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    resetHostedBrowserAuth();
    sessionStorage.clear();
    localStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  it("renders fallbackBoundary on non-hosted origins without probing /remote/v1/browser/*", () => {
    setHostedBrowserMode(false);
    const fetchMock = vi.fn<typeof globalThis.fetch>();
    vi.stubGlobal("fetch", fetchMock);

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary fallbackBoundary={<div data-testid="local-fallback" />}>
          <div data-testid="app-children" />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    expect(screen.getByTestId("local-fallback")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("renders Sign in to CompozyOS and redirects to /v1/browser/authorize with provider + PKCE", async () => {
    setHostedBrowserMode(true);
    const navigate = vi.fn<(url: string) => void>();
    const user = userEvent.setup();

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async input => {
        const path = new URL(
          input instanceof Request ? input.url : String(input),
          window.location.origin
        ).pathname;
        if (path === "/remote/v1/browser/status") return jsonResponse({ state: "active" });
        if (path === "/remote/v1/browser/peers") {
          return jsonResponse({ code: "remote_session_missing" }, 401);
        }
        return jsonResponse({}, 404);
      })
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary accessOptions={{ navigate }}>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await expect(
      screen.findByRole("heading", { name: "Sign in to CompozyOS" })
    ).resolves.toBeVisible();
    await user.click(screen.getByRole("button", { name: /Continue with Google/i }));

    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1));
    const redirectUrl = new URL(navigate.mock.calls[0]![0]);
    expect(redirectUrl.origin).toBe("https://cloud.compozy.com");
    expect(redirectUrl.pathname).toBe("/v1/browser/authorize");
    expect(redirectUrl.searchParams.get("provider")).toBe("google");
    expect(redirectUrl.searchParams.get("code_challenge_method")).toBe("S256");
    expect(redirectUrl.searchParams.get("public_jwk")).toBeTruthy();
    expect(redirectUrl.searchParams.get("state")).toBeTruthy();
  });

  it("renders needs_pro and opens https://cloud.compozy.com/billing/upgrade?hostname=... on Upgrade to Pro", async () => {
    setHostedBrowserMode(true);
    const openExternal = vi.fn<(url: string) => void>();
    const user = userEvent.setup();

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockResolvedValue(jsonResponse({ state: "plan_required" }))
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary accessOptions={{ openExternal }}>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await expect(
      screen.findByRole("heading", { name: /hosted route needs Pro/i })
    ).resolves.toBeVisible();
    await user.click(screen.getByRole("button", { name: /Upgrade to Pro/i }));
    expect(openExternal).toHaveBeenCalledWith(
      `https://cloud.compozy.com/billing/upgrade?hostname=${window.location.hostname}`
    );
  });

  it("polls waiting_approval every 5s and auto re-authorizes with the SAME public_jwk and fresh state/PKCE on approved (UT-072)", async () => {
    vi.useFakeTimers();
    setHostedBrowserMode(true);
    const navigate = vi.fn<(url: string) => void>();

    const firstAttempt = await prepareBrowserAuthorizeRedirect({
      hostname: window.location.hostname,
      provider: "google",
    });
    const firstUrl = new URL(firstAttempt.url);
    savePendingApproval({
      approvalId: "apr-wait-1",
      provider: "google",
      createdAt: Date.now(),
    });

    let approvalCalls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async input => {
        const path = new URL(
          input instanceof Request ? input.url : String(input),
          window.location.origin
        ).pathname;
        if (path === "/remote/v1/browser/status") return jsonResponse({ state: "active" });
        if (path === "/remote/v1/browser/approvals/apr-wait-1") {
          approvalCalls += 1;
          return jsonResponse({
            approval_id: "apr-wait-1",
            state: approvalCalls < 2 ? "pending" : "approved",
            expires_at: "2026-11-24T15:00:00Z",
          });
        }
        return jsonResponse({}, 404);
      })
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary accessOptions={{ navigate, pollIntervalMs: 5_000 }}>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(screen.getByRole("heading", { name: /Waiting for approval/i })).toBeInTheDocument();

    // Advance 5 seconds for the approval poll tick.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });

    expect(navigate).toHaveBeenCalledTimes(1);
    const secondUrl = new URL(navigate.mock.calls[0]![0]);
    // Same persisted P-256 public key, fresh state and PKCE challenge!
    expect(secondUrl.searchParams.get("public_jwk")).toBe(firstUrl.searchParams.get("public_jwk"));
    expect(secondUrl.searchParams.get("state")).not.toBe(firstUrl.searchParams.get("state"));
    expect(secondUrl.searchParams.get("code_challenge")).not.toBe(
      firstUrl.searchParams.get("code_challenge")
    );
  });

  it("handles browser_key_rebind_required by generating a new keypair and restarting authorize", async () => {
    setHostedBrowserMode(true);
    const navigate = vi.fn<(url: string) => void>();

    const firstAttempt = await prepareBrowserAuthorizeRedirect({
      hostname: window.location.hostname,
      provider: "google",
    });
    const firstPublicJwk = new URL(firstAttempt.url).searchParams.get("public_jwk");
    window.history.replaceState(
      null,
      "",
      `/remote/v1/browser/callback?code=c-rebind&state=${encodeURIComponent(firstAttempt.state)}`
    );

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async input => {
        const path = new URL(
          input instanceof Request ? input.url : String(input),
          window.location.origin
        ).pathname;
        if (path === "/remote/v1/browser/status") return jsonResponse({ state: "active" });
        if (path === "/remote/v1/browser/session") {
          return jsonResponse({ code: "browser_key_rebind_required" }, 401);
        }
        return jsonResponse({}, 404);
      })
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary accessOptions={{ navigate }}>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1));
    const rebindUrl = new URL(navigate.mock.calls[0]![0]);
    expect(rebindUrl.searchParams.get("public_jwk")).not.toBe(firstPublicJwk);
  });

  it("renders desktop shell with HostedComputerMenu peers, Sign out, and live device_removed transition", async () => {
    setHostedBrowserMode(true);
    const navigate = vi.fn<(url: string) => void>();
    const user = userEvent.setup();
    let deleteSessionCalled = false;

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async (input, init) => {
        const request = input instanceof Request ? input : new Request(String(input), init);
        const path = new URL(request.url, window.location.origin).pathname;
        if (path === "/remote/v1/browser/status") return jsonResponse({ state: "active" });
        if (path === "/remote/v1/browser/peers") {
          return jsonResponse({
            peers: [
              {
                computer_id: "cmp-self",
                name: "Studio Mac",
                hosted_url: `https://${window.location.hostname}`,
              },
              {
                computer_id: "cmp-peer",
                name: "Linux Box",
                hosted_url: "https://linux-box.compozy.run",
              },
            ],
          });
        }
        if (path === "/remote/v1/browser/session" && request.method === "DELETE") {
          deleteSessionCalled = true;
          return new Response(null, { status: 204 });
        }
        return jsonResponse({}, 404);
      })
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary accessOptions={{ navigate }}>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await expect(screen.findByTestId("desktop-hosted-shell")).resolves.toBeVisible();

    // Open header computer menu and switch to peer directly (no bridging).
    await user.click(screen.getByRole("button", { name: /Connected to Studio Mac/i }));
    await user.click(screen.getByRole("button", { name: /Linux Box/i }));
    expect(navigate).toHaveBeenCalledWith("https://linux-box.compozy.run");

    // Live device revocation unmounts the shell and displays device_removed immediately.
    act(() => {
      reportHostedBrowserAccess({ kind: "device_removed" });
    });
    await expect(
      screen.findByRole("heading", { name: "This device was removed from your account" })
    ).resolves.toBeVisible();

    // Clicking Sign in again returns to the sign-in screen.
    await user.click(screen.getByRole("button", { name: "Sign in again" }));
    await expect(
      screen.findByRole("heading", { name: "Sign in to CompozyOS" })
    ).resolves.toBeVisible();
    expect(deleteSessionCalled).toBe(false);
  });

  it("signs out via DELETE /remote/v1/browser/session without revoking the device", async () => {
    setHostedBrowserMode(true);
    const user = userEvent.setup();
    let deleteSessionCalled = false;

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async (input, init) => {
        const request = input instanceof Request ? input : new Request(String(input), init);
        const path = new URL(request.url, window.location.origin).pathname;
        if (path === "/remote/v1/browser/status") return jsonResponse({ state: "active" });
        if (path === "/remote/v1/browser/peers") {
          return jsonResponse({
            peers: [
              {
                computer_id: "cmp-self",
                name: "Studio Mac",
                hosted_url: `https://${window.location.hostname}`,
              },
            ],
          });
        }
        if (path === "/remote/v1/browser/session" && request.method === "DELETE") {
          deleteSessionCalled = true;
          return new Response(null, { status: 204 });
        }
        return jsonResponse({}, 404);
      })
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await expect(screen.findByTestId("desktop-hosted-shell")).resolves.toBeVisible();
    await user.click(screen.getByRole("button", { name: /Connected to Studio Mac/i }));
    await user.click(screen.getByTestId("phone-computer-menu-sign-out"));

    await waitFor(() => expect(deleteSessionCalled).toBe(true));
    await expect(
      screen.findByRole("heading", { name: "Sign in to CompozyOS" })
    ).resolves.toBeVisible();
  });

  it("renders approval_expired, not_approved, and another_account U8 gate states", async () => {
    setHostedBrowserMode(true);
    const cases: Array<{ reason: string; heading: RegExp }> = [
      { reason: "approval_expired", heading: /Approval request expired/i },
      { reason: "approval_denied", heading: /didn't approve this device/i },
      { reason: "account_mismatch", heading: /belongs to another account/i },
    ];

    for (const testCase of cases) {
      const prepared = await prepareBrowserAuthorizeRedirect({
        hostname: window.location.hostname,
        provider: "google",
      });
      window.history.replaceState(
        null,
        "",
        `/remote/v1/browser/callback?error=${testCase.reason}&state=${encodeURIComponent(prepared.state)}`
      );

      vi.stubGlobal(
        "fetch",
        vi.fn<typeof globalThis.fetch>().mockResolvedValue(jsonResponse({ state: "active" }))
      );

      const { unmount } = render(
        <QueryClientProvider client={createQueryClient()}>
          <HostedBrowserAccessBoundary>
            <DesktopHostedProbe />
          </HostedBrowserAccessBoundary>
        </QueryClientProvider>
      );

      await expect(screen.findByRole("heading", { name: testCase.heading })).resolves.toBeVisible();
      unmount();
    }
  });

  it("renders phone layout below 1024px and hides Terminal tab when terminal:operate scope is absent", async () => {
    setHostedBrowserMode(true);
    setViewportWide(false);
    const prepared = await prepareBrowserAuthorizeRedirect({
      hostname: window.location.hostname,
      provider: "google",
    });
    window.history.replaceState(
      null,
      "",
      `/remote/v1/browser/callback?code=c-narrow&state=${encodeURIComponent(prepared.state)}`
    );

    vi.stubGlobal(
      "fetch",
      vi.fn<typeof globalThis.fetch>().mockImplementation(async input => {
        const path = new URL(
          input instanceof Request ? input.url : String(input),
          window.location.origin
        ).pathname;
        if (path === "/remote/v1/browser/status") return jsonResponse({ state: "active" });
        if (path === "/remote/v1/browser/session") {
          return jsonResponse(
            {
              session_id: "bs-1",
              expires_at: Math.floor(Date.now() / 1000) + 3600,
              scopes: ["sessions:operate"],
            },
            201
          );
        }
        if (path === "/remote/v1/browser/peers") {
          return jsonResponse({
            peers: [
              {
                computer_id: "cmp-self",
                name: "Studio Mac",
                hosted_url: `https://${window.location.hostname}`,
              },
            ],
          });
        }
        if (path === "/api/sessions") {
          return jsonResponse({ sessions: [], page: { has_more: false, total: 0 } });
        }
        if (path === "/api/workspaces") {
          return jsonResponse({ workspaces: [] });
        }
        return jsonResponse({}, 404);
      })
    );

    render(
      <QueryClientProvider client={createQueryClient()}>
        <HostedBrowserAccessBoundary phoneApp={<HostedPhoneApp />}>
          <DesktopHostedProbe />
        </HostedBrowserAccessBoundary>
      </QueryClientProvider>
    );

    await expect(screen.findByTestId("hosted-phone-app")).resolves.toBeVisible();
    expect(screen.getByTestId("phone-tab-sessions")).toBeInTheDocument();
    expect(screen.getByTestId("phone-tab-approvals")).toBeInTheDocument();
    expect(screen.queryByTestId("phone-tab-terminal")).not.toBeInTheDocument();
  });
});
