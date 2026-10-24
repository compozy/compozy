// Suite: Settings › Provider usage route page (UT-066)
// Invariant: a provider switch saves `provider_usage.providers.<id>.enabled`
// with the whole config through PATCH /api/settings/provider-usage; a rejected
// apply shows the daemon's message and keeps the draft; the Claude Keychain row
// exists only for a macOS daemon host; the feature switch hides the rest.
// Boundary IN: the route page, its page hook, and the settings/usage adapters
// over a stubbed `fetch`. Boundary OUT: the router (only route matching).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import type { ProviderUsageEnvelope } from "@/systems/provider-usage";
import { codexOkTarget, usageEnvelope } from "@/systems/provider-usage/mocks";
import { resetProfileViews } from "@/systems/profiles";
import type { SettingsProviderUsageSection } from "@/systems/settings";

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useMatchRoute: () => () => false,
}));

import { ProviderUsageSettingsPage } from "../-provider-usage-settings-page";

const SECTION: SettingsProviderUsageSection = {
  section: "provider-usage",
  scope: "user",
  available_scopes: ["user"],
  keychain_supported: false,
  config: {
    enabled: true,
    refresh_interval: "5m0s",
    providers: { cursor: { enabled: false } },
  },
};

const PROVIDERS = {
  providers: [
    { name: "codex", settings: {} },
    { name: "cursor", settings: { display_name: "Cursor" } },
    { name: "opencode", settings: { display_name: "OpenCode" } },
  ],
};

const NOW = Date.now();

interface Daemon {
  /** The shell's remembered profile. */
  activeProfile?: string;
  /** The configured-providers answer; held back to keep that read pending. */
  providers?: Promise<Response>;
  section: SettingsProviderUsageSection;
  usage: ProviderUsageEnvelope;
  patch: (body: unknown) => Response;
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const APPLIED = {
  section: "provider-usage",
  scope: "user",
  write_target: "user",
  restart_required: false,
  warnings: [],
};

function serve(daemon: Daemon) {
  vi.mocked(globalThis.fetch).mockImplementation(async input => {
    const request = input as Request;
    const { pathname } = new URL(request.url);
    if (pathname === "/api/settings/provider-usage") {
      return request.method === "PATCH"
        ? daemon.patch(await request.clone().json())
        : json(daemon.section);
    }
    if (pathname === "/api/settings/providers") return daemon.providers ?? json(PROVIDERS);
    if (pathname === "/api/profiles/selection") {
      return json([{ scope: "global", profile: daemon.activeProfile ?? "default" }]);
    }
    if (pathname === "/api/provider-usage") return json(daemon.usage);
    // The apply-record reader behind the restart notice: nothing pending.
    if (pathname === "/api/settings/apply") return json({ entries: [] });
    return json({ error: "not found", code: "not_found" }, 404);
  });
}

function patchBodies(): Promise<unknown[]> {
  return Promise.all(
    vi
      .mocked(globalThis.fetch)
      .mock.calls.map(([input]) => input as Request)
      .filter(request => request.method === "PATCH")
      .map(request => request.clone().json())
  );
}

function renderPage(daemon: Partial<Daemon> = {}) {
  serve({
    activeProfile: daemon.activeProfile,
    providers: daemon.providers,
    section: daemon.section ?? SECTION,
    usage: daemon.usage ?? usageEnvelope([codexOkTarget(NOW)], {}, NOW),
    patch: daemon.patch ?? (() => json(APPLIED)),
  });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <ProviderUsageSettingsPage />
      </UIProvider>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  resetProfileViews();
});

