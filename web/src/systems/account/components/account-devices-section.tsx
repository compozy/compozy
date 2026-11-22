import { useState } from "react";
import { ShieldAlert, ShieldCheck } from "@compozy/ui/icons";

import { Button, Checkbox, ConfirmDialog, ListingRow, Pill, Switch } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { KeyFingerprint } from "@/systems/computers/components/key-fingerprint";
import type {
  AccountDeviceFixture,
  DeviceKind,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

const DEVICE_KIND_LABEL: Record<DeviceKind, string> = {
  desktop: "Desktop",
  browser: "Browser",
  cli: "CLI",
};

export interface AccountDevicesSectionProps {
  devices: readonly AccountDeviceFixture[];
  currentDeviceIsAdmin: boolean;
  onToggleAdmin: (device: AccountDeviceFixture, nextAdmin: boolean) => void;
  onRequestRevoke: (device: AccountDeviceFixture) => void;
}

/**
 * Account-wide devices list (`_uiux.md` U3, US-020):
 * - Pins `This device` first with its status pill (no switch, no self-revoke)
 * - Other devices show a single labeled `Trusted for admin` switch + `Revoke`
 */
export function AccountDevicesSection({
  devices,
  currentDeviceIsAdmin,
  onToggleAdmin,
  onRequestRevoke,
}: AccountDevicesSectionProps) {
  const ordered = [...devices].sort((a, b) => {
    if (a.thisDevice !== b.thisDevice) return a.thisDevice ? -1 : 1;
    return 0;
  });

  const adminCount = devices.filter(device => device.admin).length;

  return (
    <SettingsGroup
      data-testid="account-devices"
      description={
        currentDeviceIsAdmin
          ? "Revoking a device closes its live sessions on every connected computer within seconds."
          : "Only a device trusted for admin can change admin trust or revoke other devices."
      }
      help="Devices trusted for admin can approve new devices, promote or demote other devices, and revoke lost devices across your account."
      title={`Devices (${devices.length})`}
    >
      {ordered.map(device => {
        const isLastAdmin = device.admin && adminCount <= 1;
        const canEditAdmin = currentDeviceIsAdmin && !isLastAdmin;
        const canRevoke = currentDeviceIsAdmin && !device.thisDevice;

        return (
          <ListingRow
            className="min-h-14 py-3"
            data-device-id={device.id}
            data-this-device={device.thisDevice ? "true" : undefined}
            interactive={false}
            key={device.id}
          >
            <ListingRow.Icon>
              <ComputerPlatformIcon
                deviceKind={device.kind}
                size="sm"
                tone={device.thisDevice ? "default" : "muted"}
              />
            </ListingRow.Icon>

            <ListingRow.Main>
              <ListingRow.Name className="flex-wrap gap-y-1">
                <ListingRow.Title>{device.name}</ListingRow.Title>
                {device.thisDevice ? (
                  <Pill size="xs" tone="neutral">
                    This device
                  </Pill>
                ) : null}
                {device.thisDevice && device.admin ? (
                  <Pill size="xs" tone="info">
                    <ShieldCheck aria-hidden="true" className="size-3" />
                    Trusted for admin
                  </Pill>
                ) : null}
              </ListingRow.Name>

              <ListingRow.Meta className="mt-1">
                <span>{DEVICE_KIND_LABEL[device.kind]}</span>
                <ListingRow.MetaDot />
                <span>Last seen {device.lastSeenLabel}</span>
                <ListingRow.MetaDot />
                <KeyFingerprint copyable={false} layout="inline" value={device.keyFingerprint} />
              </ListingRow.Meta>
            </ListingRow.Main>

            <ListingRow.Trail>
              {!device.thisDevice ? (
                <div className="flex shrink-0 items-center gap-3">
                  <div className="flex items-center gap-2">
                    <label
                      className="text-meta text-muted-foreground select-none"
                      htmlFor={`device-admin-${device.id}`}
                    >
                      Trusted for admin
                    </label>
                    <Switch
                      aria-label={`Trusted for admin for ${device.name}`}
                      checked={device.admin}
                      disabled={!canEditAdmin}
                      id={`device-admin-${device.id}`}
                      onCheckedChange={next => onToggleAdmin(device, next)}
                      size="sm"
                    />
                  </div>
                  <Button
                    disabled={!canRevoke}
                    onClick={() => onRequestRevoke(device)}
                    size="sm"
                    title={
                      canRevoke
                        ? `Revoke ${device.name}`
                        : "Only a device trusted for admin can do this."
                    }
                    type="button"
                    variant="outline"
                  >
                    Revoke
                  </Button>
                </div>
              ) : null}
            </ListingRow.Trail>
          </ListingRow>
        );
      })}
    </SettingsGroup>
  );
}

export interface AccountRevokeDeviceDialogProps {
  device: AccountDeviceFixture | null;
  requireDeviceApproval: boolean;
  onClose: () => void;
  onConfirm: (device: AccountDeviceFixture, enableApproval: boolean) => void;
}

/**
 * Account-level device revocation confirm (`_uiux.md` U3, US-020, `research-references` §4.1):
 * includes the approval safeguard when any host has device approval turned off.
 */
export function AccountRevokeDeviceDialog({
  device,
  requireDeviceApproval,
  onClose,
  onConfirm,
}: AccountRevokeDeviceDialogProps) {
  const [enableApproval, setEnableApproval] = useState(!requireDeviceApproval);

  if (!device) return null;

  return (
    <ConfirmDialog
      body={
        !requireDeviceApproval ? (
          <label
            className="flex cursor-pointer items-start gap-2.5"
            htmlFor="account-revoke-require-approval"
          >
            <Checkbox
              checked={enableApproval}
              className="mt-0.5"
              id="account-revoke-require-approval"
              onCheckedChange={next => setEnableApproval(Boolean(next))}
            />
            <span className="flex flex-col gap-0.5">
              <span className="text-small-body font-medium text-fg">
                Require approval for new devices on this computer
              </span>
              <span className="text-meta text-muted-foreground">
                Hold new or re-signed-in devices until you approve their fingerprint.
              </span>
            </span>
          </label>
        ) : undefined
      }
      cancelLabel="Cancel"
      confirmLabel="Revoke device"
      description={`${device.name} loses access across your account. Every connected computer closes its streams within 2 seconds; offline computers apply the revoke when they reconnect.`}
      footNote={<KeyFingerprint copyable={false} layout="inline" value={device.keyFingerprint} />}
      icon={ShieldAlert}
      note={
        !requireDeviceApproval
          ? "Approval for new devices is off on this computer, so a revoked device signed in to your account can connect again on its own."
          : undefined
      }
      noteTone="warning"
      onConfirm={() => onConfirm(device, enableApproval)}
      onOpenChange={open => {
        if (!open) onClose();
      }}
      open
      title={`Revoke ${device.name}?`}
      tone="destructive"
    />
  );
}
