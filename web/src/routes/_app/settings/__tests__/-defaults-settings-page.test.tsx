// Invariant: Defaults never presents missing provider catalogs as authoritative emptiness.
// Owning layer: Defaults route. Canonical suite: this route-level regression.
// Boundary IN: independent settings queries. Boundary OUT: loading/error/retry page state.
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  page: {
    draft: { agent: "general", provider: null },
    envelope: {},
    error: null as Error | null,
    handleReset: vi.fn(),
    handleRetry: vi.fn(),
    handleSave: vi.fn(),
    isDirty: false,
    isLoading: false,
    isSaving: false,
    profileName: "default",
    restart: {},
    saveError: null,
    setDraft: vi.fn(),
    warnings: undefined,
  },
  providers: {
    data: { providers: [] as Array<{ name: string }> },
    error: null as Error | null,
    isLoading: false,
    refetch: vi.fn(),
  },
}));

vi.mock("@/systems/settings", async importOriginal => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    useSettingsPersonaPage: () => mocks.page,
    useSettingsProviders: () => mocks.providers,
    useSettingsSaveBarState: () => ({ kind: "clean" }),
    useSettingsTopbar: vi.fn(),
  };
});

import { DefaultsSettingsPage } from "../-defaults-settings-page";

describe("DefaultsSettingsPage", () => {
  beforeEach(() => {
    mocks.page.error = null;
    mocks.page.isLoading = false;
    mocks.page.handleRetry.mockReset();
    mocks.page.setDraft.mockReset();
    mocks.providers.error = null;
    mocks.providers.isLoading = false;
    mocks.providers.data.providers = [];
    mocks.providers.refetch.mockReset();
  });

  it("Should wait for every runtime option catalog before rendering defaults", () => {
    mocks.providers.isLoading = true;

    render(<DefaultsSettingsPage />);

    expect(screen.getByRole("status", { name: "Loading profile defaults" })).toBeVisible();
    expect(screen.queryByTestId("settings-page-defaults-session")).not.toBeInTheDocument();
  });

  it("Should surface a catalog failure and retry every Defaults dependency", () => {
    mocks.providers.error = new Error("Provider catalog unavailable");

    render(<DefaultsSettingsPage />);
    expect(screen.getByText("Provider catalog unavailable")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(mocks.page.handleRetry).toHaveBeenCalledOnce();
    expect(mocks.providers.refetch).toHaveBeenCalledOnce();
  });

  it("Should render and wire every profile default control", () => {
    mocks.providers.data.providers = [{ name: "claude" }];

    render(<DefaultsSettingsPage />);

    expect(screen.getByTestId("settings-page-defaults-agent")).toHaveValue("general");
    expect(screen.getByRole("option", { name: "claude" })).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("settings-page-defaults-agent"), {
      target: { value: "reviewer" },
    });
    fireEvent.change(screen.getByTestId("settings-page-defaults-provider"), {
      target: { value: "claude" },
    });

    const initial = { agent: "general", provider: null };
    const updatedDrafts = mocks.page.setDraft.mock.calls.map(([update]) => {
      expect.assert(typeof update === "function");
      return update(initial);
    });
    expect(updatedDrafts).toContainEqual({ ...initial, agent: "reviewer" });
    expect(updatedDrafts).toContainEqual({ ...initial, provider: "claude" });
  });
});
