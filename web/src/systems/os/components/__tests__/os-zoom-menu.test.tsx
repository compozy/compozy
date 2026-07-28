// Suite: OS zoom menu
// Invariant: each visible menu action dispatches its semantic command for the owning window.
// Owning layer: the current OsZoomMenu component.
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useOsZoomMenu, type OsZoomMenuModel } from "../../hooks/use-os-zoom-menu";
import { OsZoomMenu } from "../os-zoom-menu";

vi.mock("../../hooks/use-os-zoom-menu", () => ({ useOsZoomMenu: vi.fn() }));

function menuModel(overrides: Partial<OsZoomMenuModel> = {}): OsZoomMenuModel {
  return {
    open: true,
    onOpenChange: vi.fn(),
    onHoverEnter: vi.fn(),
    onHoverLeave: vi.fn(),
    onContentEnter: vi.fn(),
    floating: false,
    placementEnabled: true,
    arrangeEnabled: true,
    dispatchPlacement: vi.fn(),
    dispatchMakeFloating: vi.fn(),
    dispatchFill: vi.fn(),
    dispatchArrange: vi.fn(),
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("OsZoomMenu", () => {
  it("Should dispatch placement, floating, fill, and arrange actions for its window", () => {
    const model = menuModel();
    vi.mocked(useOsZoomMenu).mockReturnValue(model);

    render(
      <OsZoomMenu windowId="window:tasks">
        <button type="button">Zoom</button>
      </OsZoomMenu>
    );

    fireEvent.click(screen.getByRole("menuitem", { name: "Tile left half" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Make window floating" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Fill window" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Arrange left & right" }));

    expect(useOsZoomMenu).toHaveBeenCalledWith("window:tasks");
    expect(model.dispatchPlacement).toHaveBeenCalledWith({
      id: "window.tile.left",
      placement: "left",
      label: "Tile left half",
    });
    expect(model.dispatchMakeFloating).toHaveBeenCalledOnce();
    expect(model.dispatchFill).toHaveBeenCalledOnce();
    expect(model.dispatchArrange).toHaveBeenCalledWith("two-up");
  });

  it("Should disable placement and arrangement controls when their actions are unavailable", () => {
    vi.mocked(useOsZoomMenu).mockReturnValue(
      menuModel({ placementEnabled: false, arrangeEnabled: false })
    );

    render(
      <OsZoomMenu windowId="window:tasks">
        <button type="button">Zoom</button>
      </OsZoomMenu>
    );

    expect(screen.getByRole("menuitem", { name: "Tile left half" })).toHaveAttribute(
      "data-disabled"
    );
    expect(screen.getByRole("menuitem", { name: "Arrange left & right" })).toHaveAttribute(
      "data-disabled"
    );
  });
});
