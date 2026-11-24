import { useState } from "react";
import { Check, ShieldCheck, Sparkles, TriangleAlert, X } from "@compozy/ui/icons";

import {
  Button,
  Checkbox,
  cn,
  CopyIconButton,
  Disclosure,
  Pill,
  Spinner,
  Switch,
} from "@compozy/ui";

import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import {
  DISPLAYED_REMOTE_SCOPES,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type LocalComputerFixture,
  type PendingApprovalFixture,
  type RemoteScope,
} from "../mocks/remote-computers-types";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { KeyFingerprint } from "./key-fingerprint";

export interface ComputersThisComputerSectionProps {
  account: AccountStateFixture;
  local: LocalComputerFixture;
  hasRemoteComputers: boolean;
  devices: readonly AccountDeviceFixture[];
  pendingApprovals: readonly PendingApprovalFixture[];
  defaultDevicesOpen?: boolean;
  onRequestToggleRunAgents: (next: boolean) => void;
  onToggleRemoteEnabled: (next: boolean) => void;
  onToggleHostedRoute?: (next: boolean) => void;
  onToggleRequireApproval: (next: boolean) => void;
  onToggleDeviceScope: (deviceId: string, scope: RemoteScope, next: boolean) => void;
  onApprovePending: (approvalId: string, deviceId: string) => void;
  onDenyPending: (approvalId: string, deviceId: string) => void;
  onRequestRevokeDevice: (device: AccountDeviceFixture) => void;
  onUpgrade?: () => void;
}

