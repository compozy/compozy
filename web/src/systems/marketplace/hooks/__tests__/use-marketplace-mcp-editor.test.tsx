// Suite: marketplace MCP editor hook
// Invariant: accepted saves close the editor, while dismissal rejected during a pending save
// preserves that save's feedback.
// Boundary IN: editor lifecycle, mutation reset ownership, and XState save state.
// Boundary OUT: settings and Vault HTTP adapters.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PropsWithChildren } from "react";

import { useMarketplaceMCPEditor } from "../use-marketplace-mcp-editor";

const mocks = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
  reset: vi.fn(),
}));

vi.mock("@/systems/settings", async () => {
  const actual = await vi.importActual<typeof import("@/systems/settings")>("@/systems/settings");
  const { useState } = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    usePutSettingsMCPServer: () => {
      const [error, setError] = useState<Error | null>(null);
      return {
        data: undefined,
        error,
        mutateAsync: async (...args: unknown[]) => {
          setError(null);
          try {
            return await mocks.mutateAsync(...args);
          } catch (mutationError) {
            const nextError =
              mutationError instanceof Error
                ? mutationError
                : new Error("Could not save MCP server");
            setError(nextError);
            throw mutationError;
          }
        },
        reset: () => {
          mocks.reset();
          setError(null);
        },
      };
    },
  };
});

vi.mock("@/systems/vault", () => ({
  vaultSecretsListOptions: () => ({
    queryFn: async () => [],
    queryKey: ["vault", "mcp"] as const,
  }),
}));

function wrapper({ children }: PropsWithChildren) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useMarketplaceMCPEditor", () => {
  beforeEach(() => {
    mocks.mutateAsync.mockReset();
    mocks.reset.mockReset();
  });

  it("Should close the editor after an accepted save settles", async () => {
    mocks.mutateAsync.mockResolvedValue(undefined);
    const { result } = renderHook(
      () =>
        useMarketplaceMCPEditor({
          enabled: true,
          scope: "global",
          servers: [],
        }),
      { wrapper }
    );

    act(() => result.current.openCreate());
    act(() =>
      result.current.editorProps?.onChange(draft => ({
        ...draft,
        command: "npx",
        name: "github",
      }))
    );
    act(() => result.current.editorProps?.onSave());

    await waitFor(() => expect(result.current.editorProps).toBeNull());
    expect(mocks.mutateAsync).toHaveBeenCalledOnce();
    expect(mocks.mutateAsync).toHaveBeenCalledWith({
      body: { server: { command: "npx", name: "github", transport: "stdio" } },
      filter: { scope: "global", target: "auto" },
      name: "github",
    });
  });

  it("Should retain mutation feedback when a pending save rejects editor dismissal", async () => {
    let rejectSave!: (error: Error) => void;
    const save = new Promise<never>((_resolve, reject) => {
      rejectSave = reject;
    });
    mocks.mutateAsync.mockReturnValue(save);
    const { result } = renderHook(
      () =>
        useMarketplaceMCPEditor({
          enabled: true,
          scope: "global",
          servers: [],
        }),
      { wrapper }
    );

    act(() => result.current.openCreate());
    mocks.reset.mockClear();
    act(() =>
      result.current.editorProps?.onChange(draft => ({
        ...draft,
        command: "npx",
        name: "github",
      }))
    );
    act(() => result.current.editorProps?.onSave());
    await waitFor(() => expect(result.current.editorProps?.isSaving).toBe(true));

    act(() => result.current.editorProps?.onClose());

    expect(mocks.reset).not.toHaveBeenCalled();
    expect(result.current.editorProps?.saveError).toBeNull();

    const saveError = new Error("save rejected");
    act(() => rejectSave(saveError));
    await waitFor(() => expect(result.current.editorProps?.isSaving).toBe(false));
    expect(result.current.editorProps?.saveError).toBe(saveError.message);

    act(() => result.current.editorProps?.onClose());
    expect(mocks.reset).toHaveBeenCalledOnce();
  });
});
