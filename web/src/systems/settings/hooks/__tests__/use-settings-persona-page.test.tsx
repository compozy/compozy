// Suite: profile persona settings page model
// Invariant: the active destination selects the user or profile layer, and an in-flight
// write remains owned by the layer where it started after the operator switches profiles.
// Owning layer: settings persona page model. No prior suite owned this profile boundary.
// Boundary IN: profile destination, query filter, draft, and mutation variables.
// Boundary OUT: HTTP transport and config persistence, covered by adapter and daemon suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { getSettingsPersona, profile, updateSettingsPersona } = vi.hoisted(() => ({
  getSettingsPersona: vi.fn(),
  profile: { destination: "default" },
  updateSettingsPersona: vi.fn(),
}));

vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => ({ destination: profile.destination }),
}));
vi.mock("../../adapters/settings-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/settings-api")>();
  return { ...actual, getSettingsPersona, updateSettingsPersona };
});
vi.mock("../use-settings-page", () => ({
  useSettingsPage: () => ({ restart: { isVisible: false } }),
}));

import type { SettingsPersonaFilter, SettingsPersonaSection } from "../../types";
import { useSettingsPersonaPage } from "../use-settings-persona-page";

function section(filter: SettingsPersonaFilter): SettingsPersonaSection {
  const namedProfile = filter.scope === "profile" ? filter.profile : undefined;
  return {
    section: "persona",
    scope: filter.scope ?? "user",
    profile: namedProfile,
    available_scopes: ["user", "profile", "workspace"],
    config: {
      agent: namedProfile === "marketing" ? "campaigns" : "general",
      provider: namedProfile === "marketing" ? "openai" : "claude",
      sandbox: "local",
    },
  };
}

function mutationResult(scope: "user" | "profile", profileName?: string) {
  return {
    section: "persona" as const,
    scope,
    profile: profileName,
    applied: true,
    active_config_hash: "sha256:persona",
    active_generation: 4,
    apply_record_id: "cfg_apply_persona",
    lifecycle: "session-rebind" as const,
    next_action: "new-session" as const,
    restart_required: false,
    write_target: scope === "profile" ? ("profile-config" as const) : ("global-config" as const),
  };
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, ...renderHook(() => useSettingsPersonaPage(), { wrapper }) };
}

describe("useSettingsPersonaPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    profile.destination = "default";
    getSettingsPersona.mockImplementation((filter: SettingsPersonaFilter) =>
      Promise.resolve(section(filter))
    );
    updateSettingsPersona.mockResolvedValue(mutationResult("user"));
  });

  it("Should use the user layer for the permanent default profile", async () => {
    const { result } = renderPage();

    await waitFor(() => expect(result.current.draft?.agent).toBe("general"));
    expect(getSettingsPersona).toHaveBeenCalledWith({ scope: "user" }, expect.anything());

    act(() => result.current.setDraft(current => ({ ...current!, provider: "openai" })));
    act(() => result.current.handleSave());

    await waitFor(() => expect(updateSettingsPersona).toHaveBeenCalledTimes(1));
    expect(updateSettingsPersona).toHaveBeenCalledWith(
      { config: { agent: "general", provider: "openai", sandbox: "local" } },
      { scope: "user" }
    );
  });

  it("Should keep a late profile write attached to its originating profile", async () => {
    let resolveWrite: ((value: ReturnType<typeof mutationResult>) => void) | undefined;
    updateSettingsPersona.mockImplementation(
      () =>
        new Promise(resolve => {
          resolveWrite = resolve;
        })
    );
    profile.destination = "marketing";
    const { rerender, result } = renderPage();

    await waitFor(() => expect(result.current.draft?.agent).toBe("campaigns"));
    act(() => result.current.setDraft(current => ({ ...current!, sandbox: "browser" })));
    act(() => result.current.handleSave());
    await waitFor(() => expect(result.current.isSaving).toBe(true));

    profile.destination = "default";
    rerender();

    await waitFor(() => expect(result.current.draft?.agent).toBe("general"));
    expect(result.current.isSaving).toBe(false);
    expect(getSettingsPersona).toHaveBeenCalledWith({ scope: "user" }, expect.anything());

    await act(async () => {
      resolveWrite?.(mutationResult("profile", "marketing"));
      await Promise.resolve();
    });

    expect(updateSettingsPersona).toHaveBeenCalledWith(
      { config: { agent: "campaigns", provider: "openai", sandbox: "browser" } },
      { scope: "profile", profile: "marketing" }
    );
    expect(result.current.profileName).toBe("default");
    expect(result.current.draft?.agent).toBe("general");
  });
});
