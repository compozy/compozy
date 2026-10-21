// Suite: Usage app window
// Invariant: the Usage app shows every target expanded with a header Refresh
// and a link to the Provider usage settings; with the feature off it says so
// and keeps the settings link (US-010.AC-2, EC-2).
// Boundary IN: UsageWindow + the shared provider usage surface over a stubbed
// `fetch`; window commands come from a mocked shell.
// Boundary OUT: row rendering details (provider-usage panel suite), popover
// wiring (desktop-menubar suite).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import type { ProviderUsageEnvelope } from "@/systems/provider-usage";
import { codexOkTarget, usageEnvelope } from "@/systems/provider-usage/mocks";

import { UsageWindow } from "../usage-window";

const shell = vi.hoisted(() => ({
  coordinator: { userOpen: vi.fn(), userNavigate: vi.fn() },
}));

vi.mock("../../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: object) => unknown) =>
    selector({ hydration: "live", snapshot: {}, client: {}, windowManagerConfig: {} }),
}));

vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: shell.coordinator }),
}));

const NOW = Date.now();

function serve(envelope: ProviderUsageEnvelope) {
  vi.mocked(globalThis.fetch).mockImplementation(input => {
    const url = input instanceof Request ? input.url : String(input);
    const body = url.includes("/api/provider-usage")
      ? envelope
      : { error: "not found", code: "not_found" };
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status: url.includes("/api/provider-usage") ? 200 : 404,
        headers: { "Content-Type": "application/json" },
      })
    );
  });
}

function renderUsageWindow(envelope: ProviderUsageEnvelope) {
  serve(envelope);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(["provider-usage", "envelope", "default"], envelope);
  return render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <UsageWindow />
      </UIProvider>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
  shell.coordinator.userOpen.mockReset();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("UsageWindow", () => {
  it("Should show every target expanded under a Usage header with Refresh (US-010.AC-2)", () => {
    renderUsageWindow(usageEnvelope([codexOkTarget(NOW)], {}, NOW));

    expect(screen.getByRole("heading", { name: "Usage" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Refresh provider usage" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Codex/ })).toHaveAttribute("aria-expanded", "true");
    expect(screen.queryByRole("button", { name: "Open Usage" })).toBeNull();
  });

  it("Should say provider usage is off and link to its settings (UT-065)", async () => {
    const user = userEvent.setup();
    renderUsageWindow(usageEnvelope([], { enabled: false }, NOW));

    expect(screen.getByText("Provider usage is turned off")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Refresh provider usage" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Open settings" }));

    expect(shell.coordinator.userOpen).toHaveBeenCalledWith({
      app: "settings",
      route: { pathname: "/settings/provider-usage", search: {} },
    });
  });
});
