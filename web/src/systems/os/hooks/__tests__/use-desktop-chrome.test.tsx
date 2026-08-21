// Suite: desktop chrome boot
// Invariant: useDesktopChrome owns OsShellHandle and mounts without OsShellContext.
// Boundary IN: chrome hook projection reads and provider ownership.
// Boundary OUT: window-manager stream, client registration, and rendered DesktopShell.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({
    options: { stringifySearch: () => "" },
    history: { replace: vi.fn(), push: vi.fn() },
  }),
}));

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: () => ({ data: undefined }),
  };
});

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  return {
    ...actual,
    useWorkspaceScopeMode: () => "workspace",
  };
});

vi.mock("@/systems/session", () => ({
  useSession: () => ({ data: undefined }),
}));

vi.mock("../use-window-manager-client", () => ({
  useWindowManagerClient: () => ({
    clientId: "client:test",
    registrationEpoch: 0,
    client: null,
    status: "idle",
    error: null,
    reregister: vi.fn(),
  }),
}));

vi.mock("../use-window-manager-stream", () => ({
  useWindowManagerStream: vi.fn(),
}));

vi.mock("../use-global-shortcut-reconciliation", () => ({
  useGlobalShortcutReconciliation: () => ({ registrations: [] }),
}));

import { useDesktopChrome } from "../use-desktop-chrome";

function wrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useDesktopChrome", () => {
  it("Should mount without an OsShellContext provider", () => {
    const { result } = renderHook(() => useDesktopChrome(null), { wrapper: wrapper() });

    expect(result.current.shell.manager).toBeDefined();
    expect(result.current.shell.projection).toBeDefined();
    expect(result.current.client).toBeNull();
  });
});
