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

import {
  DISPLAYED_REMOTE_SCOPES,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type LocalComputerFixture,
  type PendingApprovalFixture,
  type RemoteScope,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { KeyFingerprint } from "@/systems/computers/components/key-fingerprint";
import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

export interface ThisComputerSectionProps {
  account: AccountStateFixture;
  local: LocalComputerFixture;
  hasRemoteComputers: boolean;
  devices: readonly AccountDeviceFixture[];
  pendingApprovals: readonly PendingApprovalFixture[];
  defaultDevicesOpen?: boolean;
  onRequestToggleRunAgents: (next: boolean) => void;
  onToggleRemoteEnabled: (next: boolean) => void;
  onToggleHostedRoute: (next: boolean) => void;
  onToggleRequireApproval: (next: boolean) => void;
  onToggleDeviceScope: (deviceId: string, scope: RemoteScope, next: boolean) => void;
  onApprovePending: (approvalId: string) => void;
  onDenyPending: (approvalId: string) => void;
  onRequestRevokeDevice: (device: AccountDeviceFixture) => void;
  onUpgrade: () => void;
  onInspectHostedRoute?: () => void;
}

function HostedRouteSubRow({
  account,
  local,
  onToggleHostedRoute,
  onUpgrade,
  onInspectHostedRoute,
}: Pick<
  ThisComputerSectionProps,
  "account" | "local" | "onToggleHostedRoute" | "onUpgrade" | "onInspectHostedRoute"
>) {
  const isPro = account.plan === "pro";

  if (!isPro) {
    return (
      <SettingRow
        className="pl-8"
        control={
          <div className="flex items-center gap-2">
            <Pill size="xs" tone="brand">
              Part of Pro
            </Pill>
            <Button onClick={onUpgrade} size="xs" type="button" variant="secondary">
              <Sparkles aria-hidden="true" />
              Upgrade
            </Button>
          </div>
        }
        description="Hosted route is part of Pro. Reach this computer from any network or your phone."
        label="Hosted route"
      />
    );
  }

  let description = (
    <span>
      Reach this computer from any network or your phone ({account.cloudRoutesUsed} of{" "}
      {account.cloudRoutesLimit} hosted routes used).
    </span>
  );

  if (local.hostedRouteEnabled && local.hostedRouteState === "ready") {
    description = (
      <span className="inline-flex flex-wrap items-center gap-1.5">
        <Pill form="plain" size="xs" tone="success">
          <Pill.Dot />
          Hosted route ready
        </Pill>
        <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
        <span className="font-mono text-mono-id text-fg">{local.hostedUrl}</span>
        <CopyIconButton
          className="size-5 text-subtle hover:text-fg"
          copiedLabel="Hosted address copied"
          copiedToastLabel="Hosted address copied"
          copyLabel="Copy hosted route address"
          value={`https://${local.hostedUrl}`}
        />
      </span>
    );
  } else if (local.hostedRouteEnabled && local.hostedRouteState === "starting") {
    description = (
      <span className="inline-flex items-center gap-1.5 text-muted-foreground">
        <Spinner aria-hidden="true" className="size-3" />
        <span>Starting hosted route…</span>
      </span>
    );
  } else if (local.hostedRouteEnabled && local.hostedRouteState === "unreachable") {
    description = (
      <span className="flex flex-col gap-0.5">
        <span className="inline-flex items-center gap-1.5 text-warning">
          <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0" />
          <span className="truncate">
            {local.hostedRouteError ?? "Hosted route can't reach Cloudflare (port 7844 blocked)"}
          </span>
        </span>
        <span className="inline-flex items-center gap-2 text-muted-foreground">
          <span>Retrying automatically</span>
          {onInspectHostedRoute ? (
            <button
              className="cursor-pointer font-medium text-fg underline-offset-3 hover:underline focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={onInspectHostedRoute}
              type="button"
            >
              Details ›
            </button>
          ) : null}
        </span>
      </span>
    );
  }

  return (
    <SettingRow
      className="pl-8"
      control={
        <Switch
          aria-label="Hosted route"
          checked={local.hostedRouteEnabled}
          onCheckedChange={onToggleHostedRoute}
          size="sm"
        />
      }
      description={description}
      label="Hosted route"
    />
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
  const [scopesOpen, setScopesOpen] = useState(device.id === "dev_5k1p9z");
  const granted = new Set(device.scopesOnLocal);
  const grantedCount = DISPLAYED_REMOTE_SCOPES.filter(item => granted.has(item.scope)).length;

  return (
    <div className="border-t border-line-soft py-2.5 first:border-t-0">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <ComputerPlatformIcon deviceKind={device.kind} size="sm" tone="muted" />
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="truncate text-small-body font-medium text-fg">{device.name}</span>
              {device.thisDevice ? (
                <Pill size="xs" tone="neutral">
                  This device
                </Pill>
              ) : null}
              {device.admin ? (
                <Pill size="xs" tone="info">
                  <ShieldCheck aria-hidden="true" className="size-3" />
                  Trusted for admin
                </Pill>
              ) : null}
            </div>
            <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-eyebrow text-faint">
              <span>Last seen {device.lastSeenLabel}</span>
              <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
              <span>
                {grantedCount === DISPLAYED_REMOTE_SCOPES.length
                  ? "All permissions"
                  : `${grantedCount} of ${DISPLAYED_REMOTE_SCOPES.length} permissions`}
              </span>
            </div>
          </div>
        </div>

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
export function ThisComputerSection({
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
  onInspectHostedRoute,
}: ThisComputerSectionProps) {
  const localApprovals = pendingApprovals.filter(
    item => item.computerId === "local" && item.state === "pending"
  );
  const runAgentsDisabled = !hasRemoteComputers && local.localRunAgents;

  return (
    <SettingsGroup title={`This computer · ${local.name}`}>
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
            onInspectHostedRoute={onInspectHostedRoute}
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
            help="When off, any device signed in to your Compozy account can connect automatically."
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
                            <TriangleAlert
                              aria-hidden="true"
                              className="size-3.5 shrink-0 text-attn"
                            />
                            <span className="text-eyebrow font-medium text-attn">
                              Waiting for approval
                            </span>
                            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                            <span className="text-eyebrow text-faint">{approval.createdLabel}</span>
                          </div>
                          <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5">
                            <KeyFingerprint layout="inline" value={approval.keyFingerprint} />
                            <span className="text-badge text-faint">
                              Approve only if the device shows this fingerprint
                            </span>
                          </div>
                        </div>
                      </div>
                      <div className="flex shrink-0 items-center gap-1.5">
                        <Button
                          onClick={() => onDenyPending(approval.id)}
                          size="xs"
                          type="button"
                          variant="outline"
                        >
                          <X aria-hidden="true" />
                          Deny
                        </Button>
                        <Button
                          onClick={() => onApprovePending(approval.id)}
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

                {devices.map(device => (
                  <LocalDeviceRow
                    device={device}
                    key={device.id}
                    onRequestRevokeDevice={onRequestRevokeDevice}
                    onToggleDeviceScope={onToggleDeviceScope}
                  />
                ))}
              </div>
            </Disclosure>
          </div>
        </>
      ) : null}
    </SettingsGroup>
  );
}
