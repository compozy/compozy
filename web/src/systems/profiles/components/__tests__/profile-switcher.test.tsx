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
  manageable?: boolean;
  onSelectProfile?: (name: string) => void;
  onSelectAggregate?: () => void;
  onCreate?: () => void;
  onEditProfile?: (name: string) => void;
  onOpenSettings?: () => void;
}

function renderSwitcher({
  quiet = false,
  aggregate = false,
  archivedCount = 2,
  manageable = true,
  onSelectProfile = () => {},
  onSelectAggregate = () => {},
  onCreate = () => {},
  onEditProfile,
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
        manageable={manageable}
        onSelectProfile={onSelectProfile}
        onSelectAggregate={onSelectAggregate}
        onCreate={onCreate}
        {...(onEditProfile ? { onEditProfile } : {})}
        onOpenSettings={onOpenSettings}
      />
    </UIProvider>
  );
}

describe("ProfileSwitcher", () => {
  it("Should render a neutral icon button without a redundant aggregate while only default exists", async () => {
    const user = userEvent.setup();
    renderSwitcher({ quiet: true, archivedCount: 0 });
    const trigger = screen.getByTestId("os-menubar-profile");
    expect(trigger).toHaveAccessibleName("Profile");
    expect(trigger).not.toHaveTextContent("default");

    await user.click(trigger);
    expect(screen.queryByTestId("profile-switcher-all")).not.toBeInTheDocument();
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

  it("Should name each profile once, not twice, for a screen reader", async () => {
    const user = userEvent.setup();
    renderSwitcher({});
    await user.click(screen.getByTestId("os-menubar-profile"));
    const row = await screen.findByTestId("profile-switcher-option-default");
    // The glyph re-renders a fact the row already states, so it must not carry a
    // label of its own — otherwise every row announces its profile twice.
    expect(row.querySelector('[data-slot="profile-glyph"]')).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryAllByRole("img", { name: "default" })).toHaveLength(0);
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

  it("Should keep remote profile rows read-only while preserving the local aggregate view", async () => {
    const user = userEvent.setup();
    const onSelectProfile = vi.fn();
    const onSelectAggregate = vi.fn();
    renderSwitcher({ manageable: false, onSelectAggregate, onSelectProfile });

    await user.click(screen.getByTestId("os-menubar-profile"));
    const profile = await screen.findByTestId("profile-switcher-option-default");
    expect(profile).toHaveAttribute("aria-disabled", "true");
    await user.click(profile);
    expect(onSelectProfile).not.toHaveBeenCalled();
    expect(screen.queryByTestId("profile-switcher-create")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("profile-switcher-all"));
    expect(onSelectAggregate).toHaveBeenCalledOnce();
  });

  // Invariant: a row's edit affordance raises the canonical identity dialog for
  // that profile without switching into it, and never appears on rows the
  // runtime would refuse.
  // Owning layer: the switcher composition.
  // Canonical suite: this ProfileSwitcher interaction suite.
  it("Should edit a profile from its row without switching into it", async () => {
    const user = userEvent.setup();
    const onEditProfile = vi.fn();
    const onSelectProfile = vi.fn();
    renderSwitcher({ onEditProfile, onSelectProfile });
    await user.click(screen.getByTestId("os-menubar-profile"));
    await user.click(await screen.findByTestId("profile-switcher-edit-default"));
    expect(onEditProfile).toHaveBeenCalledWith("default");
    expect(onSelectProfile).not.toHaveBeenCalled();
    expect(screen.queryByTestId("profile-switcher-edit-growth")).not.toBeInTheDocument();
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
