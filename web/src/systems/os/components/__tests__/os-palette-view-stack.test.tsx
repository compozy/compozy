// Suite: OS palette view stack integration
// Invariant: the stack adapts profiles-domain content without owning profile lifecycle behavior,
// and pushed domain rows use the root-owned opener that survives view dismissal.
// Boundary IN: a profiles controller projection and the profiles view registration.
// Boundary OUT: profile queries, switching, and lifecycle dialogs.

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

const profileMocks = vi.hoisted(() => ({
  useProfilesPaletteView: vi.fn(),
}));

vi.mock("@/systems/profiles", () => ({
  useProfileLens: () => ({ scope: "global" as const }),
  useProfilesPaletteView: profileMocks.useProfilesPaletteView,
}));

import type { CmdPaletteDispatch } from "../../hooks/use-cmd-palette-dispatch";
import { paletteViewDefinition } from "../../lib/palette-view-registry";
import { OsPaletteViewStack } from "../os-palette-view-stack";

const dispatch: CmdPaletteDispatch = {
  run: vi.fn(async () => ({ status: "ran" }) as const),
  runById: vi.fn(async () => ({ status: "ran" }) as const),
  executeClientOp: vi.fn(async () => undefined),
  setPinned: vi.fn(async () => undefined),
};

describe("OsPaletteViewStack profiles integration", () => {
  it("Should render profiles-domain rows through the shared stack chrome", () => {
    profileMocks.useProfilesPaletteView.mockReturnValue({
      rows: [
        {
          value: "profile:marketing",
          testId: "profile-row-marketing",
          node: <span>Marketing profile</span>,
          onSelect: vi.fn(),
        },
      ],
      header: null,
      empty: <span>No profiles</span>,
      note: null,
      backHint: "Profiles",
      resetKey: "profiles",
      onEmptyQueryBackspace: () => false,
    });

    render(
      <UIProvider reducedMotion="always">
        <OsPaletteViewStack
          breadcrumb={{ truncated: false, visible: ["Profiles"] }}
          client={null}
          dispatch={dispatch}
          onDismiss={vi.fn()}
          onPop={vi.fn()}
          openDomainRow={vi.fn()}
          viewId="profiles"
        />
      </UIProvider>
    );

    expect(screen.getByPlaceholderText("Switch profile…")).toBeVisible();
    expect(screen.getByTestId("os-palette-footer")).toHaveTextContent("switch");
    expect(paletteViewDefinition("profiles")).toMatchObject({
      placeholder: "Switch profile…",
      enterHint: "switch",
      description: "Switch, create, and manage profiles",
    });
    expect(screen.getByTestId("profile-row-marketing")).toHaveTextContent("Marketing profile");
    expect(profileMocks.useProfilesPaletteView).toHaveBeenCalledWith(
      expect.objectContaining({ lens: { scope: "global" }, query: "" })
    );
  });
});
