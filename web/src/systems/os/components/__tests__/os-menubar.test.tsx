// Suite: OsMenuBar touch tier
// Invariant: at the touch tier the identity segment is the give-way element —
// the scope label truncates through an unbroken min-w-0 chain and the trailing
// actions keep their 44px floor; no scope state may paint under the cluster
// (F3 real-device defect). Desktop rendering keeps its exact classes.
// Boundary IN: OsMenuBar presentation, truncation chain, touch floors.
// Boundary OUT: scope model wiring (desktop-menubar suite), menu contents.
import { render } from "@testing-library/react";
import type * as React from "react";
import { describe, expect, it } from "vitest";

import { OsMenuBar } from "../os-menubar";

const SQUARE_FLOOR = "max-[760px]:size-(--height-button-cta-lg)";

function renderBar(workspace: React.ComponentProps<typeof OsMenuBar>["workspace"], touch: boolean) {
  const view = render(
    <OsMenuBar
      touch={touch}
      workspace={workspace}
      notifications={2}
      onCommandClick={() => undefined}
      onSettingsClick={() => undefined}
    />
  );
  const slot = (name: string) => view.container.querySelector(`[data-slot="${name}"]`);
  return { view, slot };
}

describe("OsMenuBar touch tier", () => {
  it("Should build the min-w-0 truncation chain in Global scope at the touch tier", () => {
    const { slot } = renderBar({ name: "Global", monogram: "~" }, true);

    // Every wrapper between the shrinking leading div and the label allows
    // the squeeze, so "Global" truncates instead of spilling under the
    // trailing cluster.
    expect(slot("os-menubar-identity")).toHaveClass("min-w-0");
    const chip = slot("os-menubar-workspace");
    expect(chip).toHaveClass("min-w-0");
    const label = chip!.querySelector("span.truncate");
    expect(label).toHaveTextContent("Global");
    expect(label).toHaveClass("min-w-0", "max-w-28");
  });

  it("Should keep the mark fixed and the trailing actions at the 44px floor", () => {
    const { slot } = renderBar({ name: "Global", monogram: "~" }, true);

    expect(slot("os-menubar-logo")).toHaveClass("shrink-0", SQUARE_FLOOR);
    expect(slot("os-menubar-bell")).toHaveClass(SQUARE_FLOOR);
    expect(slot("os-menubar-command")).toHaveClass(SQUARE_FLOOR);
    expect(slot("os-menubar-settings")).toHaveClass(SQUARE_FLOOR);
  });

  it("Should keep the truncation chain for named workspaces at the touch tier", () => {
    const { slot } = renderBar({ name: "thaymel", monogram: "TH", worktree: "mobile-shell" }, true);

    // Worktree segment steps aside at touch; the workspace label carries the
    // same give-way chain regardless of scope state.
    expect(slot("os-menubar-worktree")).not.toBeInTheDocument();
    const chip = slot("os-menubar-workspace");
    expect(chip!.textContent).toContain("thaymel");
    expect(chip!.querySelector("span.truncate")).toHaveClass("min-w-0");
  });

  it("Should render desktop classes untouched without the touch tier", () => {
    const { slot } = renderBar({ name: "compozy", monogram: "CO", worktree: "main" }, false);

    expect(slot("os-menubar-identity")).not.toHaveClass("min-w-0");
    const chip = slot("os-menubar-workspace");
    expect(chip).not.toHaveClass("min-w-0");
    // The scope label never truncates on desktop (the worktree span does).
    const label = chip!.querySelector("span.truncate.text-fg-strong");
    expect(label).toBeNull();
    expect(slot("os-menubar-worktree")).toBeInTheDocument();
    expect(slot("os-menubar-bell")).not.toHaveClass(SQUARE_FLOOR);
  });
});
