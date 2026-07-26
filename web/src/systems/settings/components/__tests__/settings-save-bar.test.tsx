import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SettingsSaveBar } from "../settings-save-bar";
import type { SettingsSaveBarState } from "../../lib/save-state";

function renderSaveBar(
  state: SettingsSaveBarState,
  overrides: Partial<React.ComponentProps<typeof SettingsSaveBar>> = {}
) {
  const props = {
    slug: "general",
    onSave: vi.fn(),
    onReset: vi.fn(),
    ...overrides,
    state,
  };
  render(<SettingsSaveBar {...props} />);
  return props;
}

describe("SettingsSaveBar", () => {
  it("renders nothing while the page is clean", () => {
    renderSaveBar({ kind: "idle" });

    expect(screen.queryByTestId("settings-page-general-save-bar")).not.toBeInTheDocument();
  });

  it("appears when dirty and fires onSave from the enabled Save button", () => {
    const { onSave } = renderSaveBar({ kind: "dirty", warnings: [] });

    expect(screen.getByTestId("settings-page-general-save-bar")).toHaveAttribute(
      "data-dirty",
      "true"
    );
    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent(
      "Unsaved changes"
    );
    const save = screen.getByTestId("settings-page-general-save");
    expect(save).not.toBeDisabled();
    fireEvent.click(save);
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it("disables Save and names the blocker when invalid even if dirty", () => {
    renderSaveBar({ kind: "invalid", warnings: [] });

    expect(screen.getByTestId("settings-page-general-save")).toBeDisabled();
    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent(
      "Resolve validation errors before saving"
    );
  });

  it("names the saving state and disables both actions while saving", () => {
    renderSaveBar({ kind: "saving" });

    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent("Saving…");
    expect(screen.getByTestId("settings-page-general-save")).toBeDisabled();
    expect(screen.getByTestId("settings-page-general-reset")).toBeDisabled();
  });

  it("renders the error message through an assertive live region", () => {
    renderSaveBar({ kind: "error", message: "config write failed" });

    const bar = screen.getByTestId("settings-page-general-save-bar");
    expect(bar).toHaveAttribute("aria-live", "assertive");
    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent(
      "config write failed"
    );
  });

  it("renders an explicit saved state without implying unsaved changes", () => {
    renderSaveBar({ kind: "saved", message: "Applied 2 fields" });

    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent(
      "Applied 2 fields"
    );
    expect(screen.getByTestId("settings-page-general-save-bar")).toHaveAttribute(
      "data-state",
      "saved"
    );
  });

  it("keeps warnings visible alongside the dirty state", () => {
    renderSaveBar({ kind: "dirty", warnings: ["value clamped to 3600"] });

    expect(screen.getByTestId("settings-page-general-save-warnings")).toHaveTextContent(
      "value clamped to 3600"
    );
  });

  it("names clean post-save warnings and leaves both actions disabled", () => {
    renderSaveBar({
      kind: "warning",
      message: "Saved with warnings",
      warnings: ["value clamped to 3600"],
    });

    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent(
      "Saved with warnings"
    );
    expect(screen.getByTestId("settings-page-general-save")).toBeDisabled();
    expect(screen.getByTestId("settings-page-general-reset")).toBeDisabled();
  });
});