function HostedRouteDescription({
  account,
  local,
}: Pick<ComputersThisComputerSectionProps, "account" | "local">) {
  if (!local.hostedRouteEnabled) {
    return (
      <span>
        Reach this computer from any network or your phone ({account.cloudRoutesUsed} of{" "}
        {account.cloudRoutesLimit} hosted routes used).
      </span>
    );
  }
  if (local.hostedRouteState === "ready") {
    return (
      <span className="inline-flex flex-wrap items-center gap-1.5">
        <Pill form="plain" size="xs" tone="success">
          <Pill.Dot />
          Hosted route ready
        </Pill>
        {local.hostedUrl ? (
          <>
            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
            <span className="font-mono text-mono-id text-fg">{local.hostedUrl}</span>
            <CopyIconButton
              className="size-5 text-subtle hover:text-fg"
              copiedLabel="Hosted address copied"
              copiedToastLabel="Hosted address copied"
              copyLabel="Copy hosted route address"
              value={`https://${local.hostedUrl}`}
            />
          </>
        ) : null}
      </span>
    );
  }
  if (local.hostedRouteState === "starting") {
    return (
      <span className="inline-flex items-center gap-1.5 text-muted-foreground">
        <Spinner aria-hidden="true" className="size-3" />
        <span>Starting hosted route…</span>
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 text-warning">
      <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0" />
      <span className="truncate">
        {local.hostedRouteError ?? "Hosted route is temporarily unreachable"}
      </span>
    </span>
  );
}

function HostedRouteSubRow({
  account,
  local,
  onToggleHostedRoute,
  onUpgrade,
}: Pick<
  ComputersThisComputerSectionProps,
  "account" | "local" | "onToggleHostedRoute" | "onUpgrade"
>) {
  if (account.plan !== "pro") {
    return (
      <SettingRow
        className="pl-8"
        control={
          <div className="flex items-center gap-2">
            <Pill size="xs" tone="brand">
              Part of Pro
            </Pill>
            {onUpgrade ? (
              <Button onClick={onUpgrade} size="xs" type="button" variant="secondary">
                <Sparkles aria-hidden="true" />
                Upgrade
              </Button>
            ) : null}
          </div>
        }
        description="Hosted route is part of Pro. Reach this computer from any network or your phone."
        label="Hosted route"
      />
    );
  }

  return (
    <SettingRow
      className="pl-8"
      control={
        onToggleHostedRoute ? (
          <Switch
            aria-label="Hosted route"
            checked={local.hostedRouteEnabled}
            onCheckedChange={onToggleHostedRoute}
            size="sm"
          />
        ) : (
          <Pill size="xs" tone={local.hostedRouteEnabled ? "success" : "neutral"}>
            {local.hostedRouteEnabled ? "On" : "Off"}
          </Pill>
        )
      }
      description={<HostedRouteDescription account={account} local={local} />}
      label="Hosted route"
    />
  );
}

function LocalDeviceBadges({ admin, thisDevice }: { admin: boolean; thisDevice: boolean }) {
  return (
    <>
      {thisDevice ? (
        <Pill size="xs" tone="neutral">
          This device
        </Pill>
      ) : null}
      {admin ? (
        <Pill size="xs" tone="info">
          <ShieldCheck aria-hidden="true" className="size-3" />
          Trusted for admin
        </Pill>
      ) : null}
    </>
  );
}

function LocalDeviceSummary({
  device,
  grantedCount,
}: {
  device: AccountDeviceFixture;
  grantedCount: number;
}) {
  const permissionsSummary =
    grantedCount === DISPLAYED_REMOTE_SCOPES.length
      ? "All permissions"
      : `${grantedCount} of ${DISPLAYED_REMOTE_SCOPES.length} permissions`;

  return (
    <div className="flex min-w-0 items-center gap-2.5">
      <ComputerPlatformIcon deviceKind={device.kind} size="sm" tone="muted" />
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="truncate text-small-body font-medium text-fg">{device.name}</span>
          <LocalDeviceBadges admin={device.admin} thisDevice={device.thisDevice} />
        </div>
        <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-eyebrow text-faint">
          <span>Last seen {device.lastSeenLabel}</span>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <span>{permissionsSummary}</span>
        </div>
      </div>
    </div>
  );
}

function LocalDeviceRow({
  device,
  onToggleDeviceScope,
  onRequestRevokeDevice,
}: {
  device: AccountDeviceFixture;
  onToggleDeviceScope: (deviceId: string, scope: RemoteScope, next: boolean) => void;
  onRequestRevokeDevice: (device: AccountDeviceFixture) => void;
}) {
  const [scopesOpen, setScopesOpen] = useState(false);
  const granted = new Set(device.scopesOnLocal);
  const grantedCount = DISPLAYED_REMOTE_SCOPES.filter(item => granted.has(item.scope)).length;

  return (
    <div
      className="border-t border-line-soft py-2.5 first:border-t-0"
      data-local-device-id={device.id}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <LocalDeviceSummary device={device} grantedCount={grantedCount} />

        <div className="flex shrink-0 items-center gap-2">
          {!device.thisDevice ? (
            <Button
              onClick={() => setScopesOpen(current => !current)}
              size="xs"
              type="button"
              variant="ghost"
            >
              {scopesOpen ? "Hide permissions" : "Permissions"}
            </Button>
          ) : null}
          {!device.thisDevice ? (
            <Button
              onClick={() => onRequestRevokeDevice(device)}
              size="xs"
              type="button"
              variant="outline"
            >
              Revoke
            </Button>
          ) : null}
        </div>
      </div>

      {scopesOpen && !device.thisDevice ? (
        <div className="mt-2.5 grid grid-cols-2 gap-x-5 gap-y-2 border-t border-line-soft pt-2.5 pl-6.5 sm:grid-cols-3">
          {DISPLAYED_REMOTE_SCOPES.map(item => {
            const checked = granted.has(item.scope);
            const id = `local-scope-${device.id}-${item.scope}`;
            return (
              <label
                className="flex cursor-pointer items-center gap-2 text-meta text-fg"
                htmlFor={id}
                key={item.scope}
              >
                <Checkbox
                  checked={checked}
                  id={id}
                  onCheckedChange={next =>
                    onToggleDeviceScope(device.id, item.scope, Boolean(next))
                  }
                />
                <span className={cn(!checked && "text-muted-foreground")}>{item.label}</span>
              </label>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

/**
 * Block 1 of Settings → Computers (`_uiux.md` U1):
 * Run agents toggle, Remote access toggle, and indented Hosted route,
 * Require approval, and Devices sub-rows inside a single panelbox.
 */
export function ComputersThisComputerSection({
  account,
  local,
  hasRemoteComputers,
  devices,
  pendingApprovals,
  defaultDevicesOpen = false,
  onRequestToggleRunAgents,
  onToggleRemoteEnabled,
  onToggleHostedRoute,
  onToggleRequireApproval,
  onToggleDeviceScope,
  onApprovePending,
  onDenyPending,
  onRequestRevokeDevice,
  onUpgrade,
}: ComputersThisComputerSectionProps) {
  const localApprovals = pendingApprovals.filter(
    item => item.state === "pending" || item.state === "delivering"
  );
  const runAgentsDisabled = !hasRemoteComputers && local.localRunAgents;

  return (
    <SettingsGroup
      data-testid="computers-this-computer"
      title={
        local.name && local.name !== "This computer"
          ? `This computer · ${local.name}`
          : "This computer"
      }
    >
      <SettingRow
        control={
          <Switch
            aria-label="Run agents on this computer"
            checked={local.localRunAgents}
            disabled={runAgentsDisabled}
            onCheckedChange={onRequestToggleRunAgents}
            size="sm"
          />
        }
        description={
          runAgentsDisabled
            ? "Add a computer first before turning off agents on this computer."
            : local.localRunAgents
              ? "Turn off to use this app only to control your other computers."
              : "This computer only controls your other computers. Existing local sessions stay readable."
        }
        label="Run agents on this computer"
      />

      <SettingRow
        control={
          <Switch
            aria-label="Remote access"
            checked={local.remoteEnabled}
            onCheckedChange={onToggleRemoteEnabled}
            size="sm"
          />
        }
        description={
          local.remoteEnabled
            ? "Your other devices can connect to this computer over LAN, SSH, or a hosted route."
            : "No other device can connect to this computer while remote access is off."
        }
        label="Remote access"
      />

      {local.remoteEnabled ? (
        <>
          <HostedRouteSubRow
            account={account}
            local={local}
            onToggleHostedRoute={onToggleHostedRoute}
            onUpgrade={onUpgrade}
          />

          <SettingRow
            className="pl-8"
            control={
              <Switch
                aria-label="Require approval for new devices"
                checked={local.requireDeviceApproval}
                onCheckedChange={onToggleRequireApproval}
                size="sm"
              />
            }
            description="Hold the first connection from an unknown device until you approve it here or on a device trusted for admin."
            help="When off, any device signed in to your CompozyOS account can connect automatically."
            label="Require approval for new devices"
          />

          <div className="border-t border-line-soft px-4 py-2.5 pl-8">
            <Disclosure
              defaultOpen={defaultDevicesOpen}
              label={
                <span className="inline-flex items-center gap-2 text-body font-medium text-fg">
                  <span>Devices ({devices.length})</span>
                  {localApprovals.length > 0 ? (
                    <Pill size="xs" tone="warning">
                      {localApprovals.length} waiting for approval
                    </Pill>
                  ) : null}
                </span>
              }
              size="md"
              variant="inline"
            >
              <div className="mt-1 flex flex-col">
                {localApprovals.map(approval => (
                  <div
                    className="border-t border-line-soft py-2.5 first:border-t-0"
                    data-approval-id={approval.id}
                    key={approval.id}
                  >
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="flex min-w-0 items-start gap-2.5">
                        <ComputerPlatformIcon
                          className="mt-0.5"
                          deviceKind={approval.deviceKind}
                          size="sm"
                          tone="muted"
                        />
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="truncate text-small-body font-medium text-fg">
                              {approval.deviceName}
                            </span>
                            {approval.state === "delivering" ? (
                              <Spinner aria-hidden="true" className="size-3 shrink-0 text-attn" />
                            ) : (
                              <TriangleAlert
                                aria-hidden="true"
                                className="size-3.5 shrink-0 text-attn"
                              />
                            )}
                            <span className="text-eyebrow font-medium text-attn">
                              {approval.state === "delivering"
                                ? "Delivering decision…"
                                : "Waiting for approval"}
                            </span>
                            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                            <span className="text-eyebrow text-faint">{approval.createdLabel}</span>
                          </div>
                          {approval.keyFingerprint ? (
                            <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5">
                              <KeyFingerprint layout="inline" value={approval.keyFingerprint} />
                              <span className="text-badge text-faint">
                                Approve only if the device shows this fingerprint
                              </span>
                            </div>
                          ) : null}
                        </div>
                      </div>
                      <div className="flex shrink-0 items-center gap-1.5">
                        <Button
                          disabled={approval.state === "delivering"}
                          onClick={() => onDenyPending(approval.id, approval.computerId)}
                          size="xs"
                          type="button"
                          variant="outline"
                        >
                          <X aria-hidden="true" />
                          Deny
                        </Button>
                        <Button
                          disabled={approval.state === "delivering"}
                          onClick={() => onApprovePending(approval.id, approval.computerId)}
                          size="xs"
                          type="button"
                          variant="default"
                        >
                          <Check aria-hidden="true" />
                          Approve
                        </Button>
                      </div>
                    </div>
                  </div>
                ))}

                {devices.length === 0 && localApprovals.length === 0 ? (
                  <p className="py-2 text-meta text-muted-foreground">
                    No other devices have connected to this computer yet.
                  </p>
                ) : (
                  devices.map(device => (
                    <LocalDeviceRow
                      device={device}
                      key={device.id}
                      onRequestRevokeDevice={onRequestRevokeDevice}
                      onToggleDeviceScope={onToggleDeviceScope}
                    />
                  ))
                )}
              </div>
            </Disclosure>
          </div>
        </>
      ) : null}
    </SettingsGroup>
  );
}
