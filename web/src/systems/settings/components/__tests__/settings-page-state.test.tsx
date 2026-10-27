// Suite: SettingsPageState
// Invariant: every settings section loads as one polite status region over its own skeleton, never
// a spinner, and a failed load keeps its alert, its cause behind the details fold, and its retry.
// Owner: settings page loading and failure states.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SETTINGS_SECTIONS } from "../../lib/sections";
import { SettingsPageState } from "../settings-page-state";

describe("SettingsPageState", () => {
  it.each(SETTINGS_SECTIONS.map(section => [section.slug, section.label] as const))(
    "Should load %s as one status region over its skeleton",
    (slug, label) => {
      const { container } = render(<SettingsPageState slug={slug} state="loading" />);

      const region = screen.getByTestId(`settings-page-${slug}-loading`);
      expect(region).toHaveAttribute("role", "status");
      expect(region).toHaveAttribute("aria-busy", "true");
      expect(region).toHaveTextContent(`Loading ${label}`);
      // One region announces the whole area; the skeleton inside is decoration.
      expect(container.querySelectorAll('[role="status"]')).toHaveLength(1);
      expect(region.querySelector('[data-slot="skeleton"]')).not.toBeNull();
      expect(region.querySelector('[data-slot="block-loading"]')).toBeNull();
      expect(region.querySelector('[aria-hidden="true"] [data-slot="skeleton"]')).not.toBeNull();
    }
  );

  it("Should keep the failure alert, its cause fold and the retry", () => {
    const onRetry = vi.fn();
    render(
      <SettingsPageState
        error={new Error("daemon unreachable")}
        onRetry={onRetry}
        slug="general"
        state="error"
      />
    );

    const failure = screen.getByTestId("settings-page-general-error");
    expect(failure).toHaveTextContent("Couldn't load General");
    expect(failure).toHaveTextContent("daemon unreachable");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
