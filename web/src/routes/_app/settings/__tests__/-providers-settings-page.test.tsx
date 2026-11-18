// Suite: Settings › Providers route page — enabled switch, disabled group, Status filter
// (UT-021, UT-022, UT-023, UT-024, UT-026, UT-032).
// Invariant: disabled providers leave the grid for a collapsed "Disabled (N)" group and the header
// counts them; the "Disabled" status filter shows only them; the card switch PATCHes
// `/api/settings/providers/{name}/enabled`, reverts with an inline error when the save fails, and is
// locked on the default provider.
// Boundary IN: the route page, its page hook, the settings adapters over a stubbed `fetch`.
// Boundary OUT: the router (only route matching).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toast, TooltipProvider, UIProvider } from "@compozy/ui";
import type { SettingsProviderEntry } from "@/systems/settings";

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useMatchRoute: () => () => false,
}));

import { ProvidersSettingsPage } from "../-providers-settings-page";

function provider(
  name: string,
  {
    enabled = true,
    isDefault = false,
    displayName,
  }: Partial<{
    enabled: boolean;
    isDefault: boolean;
    displayName: string;
  }> = {}
): SettingsProviderEntry {
  return {
    name,
    default: isDefault,
    command_available: true,
    auth_status: {
      mode: "none",
      env_policy: "filtered",
      home_policy: "operator",
      state: "none",
      login: { configured: false, presence: "unknown" },
    },
    settings: { enabled, ...(displayName ? { display_name: displayName } : {}) },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "global-config", scope: "user" },
      shadowed_sources: [],
    },
  };
}

const APPLY = {
  section: "providers",
  scope: "user",
  write_target: "user",
  restart_required: false,
  warnings: [],
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** A daemon whose provider list follows accepted PATCHes; `patchStatus` forces a failure. */
function serve(providers: SettingsProviderEntry[], patchStatus = 200) {
  const state = { providers: [...providers] };
  vi.mocked(globalThis.fetch).mockImplementation(async input => {
    const request = input as Request;
    const { pathname } = new URL(request.url);
    const toggle = /^\/api\/settings\/providers\/([^/]+)\/enabled$/.exec(pathname);
    if (toggle && request.method === "PATCH") {
      if (patchStatus !== 200) return json({ error: "config write failed" }, patchStatus);
      const { enabled } = (await request.clone().json()) as { enabled: boolean };
      const name = decodeURIComponent(toggle[1] ?? "");
      state.providers = state.providers.map(entry =>
        entry.name === name ? { ...entry, settings: { ...entry.settings, enabled } } : entry
      );
      const updated = state.providers.find(entry => entry.name === name);
      return json({ provider: updated, apply: APPLY });
    }
    if (pathname === "/api/settings/providers") return json({ providers: state.providers });
    if (pathname === "/api/settings/apply") return json({ entries: [] });
    return json({ error: "not found", code: "not_found" }, 404);
  });
}

function patchRequests(): Promise<Array<{ path: string; body: unknown }>> {
  return Promise.all(
    vi
      .mocked(globalThis.fetch)
      .mock.calls.map(([input]) => input as Request)
      .filter(request => request.method === "PATCH")
      .map(async request => ({
        path: new URL(request.url).pathname,
        body: await request.clone().json(),
      }))
  );
}

function renderPage(providers: SettingsProviderEntry[], patchStatus?: number) {
  serve(providers, patchStatus);
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <TooltipProvider>
          <ProvidersSettingsPage />
        </TooltipProvider>
      </UIProvider>
    </QueryClientProvider>
  );
}

