// Suite: gateway device inventory
// Invariant: every paired device shows its name, where it was paired from, and when it was last
// active. An active device can be renamed, revoked, or removed. A revoked device stays in the
// inventory for audit and offers only Remove, which deletes its record after explicit confirmation.
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
    onRemove: vi.fn(),
    onRename: vi.fn(),
    onRevoke: vi.fn(),
    ...overrides,
  };
  render(<GatewayDeviceList {...props} />);
  return props;
}

const REVOKED = { revoked_at: "2026-08-07T09:00:00Z", revoke_epoch: 1 };

describe("GatewayDeviceList", () => {
  it("Should show name, origin, and last activity for each device", () => {
    renderList();
    const row = screen.getByTestId("gateway-device-dev_phone");

    expect(row).toHaveTextContent("Work phone");
    expect(row).toHaveTextContent("Private overlay");
    expect(row).toHaveTextContent("Last active");
  });

  it("Should fall back to the pairing time when a device has never been seen", () => {
    const device = gatewayDeviceFixture({ last_seen_at: null });
    renderList({ devices: [device] });
    const row = screen.getByTestId("gateway-device-dev_phone");

    expect(row).toHaveTextContent("Last active");
    expect(row.querySelector("time")).toHaveAttribute("datetime", device.created_at);
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

  it("Should keep a revoked device visible for audit, withdrawing rename and revoke but keeping Remove", () => {
    renderList({ devices: [gatewayDeviceFixture(REVOKED)] });
    const row = screen.getByTestId("gateway-device-dev_phone");

    expect(row).toHaveTextContent("Revoked");
    expect(screen.queryByTestId("gateway-device-dev_phone-revoke")).not.toBeInTheDocument();
    expect(screen.queryByTestId("gateway-device-dev_phone-rename")).not.toBeInTheDocument();
    expect(screen.getByTestId("gateway-device-dev_phone-remove")).toBeInTheDocument();
  });

  it("Should explain that removing an active device revokes it first and deletes its record", async () => {
    renderList();
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-remove"));

    const dialog = await screen.findByTestId("gateway-device-dev_phone-remove-dialog");
    expect(dialog).toHaveTextContent("Remove Work phone?");
    expect(dialog).toHaveTextContent("loses its session");
    expect(dialog).toHaveTextContent("live streams close");
    expect(dialog).toHaveTextContent("its record is deleted");
    expect(dialog).toHaveTextContent("This cannot be undone");
    expect(dialog).toHaveTextContent("revokes it first");
  });

  it("Should describe removing an already revoked device as deleting only its record", async () => {
    renderList({ devices: [gatewayDeviceFixture(REVOKED)] });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-remove"));

    const dialog = await screen.findByTestId("gateway-device-dev_phone-remove-dialog");
    expect(dialog).toHaveTextContent("is already revoked");
    expect(dialog).toHaveTextContent("deletes its record from this list");
    expect(dialog).not.toHaveTextContent("live streams");
    expect(dialog).not.toHaveTextContent("revokes it first");
  });

  it("Should not remove until the removal is confirmed, then remove exactly that device", async () => {
    const onRemove = vi.fn();
    renderList({ onRemove });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-remove"));

    expect(onRemove).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByTestId("gateway-device-dev_phone-remove-confirm"));

    expect(onRemove).toHaveBeenCalledTimes(1);
    expect(onRemove).toHaveBeenCalledWith(expect.objectContaining({ id: "dev_phone" }));
  });

  it("Should keep the record when the operator keeps the device", async () => {
    const onRemove = vi.fn();
    renderList({ devices: [gatewayDeviceFixture(REVOKED)], onRemove });
    fireEvent.click(screen.getByTestId("gateway-device-dev_phone-remove"));
    fireEvent.click(await screen.findByRole("button", { name: "Keep this record" }));

    expect(onRemove).not.toHaveBeenCalled();
  });

  it("Should disable every device action while another device change is in flight", () => {
    renderList({ isBusy: true });

    expect(screen.getByTestId("gateway-device-dev_phone-rename")).toBeDisabled();
    expect(screen.getByTestId("gateway-device-dev_phone-revoke")).toBeDisabled();
    expect(screen.getByTestId("gateway-device-dev_phone-remove")).toBeDisabled();
  });

  it("Should announce a device change while it is in flight and clear the announcement after", () => {
    const { rerender } = render(
      <GatewayDeviceList
        devices={[gatewayDeviceFixture()]}
        isBusy
        onRemove={vi.fn()}
        onRename={vi.fn()}
        onRevoke={vi.fn()}
      />
    );
    expect(screen.getByRole("status")).toHaveTextContent("Updating devices");

    rerender(
      <GatewayDeviceList
        devices={[gatewayDeviceFixture()]}
        isBusy={false}
        onRemove={vi.fn()}
        onRename={vi.fn()}
        onRevoke={vi.fn()}
      />
    );
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
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
