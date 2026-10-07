// Suite: the three aggregate-mode composites.
// Invariant: an owner tag names its profile and mutes an archived one; the
// destination chip is fixed text, never a control; the owner banner informs and
// offers the switch without blocking the item.
// Boundary IN: what each composite renders and announces.
// Boundary OUT: which rows get tagged (the listings decide) and how a switch is
// persisted (the selection routes do).
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { identityColorsFor, identitySurfaceFor, UIProvider } from "@compozy/ui";

import { themePreferenceStore } from "@/systems/theme";

import type { ProfileOwner } from "../../lib/profile-scope";
import { ProfileDestinationChip } from "../profile-destination-chip";
import { ProfileOwnerBanner } from "../profile-owner-banner";
import { ProfileOwnerTag } from "../profile-owner-tag";

const MARKETING: ProfileOwner = {
  id: "01J9MARKETING00000000000000",
  name: "marketing",
  color: "#c26ad6",
  icon: "megaphone",
  emoji: null,
  archived: false,
};

const ARCHIVED: ProfileOwner = { ...MARKETING, id: "old", name: "old agency", archived: true };

function hexToRgb(hex: string): string {
  const [r, g, b] = [1, 3, 5].map(index => Number.parseInt(hex.slice(index, index + 2), 16));
  return `rgb(${r}, ${g}, ${b})`;
}

function renderWithUI(node: React.ReactNode) {
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      {node}
    </UIProvider>
  );
}

describe("ProfileOwnerTag", () => {
  it("Should announce the owner exactly once", () => {
    renderWithUI(<ProfileOwnerTag owner={MARKETING} />);
    // The glyph is a second rendering of the same fact. Labelling it as an image
    // too makes one two-word tag read as four to a screen reader.
    expect(screen.queryAllByRole("img", { name: "marketing" })).toHaveLength(0);
    expect(screen.getAllByText("marketing")).toHaveLength(1);
  });

  it("Should say an archived owner is archived rather than only muting it", () => {
    renderWithUI(<ProfileOwnerTag owner={ARCHIVED} />);
    expect(screen.getByText("old agency · archived")).toBeInTheDocument();
    expect(screen.getByText("old agency · archived").closest("[data-archived]")).not.toBeNull();
  });

  // Invariant: the compact tag still names its owner and forwards native span
  // behavior while showing only the colored glyph.
  // Owning layer: ProfileOwnerTag's compact contract.
  // Canonical suite: this aggregate composites suite.
  it("Should keep the compact owner accessible and preserve inherited span props", async () => {
    const onClick = vi.fn();
    renderWithUI(
      <ProfileOwnerTag
        compact
        owner={MARKETING}
        id="compact-marketing-owner"
        tabIndex={0}
        aria-describedby="owner-description"
        style={{ cursor: "pointer" }}
        onClick={onClick}
      />
    );
    expect(screen.queryByText("marketing")).not.toBeInTheDocument();
    const glyph = screen.getByRole("img", { name: "marketing" });
    expect(glyph).toHaveAttribute("title", "marketing");
    expect(glyph).toHaveAttribute("id", "compact-marketing-owner");
    expect(glyph).toHaveAttribute("tabindex", "0");
    expect(glyph).toHaveAttribute("aria-describedby", "owner-description");
    expect(glyph).toHaveStyle({ cursor: "pointer" });
    await userEvent.click(glyph);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("Should carry the archived suffix into the compact label", () => {
    renderWithUI(<ProfileOwnerTag compact owner={ARCHIVED} />);
    expect(screen.getByRole("img", { name: "old agency · archived" })).toHaveAttribute(
      "data-archived",
      "true"
    );
  });

  // Invariant: the identity plate and ink are measured against the surface of the
  // theme actually painted, so a glyph stays readable after a theme switch.
  // Owning layer: ProfileGlyph via its owner-tag host.
  it("Should paint the owner glyph against the active theme's surface", () => {
    const initial = themePreferenceStore.getSnapshot().context.preference;
    try {
      for (const theme of ["light", "dark"] as const) {
        themePreferenceStore.trigger.preferenceSet({ preference: theme });
        const { unmount } = renderWithUI(<ProfileOwnerTag owner={MARKETING} />);
        const glyph = document.querySelector<HTMLElement>('[data-slot="profile-glyph"]')!;
        const expected = identityColorsFor(MARKETING.color, identitySurfaceFor(theme));
        expect(glyph.style.backgroundColor, theme).toBe(hexToRgb(expected.bg));
        expect(glyph.style.color, theme).toBe(hexToRgb(expected.fg));
        unmount();
      }
    } finally {
      themePreferenceStore.trigger.preferenceSet({ preference: initial });
    }
  });
});

describe("ProfileDestinationChip", () => {
  it("Should state the destination as fixed text", () => {
    renderWithUI(<ProfileDestinationChip profile="default" />);
    expect(screen.getByTestId("profile-destination-chip")).toHaveTextContent("default");
    expect(screen.getByRole("img", { name: "Will be created in default" })).toBeInTheDocument();
    const chip = screen.getByTestId("profile-destination-chip");
    expect(chip.querySelector("button, select, input, a")).toBeNull();
    expect(chip.tagName).toBe("SPAN");
  });
});

describe("ProfileOwnerBanner", () => {
  it("Should name the owner and offer exactly one move", async () => {
    const onSwitch = vi.fn();
    renderWithUI(<ProfileOwnerBanner noun="session" owner={MARKETING} onSwitch={onSwitch} />);
    expect(screen.getByTestId("profile-owner-banner")).toHaveTextContent(
      "This session belongs to marketing."
    );
    expect(screen.queryAllByRole("img", { name: "marketing" })).toHaveLength(0);
    expect(screen.getByTestId("profile-owner-banner")).toHaveAttribute("data-tone", "info");
    await userEvent.click(screen.getByTestId("profile-owner-banner-switch"));
    expect(onSwitch).toHaveBeenCalledTimes(1);
  });

  it("Should hold the switch while one is already in flight", () => {
    renderWithUI(
      <ProfileOwnerBanner noun="session" owner={MARKETING} onSwitch={vi.fn()} switchPending />
    );
    expect(screen.getByTestId("profile-owner-banner-switch")).toBeDisabled();
  });
});
