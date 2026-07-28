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
import { HooksSection } from "../-hooks-section";

function PolicyHarness() {
  const [draft, setDraft] = useState(settingsHooksExtensionsSectionFixture.config);
  return <PolicySection canMutate draft={draft} setDraft={setDraft} />;
}

describe("Settings route split", () => {
  it("Should render exactly the three supported extension marketplace policy fields", () => {
    render(<PolicyHarness />);

    expect(screen.getByTestId("settings-page-extensions-policy-registry")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-extensions-policy-base-url")).toBeInTheDocument();
    expect(
      screen.getByTestId("settings-page-extensions-policy-allow-unverified")
    ).toBeInTheDocument();
    const controls = [...screen.getAllByRole("textbox"), ...screen.getAllByRole("switch")];
    expect(screen.getAllByRole("textbox")).toHaveLength(2);
    expect(screen.getAllByRole("switch")).toHaveLength(1);
    expect(controls).toHaveLength(3);
    expect(screen.queryByText(/allowed kinds|max scope|rate limit/i)).not.toBeInTheDocument();
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