describe("ProviderUsageSettingsPage", () => {
  it("Should save a provider turned off with the whole config (UT-066)", async () => {
    const user = userEvent.setup();
    renderPage();

    const codex = await screen.findByTestId("settings-provider-usage-provider-codex");
    expect(codex).toBeChecked();
    // Named as the usage rows name it, not by the bare provider ID.
    expect(await screen.findByRole("switch", { name: "Codex" })).toBe(codex);
    expect(screen.getByTestId("settings-provider-usage-provider-cursor")).not.toBeChecked();
    expect(screen.queryByTestId("settings-provider-usage-provider-opencode")).toBeNull();
    expect(screen.getByTestId("settings-provider-usage-interval")).toHaveValue("5m");

    await user.click(codex);
    await user.selectOptions(screen.getByTestId("settings-provider-usage-interval"), "10m");
    await user.click(screen.getByTestId("settings-page-provider-usage-save"));

    await waitFor(async () =>
      expect(await patchBodies()).toEqual([
        {
          config: {
            enabled: true,
            refresh_interval: "10m",
            providers: { cursor: { enabled: false }, codex: { enabled: false } },
          },
        },
      ])
    );
    await waitFor(() =>
      expect(screen.getByTestId("settings-page-provider-usage-save-message")).toHaveTextContent(
        /applied/
      )
    );
  });

  // Invariant: the page is one loading area. Its section and the provider list its rows come from
  // arrive together, so the Providers group is never inserted into a page that already rendered.
  it("Should hold the loading shape until the provider list has answered too", async () => {
    let release!: (response: Response) => void;
    const providers = new Promise<Response>(resolve => {
      release = resolve;
    });
    renderPage({ providers });

    const loading = await screen.findByTestId("settings-page-provider-usage-loading");
    expect(loading).toHaveAttribute("aria-busy", "true");
    // The section itself answers at once; the page still waits for the provider list.
    await waitFor(() =>
      expect(
        vi
          .mocked(globalThis.fetch)
          .mock.calls.some(([input]) =>
            (input as Request).url.endsWith("/api/settings/provider-usage")
          )
      ).toBe(true)
    );
    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 0));
    });
    expect(screen.getByTestId("settings-page-provider-usage-loading")).toBe(loading);
    expect(screen.queryByTestId("settings-page-provider-usage")).toBeNull();

    release(json(PROVIDERS));

    expect(await screen.findByTestId("settings-provider-usage-provider-codex")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-provider-usage-loading")).toBeNull();
    expect(screen.getByTestId("settings-provider-usage-provider-cursor")).toBeInTheDocument();
  });

  it("Should keep the draft and show the daemon's message when the apply is rejected", async () => {
    const user = userEvent.setup();
    renderPage({
      patch: () =>
        json(
          {
            error: "provider_usage.providers.codex.enabled: unknown canonical provider ID",
            code: "validation_error",
          },
          400
        ),
    });

    const codex = await screen.findByTestId("settings-provider-usage-provider-codex");
    await user.click(codex);
    await user.click(screen.getByTestId("settings-page-provider-usage-save"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-page-provider-usage-save-message")).toHaveTextContent(
        /unknown canonical provider ID/
      )
    );
    expect(codex).not.toBeChecked();
  });

  it("Should show Claude Keychain access only when the daemon supports it", async () => {
    const view = renderPage();
    await screen.findByTestId("settings-provider-usage-provider-codex");
    expect(screen.queryByTestId("settings-provider-usage-keychain")).toBeNull();
    view.unmount();

    const mac = renderPage({ section: { ...SECTION, keychain_supported: true } });
    expect(await screen.findByTestId("settings-provider-usage-keychain")).not.toBeChecked();
    mac.unmount();

    // Consent stays withdrawable wherever it was saved.
    renderPage({
      section: {
        ...SECTION,
        config: { ...SECTION.config, providers: { claude: { keychain_access: true } } },
      },
    });
    expect(await screen.findByTestId("settings-provider-usage-keychain")).toBeChecked();
  });

  it("Should hide everything but the master switch while the feature is off", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByTestId("settings-provider-usage-enabled"));

    expect(screen.queryByTestId("settings-provider-usage-interval")).toBeNull();
    expect(screen.queryByTestId("settings-provider-usage-providers")).toBeNull();
  });

  it("Should carry the active profile into the consent apply", async () => {
    const user = userEvent.setup();
    renderPage({ activeProfile: "work", section: { ...SECTION, keychain_supported: true } });

    await user.click(await screen.findByTestId("settings-provider-usage-keychain"));
    await user.click(screen.getByTestId("settings-page-provider-usage-save"));

    await waitFor(() => {
      const patch = vi
        .mocked(globalThis.fetch)
        .mock.calls.map(([input]) => input as Request)
        .find(request => request.method === "PATCH");
      expect(new URL(patch?.url ?? "http://x").searchParams.get("collection_profile")).toBe("work");
    });
  });
});
