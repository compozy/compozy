// Suite: OS zoom menu
// Invariant: each visible menu action dispatches its semantic command for the owning window.
// Owning layer: the current OsZoomMenu component.
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { useOsZoomMenu, type OsZoomMenuModel } from "../../hooks/use-os-zoom-menu";
import { paletteRegistryFixture, resolvedPaletteCommand } from "../../mocks/cmd-palette-fixtures";
import { OsZoomMenu } from "../os-zoom-menu";

const ZOOM_REGISTRY = paletteRegistryFixture([
  resolvedPaletteCommand({ id: "window.tile.left", title: "Tile left half", section: "Window" }),
  resolvedPaletteCommand({
    id: "window.tile.right",
    title: "Tile right half",
    section: "Window",
  }),
  resolvedPaletteCommand({ id: "window.zoom", title: "Fill window", section: "Window" }),
  resolvedPaletteCommand({
    id: "layout.arrange.two-up",
    title: "Arrange left and right",
    section: "Layout",
  }),
  resolvedPaletteCommand({
    id: "layout.arrange.grid",
    title: "Arrange in grid",
    section: "Layout",
  }),
]);

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
      <CmdPaletteRegistryProvider registry={ZOOM_REGISTRY}>
        <OsZoomMenu windowId="window:tasks">
          <button type="button">Zoom</button>
        </OsZoomMenu>
      </CmdPaletteRegistryProvider>
    );

    fireEvent.click(screen.getByRole("menuitem", { name: "Tile left half" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Make window floating" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Fill window" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Arrange left and right" }));

    expect(useOsZoomMenu).toHaveBeenCalledWith("window:tasks");
    expect(model.dispatchPlacement).toHaveBeenCalledWith({
      placement: "left",
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
      <CmdPaletteRegistryProvider registry={ZOOM_REGISTRY}>
        <OsZoomMenu windowId="window:tasks">
          <button type="button">Zoom</button>
        </OsZoomMenu>
      </CmdPaletteRegistryProvider>
    );

    expect(screen.getByRole("menuitem", { name: "Tile left half" })).toHaveAttribute(
      "data-disabled"
    );
    expect(screen.getByRole("menuitem", { name: "Arrange left and right" })).toHaveAttribute(
      "data-disabled"
    );
  });

  it("Should omit zoom actions whose registry rows are missing", () => {
    vi.mocked(useOsZoomMenu).mockReturnValue(menuModel());

    render(
      <CmdPaletteRegistryProvider
        registry={paletteRegistryFixture([
          resolvedPaletteCommand({
            id: "window.tile.left",
            title: "Tile left half",
            section: "Window",
          }),
        ])}
      >
        <OsZoomMenu windowId="window:tasks">
          <button type="button">Zoom</button>
        </OsZoomMenu>
      </CmdPaletteRegistryProvider>
    );

    expect(screen.getByRole("menuitem", { name: "Tile left half" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Tile right half" })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Fill window" })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Arrange left and right" })
    ).not.toBeInTheDocument();
  });
});
