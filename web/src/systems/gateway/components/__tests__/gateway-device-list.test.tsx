// Suite: gateway device inventory
// Invariant: every paired device shows its name, where it was paired from, and when it was last
// active, and an active device can be renamed and revoked immediately. A revoked device stays in
// the inventory for audit but offers no actions.
// Owning layer: the gateway device inventory components.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { gatewayDeviceFixture } from "../../mocks/fixtures";
import { GatewayDeviceList } from "../gateway-device-list";

function renderList(overrides: Partial<React.ComponentProps<typeof GatewayDeviceList>> = {}) {
  const props = {
    devices: [gatewayDeviceFixture()],
    isBusy: false,
    onPair: vi.fn(),
    onRename: vi.fn(),
    onRevoke: vi.fn(),
    ...overrides,
  };
  render(<GatewayDeviceList {...props} />);
  return props;
}

describe("GatewayDeviceList", () => {
  it("Should show name, origin, and last activity for each device", () => {
    renderList();
    const row = screen.getByTestId("gateway-device-dev_phone");

    expect(row).toHaveTextContent("Work phone");
    expect(row).toHaveTextContent("Private overlay");
    expect(row).toHaveTextContent("Last active");
  });

  it("Should fall back to the pairing time when a device has never been seen", () => {
    renderList({ devices: [gatewayDeviceFixture({ last_seen_at: null })] });
    expect(screen.getByTestId("gateway-device-dev_phone")).toHaveTextContent("Last active");
  });

  it("Should rename a device from the inventory", () => {
    const { onRename } = renderList();
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-rename"));

    const input = screen.getByTestId("gateway-device-dev_phone-name-input");
    fireEvent.change(input, { target: { value: "Kitchen tablet" } });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-rename-save"));

    expect(onRename).toHaveBeenCalledWith("dev_phone", "Kitchen tablet");
  });

  it("Should not submit an unchanged or blank name", () => {
    const { onRename } = renderList();
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-rename"));
    fireEvent.change(screen.getByTestId("gateway-device-dev_phone-name-input"), {
      target: { value: "   " },
    });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-rename-save"));

    expect(onRename).not.toHaveBeenCalled();
  });

  it("Should not revoke until the terminal consequence is confirmed", async () => {
    const onRevoke = vi.fn();
    renderList({ onRevoke });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-revoke"));

    const dialog = await screen.findByTestId("gateway-device-dev_phone-revoke-dialog");
    expect(dialog).toHaveTextContent("loses its session immediately");
    expect(dialog).toHaveTextContent("live streams are closed");
    expect(dialog).toHaveTextContent("needs a new pairing code");
    expect(onRevoke).not.toHaveBeenCalled();
  });

  it("Should revoke once confirmed", async () => {
    const onRevoke = vi.fn();
    renderList({ onRevoke });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-revoke"));
    fireEvent.click(await screen.findByTestId("gateway-device-dev_phone-revoke-confirm"));

    expect(onRevoke).toHaveBeenCalledTimes(1);
    expect(onRevoke).toHaveBeenCalledWith(expect.objectContaining({ id: "dev_phone" }));
  });

  it("Should leave the device untouched when the operator backs out", async () => {
    const onRevoke = vi.fn();
    renderList({ onRevoke });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-revoke"));
    fireEvent.click(await screen.findByRole("button", { name: "Keep this device" }));

    expect(onRevoke).not.toHaveBeenCalled();
  });

  it("Should keep a revoked device visible for audit but offer no actions", () => {
    renderList({
      devices: [gatewayDeviceFixture({ revoked_at: "2026-08-07T09:00:00Z", revoke_epoch: 1 })],
    });
    const row = screen.getByTestId("gateway-device-dev_phone");

    expect(row).toHaveTextContent("Revoked");
    expect(screen.queryByTestId("gateway-device-dev_phone-revoke")).not.toBeInTheDocument();
    expect(screen.queryByTestId("gateway-device-dev_phone-rename")).not.toBeInTheDocument();
  });

  it("Should explain the empty inventory and point at pairing", () => {
    renderList({ devices: [] });

    expect(screen.getByText("No paired devices yet")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-pair-device")).toBeInTheDocument();
  });

  it("Should report a refused device change rather than showing it as applied", () => {
    renderList({ error: "gateway device not found" });
    expect(screen.getByRole("alert")).toHaveTextContent("gateway device not found");
  });
});
