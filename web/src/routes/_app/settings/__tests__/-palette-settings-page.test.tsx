// Suite: Settings Palette route composition.
// Invariant: live controls call their owning mutations, while reset requires an explicit
// scope-naming confirmation before personalization data is deleted.
// Owning layer: Settings Palette route page. Boundary OUT: query and HTTP transport.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
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
    isLoading: false,
    isSaving: false,
    error: null,
    saveError: null,
    restart: { isVisible: false },
    scopeLabel: "Workspace Alpha",
    setFallbackAgentEnabled: state.setFallbackAgentEnabled,
    setPersonalization: state.setPersonalization,
    resetPersonalization: state.resetPersonalization,
    isResetting: false,
    resetError: null,
    handleRetry: vi.fn(),
  }),
  useSettingsTopbar: vi.fn(),
}));

import { PaletteSettingsPage } from "../-palette-settings-page";

describe("PaletteSettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.resetPersonalization.mockResolvedValue(undefined);
  });

  it("Should toggle agent fallback through the live settings model [UT-151]", async () => {
    const user = userEvent.setup();
    render(<PaletteSettingsPage />);

    await user.click(screen.getByRole("switch", { name: "Agent fallback" }));

    expect(state.setFallbackAgentEnabled).toHaveBeenCalledWith(false);
  });

  it("Should name the scope and wait for reset confirmation [UT-151]", async () => {
    const user = userEvent.setup();
    render(<PaletteSettingsPage />);

    await user.click(screen.getByTestId("settings-palette-reset"));

    expect(state.resetPersonalization).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toHaveTextContent("Workspace Alpha");

    await user.click(screen.getByRole("button", { name: "Reset personalization" }));

    expect(state.resetPersonalization).toHaveBeenCalledTimes(1);
  });
});
