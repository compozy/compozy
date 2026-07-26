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
  state: "agh_mcp_x",
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
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });

    await waitFor(() => expect(result.current.phase).toBe("waiting"));
    expect(result.current.begin?.authorization_url).toBe(beginResponse.authorization_url);
    expect(result.current.isAwaiting).toBe(true);
    expect(beginSettingsMCPAuth).toHaveBeenCalledWith("linear", filter, { mode: "automatic" });
  });

  it("marks the flow failed when begin errors, preserving the prior status", async () => {
    vi.mocked(beginSettingsMCPAuth).mockRejectedValue(new Error("provider unreachable"));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });

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
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    let firstAttempt: Promise<void> | undefined;
    act(() => {
      firstAttempt = result.current.beginAuthorize("linear", prior);
    });
    await waitFor(() => expect(result.current.phase).toBe("beginning"));
    act(() => result.current.cancel());
    let secondAttempt: Promise<void> | undefined;
    act(() => {
      secondAttempt = result.current.beginAuthorize("linear", prior);
    });
    await waitFor(() => expect(result.current.phase).toBe("beginning"));

    await act(async () => {
      resolveFirst?.(beginResponse);
      await firstAttempt;
    });
    resolveSecond?.({
      ...beginResponse,
      authorization_url: "https://auth.linear.app/oauth/authorize?state=retry",
      state: "agh_mcp_retry",
    });
    await act(async () => {
      await secondAttempt;
    });

    expect(result.current.phase).toBe("waiting");
    expect(result.current.begin?.state).toBe("agh_mcp_retry");
  });

  it("completes manual exchange with a bare code only on a confirmed credential", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue(authenticatedStatus());
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });
    await act(async () => {
      await result.current.enterManual();
    });
    await act(async () => {
      await result.current.submitManual("code-123");
    });

    await waitFor(() => expect(result.current.phase).toBe("confirmed"));
    expect(exchangeSettingsMCPAuth).toHaveBeenCalledWith("linear", filter, { code: "code-123" });
    expect(beginSettingsMCPAuth).toHaveBeenLastCalledWith("linear", filter, { mode: "manual" });
  });

  it("accepts a full redirect URL for the exchange", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue(authenticatedStatus());
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });
    await act(async () => {
      await result.current.enterManual();
    });
    const redirect = "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc&state=x";
    await act(async () => {
      await result.current.submitManual(redirect);
    });

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
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });
    await act(async () => {
      await result.current.enterManual();
    });
    await act(async () => {
      await result.current.submitManual("code-123");
    });

    await waitFor(() => expect(result.current.phase).toBe("failed"));
    expect(result.current.error).toBe("The provider did not return a confirmed credential.");
  });

  it("auto-confirms from a polled status only on authenticated && token_present", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });
    await waitFor(() => expect(result.current.phase).toBe("waiting"));

    // A tools/list-style success without a token must not flip the UI.
    act(() => result.current.acknowledgeStatus("needs_login", false));
    expect(result.current.phase).toBe("waiting");

    act(() => result.current.acknowledgeStatus("authenticated", true));
    expect(result.current.phase).toBe("confirmed");
  });

  it("cancels back to idle", async () => {
    vi.mocked(beginSettingsMCPAuth).mockResolvedValue(beginResponse);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMCPAuthorize(filter), { wrapper });

    await act(async () => {
      await result.current.beginAuthorize("linear", prior);
    });
    act(() => result.current.cancel());

    expect(result.current.phase).toBe("idle");
    expect(result.current.isOpen).toBe(false);
  });
});
