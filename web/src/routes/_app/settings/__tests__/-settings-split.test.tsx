import { render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { settingsHooksExtensionsSectionFixture } from "@/systems/settings/mocks/fixtures";

vi.mock("@tanstack/react-router", async importOriginal => {
  const original = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...original,
    Link: ({
      to,
      children,
      ...rest
    }: {
      to: string;
      children?: React.ReactNode;
      [key: string]: unknown;
    }) => (
      <a href={String(to)} {...(rest as Record<string, unknown>)}>
        {children}
      </a>
    ),
  };
});

import { PolicySection } from "../-extensions-policy-section";
import { ExtensionPalettePanel } from "../-extension-palette-panel";
import { HooksSection } from "../-hooks-section";

function PolicyHarness() {
  const [draft, setDraft] = useState(settingsHooksExtensionsSectionFixture.config);
  return <PolicySection canMutate draft={draft} setDraft={setDraft} />;
}

describe("Settings route split", () => {
  it("Should show effective, dormant, and view palette contributions", () => {
    const extension = settingsHooksExtensionsSectionFixture.installed?.find(
      item => item.name === "notes"
    );
    if (!extension) throw new Error("notes extension fixture is required");

    render(<ExtensionPalettePanel extension={extension} />);

    expect(screen.getByText("Capture note")).toBeInTheDocument();
    expect(screen.getByText("⌥⇧N")).toBeInTheDocument();
    expect(screen.getByText("dormant")).toBeInTheDocument();
    expect(
      screen.getByText("default unavailable — conflicts with session.new")
    ).toBeInTheDocument();
    expect(screen.getByTestId("extension-palette-view-ext.notes.browse")).toHaveTextContent(
      "Browse notes"
    );
  });

  it("Should render exactly the four supported extension source and trust policy fields", () => {
    render(<PolicyHarness />);

    expect(
      screen.getByTestId("settings-page-extensions-policy-github-enabled")
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("settings-page-extensions-policy-github-base-url")
    ).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-extensions-policy-git-enabled")).toBeInTheDocument();
    expect(
      screen.getByTestId("settings-page-extensions-policy-allow-unverified")
    ).toBeInTheDocument();
    const controls = [...screen.getAllByRole("textbox"), ...screen.getAllByRole("switch")];
    expect(screen.getAllByRole("textbox")).toHaveLength(1);
    expect(screen.getAllByRole("switch")).toHaveLength(3);
    expect(controls).toHaveLength(4);
    expect(screen.queryByText(/allowed kinds|max scope|rate limit|watch interval/i)).toBeNull();
    expect(screen.queryByRole("spinbutton")).not.toBeInTheDocument();
  });

  it("Should keep the Hooks surface free of installed-extension management", () => {
    render(
      <HooksSection
        canMutate
        hookError={null}
        hooks={settingsHooksExtensionsSectionFixture.hooks ?? []}
        onToggle={vi.fn()}
        pendingHookName={null}
      />
    );

    expect(screen.getByTestId("settings-page-hooks-section")).toBeInTheDocument();
    expect(screen.queryByText(/installed extensions/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /provenance|remove extension/i })).toBeNull();
  });
});