const grid = () => screen.getByTestId("settings-page-providers-list");

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("ProvidersSettingsPage", () => {
  it("Should group disabled providers after the grid and count them in the header (UT-021)", async () => {
    const user = userEvent.setup();
    renderPage([
      provider("claude", { isDefault: true }),
      provider("codex"),
      provider("gemini", { enabled: false }),
      provider("cursor", { enabled: false }),
    ]);

    await screen.findByTestId("settings-page-providers-list");
    expect(screen.getByTestId("settings-page-providers-disabled")).toHaveTextContent("2 disabled");
    expect(within(grid()).getByTestId("settings-page-providers-card-claude")).toBeInTheDocument();
    expect(within(grid()).queryByTestId("settings-page-providers-card-gemini")).toBeNull();

    const toggle = screen.getByTestId("settings-page-providers-disabled-toggle");
    expect(toggle).toHaveTextContent("Disabled (2)");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);
    const group = screen.getByTestId("settings-page-providers-disabled-list");
    expect(
      within(group).getByTestId("settings-page-providers-card-gemini-status")
    ).toHaveTextContent("Disabled");
    expect(within(group).getByTestId("settings-page-providers-card-cursor")).toBeInTheDocument();
  });

  it("Should show no group and no suffix when nothing is disabled (UT-022)", async () => {
    renderPage([provider("claude", { isDefault: true }), provider("codex")]);

    await screen.findByTestId("settings-page-providers-list");
    expect(screen.queryByTestId("settings-page-providers-disabled")).toBeNull();
    expect(screen.queryByTestId("settings-page-providers-disabled-group")).toBeNull();
  });

  it("Should keep only the default in the grid when every other provider is off (UT-022)", async () => {
    renderPage([
      provider("claude", { isDefault: true }),
      provider("codex", { enabled: false }),
      provider("gemini", { enabled: false }),
      provider("cursor", { enabled: false }),
    ]);

    await screen.findByTestId("settings-page-providers-list");
    expect(grid().querySelectorAll('[data-slot="catalog-card"]')).toHaveLength(1);
    expect(within(grid()).getByTestId("settings-page-providers-card-claude")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-providers-disabled-toggle")).toHaveTextContent(
      "Disabled (3)"
    );
  });

  it("Should show only disabled providers, without the group, for the Disabled filter (UT-023)", async () => {
    const user = userEvent.setup();
    renderPage([
      provider("claude", { isDefault: true }),
      provider("codex"),
      provider("gemini", { enabled: false }),
    ]);

    await user.click(await screen.findByTestId("settings-providers-status-filter"));
    await user.click(await screen.findByTestId("settings-providers-status-disabled"));

    await waitFor(() =>
      expect(within(grid()).getByTestId("settings-page-providers-card-gemini")).toBeInTheDocument()
    );
    expect(within(grid()).queryByTestId("settings-page-providers-card-claude")).toBeNull();
    expect(within(grid()).queryByTestId("settings-page-providers-card-codex")).toBeNull();
    expect(screen.queryByTestId("settings-page-providers-disabled-group")).toBeNull();
  });

  it("Should PATCH the switch off, move the card to the group and confirm with a toast (UT-024)", async () => {
    const user = userEvent.setup();
    const success = vi.spyOn(toast, "success");
    renderPage([
      provider("claude", { isDefault: true }),
      provider("gemini", { displayName: "Gemini" }),
    ]);

    const toggle = await screen.findByTestId("settings-providers-enabled-gemini");
    expect(toggle).toBeChecked();
    await user.click(toggle);

    await waitFor(async () =>
      expect(await patchRequests()).toEqual([
        { path: "/api/settings/providers/gemini/enabled", body: { enabled: false } },
      ])
    );
    await waitFor(() =>
      expect(screen.getByTestId("settings-page-providers-disabled-toggle")).toHaveTextContent(
        "Disabled (1)"
      )
    );
    expect(within(grid()).queryByTestId("settings-page-providers-card-gemini")).toBeNull();
    expect(success).toHaveBeenCalledWith(
      "Gemini disabled. Running sessions keep going; new ones can't use it."
    );
  });

  it("Should revert the switch and show the error inline when the save fails (UT-026)", async () => {
    const user = userEvent.setup();
    renderPage([provider("claude", { isDefault: true }), provider("gemini")], 500);

    const toggle = await screen.findByTestId("settings-providers-enabled-gemini");
    await user.click(toggle);

    expect(await screen.findByTestId("settings-providers-enabled-gemini-error")).toHaveTextContent(
      "config write failed"
    );
    expect(screen.getByTestId("settings-providers-enabled-gemini")).toBeChecked();
    expect(within(grid()).getByTestId("settings-page-providers-card-gemini")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-providers-disabled-group")).toBeNull();
  });

  it("Should lock the default provider's switch with the reason (UT-032)", async () => {
    const user = userEvent.setup();
    renderPage([provider("claude", { isDefault: true }), provider("codex")]);

    const toggle = await screen.findByTestId("settings-providers-enabled-claude");
    expect(toggle).toHaveAttribute("data-disabled");
    expect(toggle).toBeChecked();

    const reason = "Choose another default provider before disabling this one";
    expect(toggle).toHaveAccessibleDescription(reason);
    expect(screen.getAllByText(reason)).toHaveLength(1);
    await user.hover(screen.getByTestId("settings-providers-enabled-claude-locked"));
    await waitFor(() => expect(screen.getAllByText(reason)).toHaveLength(2));
    expect(await patchRequests()).toEqual([]);
  });
});
