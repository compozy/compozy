// Suite: Settings Palette route composition.
// Invariant: live controls call their owning mutations, while reset requires an explicit
// scope-naming confirmation before personalization data is deleted.
// Owning layer: Settings Palette route page. Boundary OUT: query and HTTP transport.
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  handleRetry: vi.fn(),
  isLoading: false,
  error: null as Error | null,
  saveError: null as string | null,
  resetError: null as string | null,
  resetPersonalization: vi.fn<() => Promise<void>>(),
  setFallbackAgentEnabled: vi.fn(),
  setPersonalization: vi.fn(),
}));

vi.mock("@/systems/settings", () => ({
  SettingsGroup: ({ children }: PropsWithChildren) => <section>{children}</section>,
  SettingsPageFrame: ({ children }: PropsWithChildren) => <main>{children}</main>,
  SettingRow: ({
    control,
    label,
    description,
  }: {
    control: ReactNode;
    label: string;
    description?: string;
  }) => (
    <div>
      <span>{label}</span>
      {description ? <span>{description}</span> : null}
      {control}
    </div>
  ),
  useSettingsPalettePage: () => ({
    section: { fallback_agent_enabled: true, personalization: true },
    isLoading: state.isLoading,
    isSaving: false,
    error: state.error,
    saveError: state.saveError,
    restart: { isVisible: false },
    scopeLabel: "Workspace Alpha",
    setFallbackAgentEnabled: state.setFallbackAgentEnabled,
    setPersonalization: state.setPersonalization,
    resetPersonalization: state.resetPersonalization,
    canResetPersonalization: true,
    isResetting: false,
    resetError: state.resetError,
    handleRetry: state.handleRetry,
  }),
  useSettingsTopbar: vi.fn(),
}));

import { PaletteSettingsPage } from "../-palette-settings-page";

describe("PaletteSettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.isLoading = false;
    state.error = null;
    state.saveError = null;
    state.resetError = null;
    state.resetPersonalization.mockResolvedValue(undefined);
  });

  it("Should toggle agent fallback through the live settings model [UT-151]", async () => {
    const user = userEvent.setup();
    render(<PaletteSettingsPage />);

    await user.click(screen.getByRole("switch", { name: "Agent fallback" }));

    expect(state.setFallbackAgentEnabled).toHaveBeenCalledWith(false);
  });

  it("Should toggle personalization through the live settings model [UT-151]", async () => {
    const user = userEvent.setup();
    render(<PaletteSettingsPage />);

    await user.click(screen.getByRole("switch", { name: "Palette personalization" }));

    expect(state.setPersonalization).toHaveBeenCalledWith(false);
  });

  it("Should name the scope and wait for reset confirmation [UT-151]", async () => {
    const user = userEvent.setup();
    render(<PaletteSettingsPage />);

    await user.click(screen.getByTestId("settings-palette-reset"));

    expect(state.resetPersonalization).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toHaveTextContent("Workspace Alpha");

    await user.click(screen.getByRole("button", { name: "Reset personalization" }));

    expect(state.resetPersonalization).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("Should render the loading status while the page model is pending [UT-151]", () => {
    state.isLoading = true;
    render(<PaletteSettingsPage />);

    expect(screen.getByTestId("settings-page-palette-loading")).toBeVisible();
  });

  it("Should render the error branch and retry the page model [UT-151]", async () => {
    const user = userEvent.setup();
    state.error = new Error("palette settings failed");
    render(<PaletteSettingsPage />);

    expect(screen.getByTestId("settings-page-palette-error")).toHaveTextContent(
      "palette settings failed"
    );
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(state.handleRetry).toHaveBeenCalledTimes(1);
  });

  it("Should surface a save error in the page alert [UT-151]", () => {
    state.saveError = "Could not save palette settings.";
    render(<PaletteSettingsPage />);

    expect(screen.getByTestId("settings-palette-save-error")).toHaveTextContent(
      "Could not save palette settings."
    );
  });

  it("Should surface a reset error in the page alert [UT-151]", () => {
    state.resetError = "Could not reset personalization.";
    render(<PaletteSettingsPage />);

    expect(screen.getByTestId("settings-palette-save-error")).toHaveTextContent(
      "Could not reset personalization."
    );
  });
});
