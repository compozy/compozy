// Suite: desktop chrome boot
// Invariant: useDesktopChrome owns OsShellHandle and mounts without OsShellContext;
// the settings read that names this client waits for its registration.
// Boundary IN: chrome hook projection reads and provider ownership.
// Boundary OUT: window-manager stream, client registration, and rendered DesktopShell.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const queryOptionsSeen = vi.hoisted(() => [] as Array<{ queryKey: unknown; enabled?: unknown }>);
const registration = vi.hoisted(() => ({ status: "idle" as string }));

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
    useQuery: (options: { queryKey: unknown; enabled?: unknown }) => {
      queryOptionsSeen.push(options);
      return { data: undefined };
    },
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
    status: registration.status,
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

function settingsReads() {
  return queryOptionsSeen.filter(
    options => Array.isArray(options.queryKey) && options.queryKey.includes("client:test")
  );
}

describe("useDesktopChrome", () => {
  beforeEach(() => {
    queryOptionsSeen.length = 0;
    registration.status = "idle";
  });

  it("Should mount without an OsShellContext provider", () => {
    const { result } = renderHook(() => useDesktopChrome(null), { wrapper: wrapper() });

    expect(result.current.shell.manager).toBeDefined();
    expect(result.current.shell.projection).toBeDefined();
    expect(result.current.client).toBeNull();
  });

  it("Should hold the client-scoped settings read until the client is registered", () => {
    const { rerender } = renderHook(() => useDesktopChrome("workspace:one"), {
      wrapper: wrapper(),
    });
    expect(settingsReads().length).toBeGreaterThan(0);
    expect(settingsReads().every(options => options.enabled === false)).toBe(true);

    registration.status = "registered";
    queryOptionsSeen.length = 0;
    rerender();
    expect(settingsReads().at(-1)?.enabled).toBe(true);
  });
});
