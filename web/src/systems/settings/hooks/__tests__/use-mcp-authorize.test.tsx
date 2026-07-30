import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/settings/adapters/settings-mcp-auth-api", () => ({
  beginSettingsMCPAuth: vi.fn(),
  exchangeSettingsMCPAuth: vi.fn(),
  logoutSettingsMCPAuth: vi.fn(),
}));

import {
  beginSettingsMCPAuth,
  exchangeSettingsMCPAuth,
} from "@/systems/settings/adapters/settings-mcp-auth-api";
import { useMCPAuthorize } from "@/systems/settings/hooks/use-mcp-authorize";

const filter = { scope: "workspace" as const, workspace_id: "ws-alpha" };
const prior = { status: "needs_login", tokenPresent: false };

const beginResponse = {
  authorization_url: "https://auth.linear.app/oauth/authorize?state=x",
  callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
  expires_at: "2026-07-15T00:05:00Z",
  manual_supported: true,
  state: "compozy_mcp_x",
};

function authenticatedStatus() {
  return {
    server_name: "linear",
    scope: "workspace",
    status: "authenticated",
    token_present: true,
    refreshable: true,
  };
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useMCPAuthorize", () => {
  it("begins authorization and exposes the live copyable URL from auth/begin", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => {
      result.current.beginAuthorize(filter, "linear", prior);
      result.current.beginAuthorize(filter, "linear", prior);
    });

    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    expect(result.current.begin?.authorization_url).toBe(beginResponse.authorization_url);
    expect(result.current.phase).toBe("waiting");
    expect(beginSettingsMCPAuth).toHaveBeenCalledOnce();
    expect(beginSettingsMCPAuth).toHaveBeenCalledWith("linear", filter, { mode: "automatic" });
  });

  it("requires explicit confirmation before beginning a scope escalation", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    let started = true;
    act(() => {
      started = result.current.beginScopeEscalation(
        filter,
        "linear",
        prior,
        ["read", "write"],
        false
      );
    });
    expect(started).toBe(false);
    expect(beginSettingsMCPAuth).not.toHaveBeenCalled();

    act(() => {
      started = result.current.beginScopeEscalation(
        filter,
        "linear",
        prior,
        ["read", " write "],
        true
      );
    });
    expect(started).toBe(true);
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    expect(beginSettingsMCPAuth).toHaveBeenCalledWith("linear", filter, {
      approve_scope_escalation: true,
      approved_scopes: ["read", "write"],
      mode: "automatic",
    });
  });

  it("marks the flow failed when begin errors, preserving the prior status", async () => {
    vi.mocked(beginSettingsMCPAuth).mockRejectedValue(new Error("provider unreachable"));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));

    await waitFor(() => expect(result.current.phase).toBe("failed"));
    expect(result.current.error).toBe("provider unreachable");
    expect(result.current.prior).toEqual(prior);
  });

  it("keeps an older begin completion from overwriting a same-server retry", async () => {
    let resolveFirst: ((value: typeof beginResponse) => void) | undefined;
    let resolveSecond: ((value: typeof beginResponse) => void) | undefined;
    vi.mocked(beginSettingsMCPAuth)
      .mockReturnValueOnce(
        new Promise(resolve => {
          resolveFirst = resolve;
        })
      )
      .mockReturnValueOnce(
        new Promise(resolve => {
          resolveSecond = resolve;
        })
      );
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => {
      result.current.beginAuthorize(filter, "linear", prior);
    });
    await waitFor(() => expect(result.current.phase).toBe("beginning"));
    act(() => result.current.cancel());
    act(() => {
      result.current.beginAuthorize(filter, "linear", prior);
    });
    await waitFor(() => expect(result.current.phase).toBe("beginning"));

    await act(async () => {
      resolveFirst?.(beginResponse);
    });
    resolveSecond?.({
      ...beginResponse,
      authorization_url: "https://auth.linear.app/oauth/authorize?state=retry",
      state: "compozy_mcp_retry",
    });
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    expect(result.current.begin?.state).toBe("compozy_mcp_retry");
  });

  it("completes manual exchange with a full redirect URL only on a confirmed credential", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue(authenticatedStatus());
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    act(() => result.current.enterManual());
    await waitFor(() => expect(result.current.phase).toBe("manual"));
    act(() =>
      result.current.submitManual(
        "http://127.0.0.1:2123/api/mcp/oauth/callback?code=code-123&state=x"
      )
    );

    await waitFor(() => expect(result.current.phase).toBe("confirmed"));
    expect(exchangeSettingsMCPAuth).toHaveBeenCalledWith("linear", filter, {
      redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=code-123&state=x",
    });
    expect(beginSettingsMCPAuth).toHaveBeenLastCalledWith("linear", filter, { mode: "manual" });
  });

  it("Should notify an accepted manual confirmation after the hook unmounts", async () => {
    let resolveExchange!: (status: ReturnType<typeof authenticatedStatus>) => void;
    const exchange = new Promise<ReturnType<typeof authenticatedStatus>>(resolve => {
      resolveExchange = resolve;
    });
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    vi.mocked(exchangeSettingsMCPAuth).mockReturnValue(exchange);
    const onConfirmed = vi.fn();
    const { wrapper } = createWrapper();
    const { result, unmount } = renderHook(() => useMCPAuthorize({ onConfirmed }), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    act(() => result.current.enterManual());
    await waitFor(() => expect(result.current.phase).toBe("manual"));
    act(() =>
      result.current.submitManual(
        "http://127.0.0.1:2123/api/mcp/oauth/callback?code=delayed&state=x"
      )
    );
    await waitFor(() => expect(result.current.phase).toBe("exchanging"));

    unmount();
    resolveExchange(authenticatedStatus());

    await waitFor(() => expect(onConfirmed).toHaveBeenCalledWith("linear"));
  });

  it("accepts a full redirect URL for the exchange", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue(authenticatedStatus());
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    act(() => result.current.enterManual());
    await waitFor(() => expect(result.current.phase).toBe("manual"));
    const redirect = "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc&state=x";
    act(() => result.current.submitManual(redirect));

    await waitFor(() => expect(result.current.phase).toBe("confirmed"));
    expect(exchangeSettingsMCPAuth).toHaveBeenCalledWith("linear", filter, {
      redirect_url: redirect,
    });
  });

  it("never flips to success when the exchange returns without a confirmed token", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue({
      ...authenticatedStatus(),
      token_present: false,
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    act(() => result.current.enterManual());
    await waitFor(() => expect(result.current.phase).toBe("manual"));
    act(() =>
      result.current.submitManual(
        "http://127.0.0.1:2123/api/mcp/oauth/callback?code=rejected&state=x"
      )
    );

    await waitFor(() => expect(result.current.phase).toBe("failed"));
    expect(result.current.error).toBe("The provider did not return a confirmed credential.");
  });

  it("auto-confirms from a polled status only on authenticated && token_present", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));

    // A tools/list-style success without a token must not flip the UI.
    act(() => result.current.acknowledgeStatus("needs_login", false));
    expect(result.current.phase).toBe("waiting");

    act(() => result.current.acknowledgeStatus("authenticated", true));
    expect(result.current.phase).toBe("confirmed");
  });

  it("auto-dismisses an accepted confirmation and notifies the latest boundary once", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const firstConfirmed = vi.fn();
    const latestConfirmed = vi.fn();
    const { wrapper } = createWrapper();
    const { result, rerender } = renderHook(
      ({ onConfirmed }: { onConfirmed: (server: string) => void }) =>
        useMCPAuthorize({ dismissOnConfirmation: true, onConfirmed }),
      { initialProps: { onConfirmed: firstConfirmed }, wrapper }
    );

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));

    rerender({ onConfirmed: latestConfirmed });
    act(() => result.current.acknowledgeStatus("authenticated", true));

    expect(result.current.phase).toBe("idle");
    expect(firstConfirmed).not.toHaveBeenCalled();
    expect(latestConfirmed).toHaveBeenCalledOnce();
    expect(latestConfirmed).toHaveBeenCalledWith("linear");
  });

  it("cancels back to idle", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(), { wrapper });

    act(() => result.current.beginAuthorize(filter, "linear", prior));
    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    act(() => result.current.cancel());

    expect(result.current.phase).toBe("idle");
  });
});
