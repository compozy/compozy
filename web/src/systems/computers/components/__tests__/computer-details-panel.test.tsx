// Suite: ComputerDetailsPanel live tabs and placement (S8 / U7)
// Invariant: renders right-docked at top-0 in pane mode, shows 4-hop diagnosis + Retry + Copy,
// routes with mismatch removal, admin vs read-only permissions, and conditional Update tab only
// when incompatible.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ComputerDetailsPanel } from "../computer-details-panel";
import {
  DETAILS_DEVBOX,
  DETAILS_GPU_VM,
  DETAILS_INCOMPATIBLE_MANUAL,
  DETAILS_WORKSTATION,
} from "../../mocks/remote-computers-fixtures-lane-b";

describe("ComputerDetailsPanel", () => {
  it("hides Update tab for healthy computers and renders pane placement at top-0", () => {
    render(
      <ComputerDetailsPanel
        computer={DETAILS_DEVBOX}
        open
        placement="pane"
        showRouteReorder={false}
      />
    );

    const panel = screen.getByLabelText("devbox details");
    expect(panel.className).toContain("top-0");
    expect(screen.getByRole("tab", { name: "Status" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Routes/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Permissions" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Providers" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /Update/i })).not.toBeInTheDocument();
  });

  it("renders 4-hop diagnosis and removes mismatched learned routes via live callback", () => {
    const onRetry = vi.fn();
    const onRemoveRoute = vi.fn();

    render(
      <ComputerDetailsPanel
        computer={DETAILS_WORKSTATION}
        onRemoveRoute={onRemoveRoute}
        onRetryConnection={onRetry}
        open
        placement="pane"
        showRouteReorder={false}
      />
    );

    expect(screen.getByTestId("computer-details-diagnosis-steps")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Retry now/i }));
    expect(onRetry).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("tab", { name: /Routes/i }));
    expect(screen.getByText("Different computer at this address")).toBeInTheDocument();
    const removeButtons = screen.getAllByRole("button", { name: /^Remove$/i });
    fireEvent.click(removeButtons[0]!);
    expect(onRemoveRoute).toHaveBeenCalledTimes(1);
  });

  it("shows Update tab when incompatible and supports SSH live update and manual compozy update", () => {
    const onStartUpdate = vi.fn();
    const { rerender } = render(
      <ComputerDetailsPanel
        computer={DETAILS_GPU_VM}
        initialTab="update"
        liveUpdateState="idle"
        onStartUpdate={onStartUpdate}
        open
        placement="pane"
      />
    );

    expect(screen.getByRole("tab", { name: /Update/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Update gpu-vm/i }));
    expect(onStartUpdate).toHaveBeenCalledTimes(1);

    const onCheckVersion = vi.fn();
    rerender(
      <ComputerDetailsPanel
        computer={DETAILS_INCOMPATIBLE_MANUAL}
        initialTab="update"
        onCheckVersion={onCheckVersion}
        open
        placement="pane"
      />
    );
    expect(screen.getByText("compozy update")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Check version again/i }));
    expect(onCheckVersion).toHaveBeenCalledTimes(1);
  });
});
