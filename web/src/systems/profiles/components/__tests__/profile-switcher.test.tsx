// Suite: menubar profile switcher
// Invariant: the switcher is quiet until a second profile exists, answers the boundary
// question in one sentence, marks the active context, and refuses to offer a profile the
// runtime would reject.
// Boundary IN: the switcher composition and its menu.
// Boundary OUT: selection persistence (hook suites) and row projection (profile-rows suite).

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { PROFILE_BOUNDARY_ANSWER } from "../../lib/profile-copy";
import { toProfileRows } from "../../lib/profile-rows";
import {
  defaultProfileFixture,
  growthProfileFixture,
  marketingProfileFixture,
} from "../../mocks/fixtures";
import { ProfileSwitcher } from "../profile-switcher";

const PLURAL_ROWS = toProfileRows(
  [defaultProfileFixture, marketingProfileFixture, growthProfileFixture],
  "marketing"
);

interface HarnessProps {
  quiet?: boolean;
  aggregate?: boolean;
  archivedCount?: number;
  onSelectProfile?: (name: string) => void;
  onSelectAggregate?: () => void;
  onCreate?: () => void;
  onOpenSettings?: () => void;
}

function renderSwitcher({
  quiet = false,
  aggregate = false,
  archivedCount = 2,
  onSelectProfile = () => {},
  onSelectAggregate = () => {},
  onCreate = () => {},
  onOpenSettings = () => {},
}: HarnessProps = {}) {
  return render(
    <UIProvider reducedMotion="never">
      <ProfileSwitcher
        rows={quiet ? toProfileRows([defaultProfileFixture], "default") : PLURAL_ROWS}
        activeName={quiet ? "default" : "marketing"}
        aggregate={aggregate}
        quiet={quiet}
        archivedCount={archivedCount}
        onSelectProfile={onSelectProfile}
        onSelectAggregate={onSelectAggregate}
        onCreate={onCreate}
        onOpenSettings={onOpenSettings}
      />
    </UIProvider>
  );
}

describe("ProfileSwitcher", () => {
  it("Should render a neutral icon button while only default exists", () => {
    renderSwitcher({ quiet: true });
    const trigger = screen.getByTestId("os-menubar-profile");
    expect(trigger).toHaveAccessibleName("Profile");
    expect(trigger).not.toHaveTextContent("default");
  });

  it("Should become an identity element once a second profile exists", () => {
    renderSwitcher();
    const trigger = screen.getByTestId("os-menubar-profile");
    expect(trigger).toHaveTextContent("marketing");
    expect(trigger).toHaveAccessibleName("Profile: marketing");
  });

  it("Should name the aggregate rather than a profile when it is on", () => {
    renderSwitcher({ aggregate: true });
    expect(screen.getByTestId("os-menubar-profile")).toHaveTextContent("All profiles");
  });

  it("Should answer the boundary question in one sentence", async () => {
    const user = userEvent.setup();
    renderSwitcher();
    await user.click(screen.getByTestId("os-menubar-profile"));
    expect(await screen.findByText(PROFILE_BOUNDARY_ANSWER)).toBeInTheDocument();
  });

  it("Should switch to the chosen profile", async () => {
    const user = userEvent.setup();
    const onSelectProfile = vi.fn();
    renderSwitcher({ onSelectProfile });
    await user.click(screen.getByTestId("os-menubar-profile"));
    await user.click(await screen.findByTestId("profile-switcher-option-default"));
    expect(onSelectProfile).toHaveBeenCalledWith("default");
  });

  it("Should refuse a profile the runtime would reject, and say why", async () => {
    const user = userEvent.setup();
    const onSelectProfile = vi.fn();
    renderSwitcher({ onSelectProfile });
    await user.click(screen.getByTestId("os-menubar-profile"));
    const row = await screen.findByTestId("profile-switcher-option-growth");
    expect(row).toHaveTextContent("needs setup");
    expect(row).toHaveAttribute("aria-disabled", "true");
    await user.click(row);
    expect(onSelectProfile).not.toHaveBeenCalled();
  });

  it("Should offer the aggregate and creation from the menu", async () => {
    const user = userEvent.setup();
    const onSelectAggregate = vi.fn();
    const onCreate = vi.fn();
    renderSwitcher({ onSelectAggregate, onCreate });
    await user.click(screen.getByTestId("os-menubar-profile"));
    await user.click(await screen.findByTestId("profile-switcher-all"));
    expect(onSelectAggregate).toHaveBeenCalledOnce();

    await user.click(screen.getByTestId("os-menubar-profile"));
    await user.click(await screen.findByTestId("profile-switcher-create"));
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("Should demote archive management to Settings rather than listing it here", async () => {
    const user = userEvent.setup();
    const onOpenSettings = vi.fn();
    renderSwitcher({ onOpenSettings });
    await user.click(screen.getByTestId("os-menubar-profile"));
    expect(await screen.findByText("2 archived")).toBeInTheDocument();
    await user.click(screen.getByTestId("profile-switcher-settings-link"));
    expect(onOpenSettings).toHaveBeenCalledOnce();
  });

  it("Should stay silent about archives when there are none", async () => {
    const user = userEvent.setup();
    renderSwitcher({ archivedCount: 0 });
    await user.click(screen.getByTestId("os-menubar-profile"));
    await screen.findByTestId("profile-switcher-create");
    expect(screen.queryByTestId("profile-switcher-settings-link")).not.toBeInTheDocument();
  });
});
