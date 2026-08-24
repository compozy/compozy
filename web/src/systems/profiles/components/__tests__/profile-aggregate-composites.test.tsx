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

import { UIProvider } from "@compozy/ui";

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

function renderWithUI(node: React.ReactNode) {
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      {node}
    </UIProvider>
  );
}

describe("ProfileOwnerTag", () => {
  it("Should name the owner in words, not colour alone", () => {
    renderWithUI(<ProfileOwnerTag owner={MARKETING} />);
    expect(screen.getByText("marketing")).toBeInTheDocument();
  });

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
});

describe("ProfileDestinationChip", () => {
  it("Should state the destination as fixed text", () => {
    renderWithUI(<ProfileDestinationChip profile="default" />);
    expect(screen.getByTestId("profile-destination-chip")).toHaveTextContent("default");
    expect(screen.getByRole("img", { name: "Will be created in default" })).toBeInTheDocument();
  });

  it("Should offer no control — it is a label, never a picker (ADR-005)", () => {
    renderWithUI(<ProfileDestinationChip profile="default" />);
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
    await userEvent.click(screen.getByTestId("profile-owner-banner-switch"));
    expect(onSwitch).toHaveBeenCalledTimes(1);
  });

  it("Should announce the owner once in the banner sentence", () => {
    renderWithUI(<ProfileOwnerBanner noun="session" owner={MARKETING} onSwitch={vi.fn()} />);
    // The sentence already names the profile; the glyph beside it is decoration.
    expect(screen.queryAllByRole("img", { name: "marketing" })).toHaveLength(0);
  });

  it("Should read as information, not as a failure", () => {
    renderWithUI(<ProfileOwnerBanner noun="session" owner={MARKETING} onSwitch={vi.fn()} />);
    expect(screen.getByTestId("profile-owner-banner")).toHaveAttribute("data-tone", "info");
  });

  it("Should hold the switch while one is already in flight", () => {
    renderWithUI(
      <ProfileOwnerBanner noun="session" owner={MARKETING} onSwitch={vi.fn()} switchPending />
    );
    expect(screen.getByTestId("profile-owner-banner-switch")).toBeDisabled();
  });
});
