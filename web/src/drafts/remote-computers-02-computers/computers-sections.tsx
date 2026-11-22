import { useEffect, useRef, useState } from "react";
import { Plus, Sparkles, Trash2 } from "@compozy/ui/icons";

import {
  Button,
  Checkbox,
  ConfirmDialog,
  ListingRow,
  NativeSelect,
  NativeSelectOption,
  StatusDot,
  Switch,
} from "@compozy/ui";

import {
  AccountRevokeDeviceDialog,
  type AccountRevokeDeviceDialogProps,
} from "@/systems/account/components/account-devices-section";
import {
  LOAD_BALANCING_WEIGHT_OPTIONS,
  type AccountStateFixture,
  type AccountUnaddedComputerFixture,
  type ComputerFixture,
  type LoadBalancingWeight,
  type LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ComputerLoadMeter } from "@/systems/computers/components/computer-load-meter";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsAdvancedFold } from "@/systems/settings/components/settings-advanced-fold";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

export interface AccountComputersSectionProps {
  account: AccountStateFixture;
  computers: readonly AccountUnaddedComputerFixture[];
  onAddAccountComputer: (computer: AccountUnaddedComputerFixture) => void;
  onUpgrade: () => void;
}

/** Block 3 of Settings → Computers (`_uiux.md` U1): account-linked computers not yet added here. */
export function AccountComputersSection({
  account,
  computers,
  onAddAccountComputer,
  onUpgrade,
}: AccountComputersSectionProps) {
  if (!account.signedIn || computers.length === 0) return null;

  return (
    <SettingsGroup
      description={`Linked to ${account.email} from another device or command.`}
      title="On your account"
    >
      {computers.map(computer => {
        const hostedOnlyBlocked = account.plan === "free" && computer.hostedOnly;
        return (
          <ListingRow
            className="min-h-14 py-3"
            data-account-computer-id={computer.id}
            interactive={false}
            key={computer.id}
          >
            <ListingRow.Icon>
              <ComputerPlatformIcon
                iconKind={computer.iconKind}
                platform={computer.platform}
                size="sm"
                tone="muted"
              />
            </ListingRow.Icon>
            <ListingRow.Main>
              <ListingRow.Name>
                <ListingRow.Title>{computer.name}</ListingRow.Title>
              </ListingRow.Name>
              <ListingRow.Meta className="mt-1">
                <span>Not added</span>
                <ListingRow.MetaDot />
                <span className="inline-flex items-center gap-1.5">
                  <StatusDot size="sm" tone={computer.online ? "success" : "neutral"} />
                  <span>{computer.online ? "Online" : "Offline"}</span>
                </span>
                {hostedOnlyBlocked ? (
                  <>
                    <ListingRow.MetaDot />
                    <span className="text-warning">
                      This computer can only be reached through the hosted route (Pro)
                    </span>
                  </>
                ) : null}
              </ListingRow.Meta>
            </ListingRow.Main>
            <ListingRow.Trail>
              {hostedOnlyBlocked ? (
                <Button onClick={onUpgrade} size="sm" type="button" variant="secondary">
                  <Sparkles aria-hidden="true" />
                  Upgrade
                </Button>
              ) : (
                <Button
                  onClick={() => onAddAccountComputer(computer)}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  <Plus aria-hidden="true" />
                  Add
                </Button>
              )}
            </ListingRow.Trail>
          </ListingRow>
        );
      })}
    </SettingsGroup>
  );
}

export interface LoadBalancingSectionProps {
  local: LocalComputerFixture;
  computers: readonly ComputerFixture[];
  enabled: boolean;
  defaultOpen?: boolean;
  onToggleEnabled: (next: boolean) => void;
  onChangeLocalWeight: (weight: LoadBalancingWeight) => void;
  onChangeComputerWeight: (computerId: string, weight: LoadBalancingWeight) => void;
}

function foldedSummaryText(
  enabled: boolean,
  local: LocalComputerFixture,
  computers: readonly ComputerFixture[]
): string {
  if (!enabled) return "Off";
  const overrides: string[] = [];
  const labelFor = (w: LoadBalancingWeight) =>
    LOAD_BALANCING_WEIGHT_OPTIONS.find(option => option.value === w)?.shortLabel ?? "normal";

  if (local.localRunAgents && local.loadBalancingWeight !== 50) {
    overrides.push(`${local.name} ${labelFor(local.loadBalancingWeight)}`);
  }
  for (const computer of computers) {
    if (computer.loadBalancingWeight !== 50) {
      overrides.push(`${computer.name} ${labelFor(computer.loadBalancingWeight)}`);
    }
  }
  return overrides.length > 0 ? overrides.slice(0, 3).join(" · ") : "On · balanced";
}

/**
 * Block 4 of Settings → Computers (`_uiux.md` U1, US-015):
 * Folded optional section shown only with >= 2 connected computers, carrying
 * a live summary string when collapsed and per-computer `ComputerLoadMeter` + weight select inside.
 */
export function LoadBalancingSection({
  local,
  computers,
  enabled,
  defaultOpen = false,
  onToggleEnabled,
  onChangeLocalWeight,
  onChangeComputerWeight,
}: LoadBalancingSectionProps) {
  const sectionRef = useRef<HTMLDivElement | null>(null);
  const connected = computers.filter(
    computer => computer.status === "connected" || computer.status === "needs_attention"
  );

  useEffect(() => {
    if (!defaultOpen) return;
    sectionRef.current?.scrollIntoView({ block: "start", behavior: "instant" });
  }, [defaultOpen]);

  if (connected.length < 2) return null;

  const summary = foldedSummaryText(enabled, local, computers);

  return (
    <div ref={sectionRef}>
      <SettingsAdvancedFold
        data-testid="computers-load-balancing"
        defaultOpen={defaultOpen}
        label={
          <span className="flex flex-1 items-center justify-between gap-3">
            <span className="font-medium text-fg">Load balancing</span>
            <span className="text-meta font-normal text-muted-foreground">{summary}</span>
          </span>
        }
      >
        <div className="bg-card">
          <SettingRow
            control={
              <Switch
                aria-label="Enable load balancing"
                checked={enabled}
                onCheckedChange={onToggleEnabled}
                size="sm"
              />
            }
            description="Adds Auto to Run on so new sessions start on the computer with the most free CPU and memory."
            label="Enable load balancing"
          />

          {local.localRunAgents ? (
            <SettingRow
              control={
                <div className="flex items-center gap-3">
                  <ComputerLoadMeter computerName={local.name} load={local.load} />
                  <NativeSelect
                    aria-label={`Load balancing preference for ${local.name}`}
                    disabled={!enabled}
                    onChange={event =>
                      onChangeLocalWeight(Number(event.target.value) as LoadBalancingWeight)
                    }
                    size="sm"
                    value={String(local.loadBalancingWeight)}
                  >
                    {LOAD_BALANCING_WEIGHT_OPTIONS.map(option => (
                      <NativeSelectOption key={option.value} value={String(option.value)}>
                        {option.label}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </div>
              }
              description="This computer"
              label={
                <span className="inline-flex items-center gap-2">
                  <ComputerPlatformIcon
                    iconKind={local.iconKind}
                    platform={local.platform}
                    size="xs"
                    tone="muted"
                  />
                  <span>{local.name}</span>
                </span>
              }
            />
          ) : null}

          {connected.map(computer => (
            <SettingRow
              control={
                <div className="flex items-center gap-3">
                  <ComputerLoadMeter computerName={computer.name} load={computer.load} />
                  <NativeSelect
                    aria-label={`Load balancing preference for ${computer.name}`}
                    disabled={!enabled}
                    onChange={event =>
                      onChangeComputerWeight(
                        computer.id,
                        Number(event.target.value) as LoadBalancingWeight
                      )
                    }
                    size="sm"
                    value={String(computer.loadBalancingWeight)}
                  >
                    {LOAD_BALANCING_WEIGHT_OPTIONS.map(option => (
                      <NativeSelectOption key={option.value} value={String(option.value)}>
                        {option.label}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </div>
              }
              description={computer.routeInUse?.label ?? "Connected"}
              key={computer.id}
              label={
                <span className="inline-flex items-center gap-2">
                  <ComputerPlatformIcon
                    iconKind={computer.iconKind}
                    platform={computer.platform}
                    size="xs"
                    tone="muted"
                  />
                  <span>{computer.name}</span>
                </span>
              }
            />
          ))}
        </div>
      </SettingsAdvancedFold>
    </div>
  );
}

export interface RemoveComputerDialogProps {
  computer: ComputerFixture | null;
  isFreePlan: boolean;
  onClose: () => void;
  onConfirm: (computer: ComputerFixture, options: { unlink: boolean; uninstall: boolean }) => void;
}

export function RemoveComputerDialog({
  computer,
  isFreePlan,
  onClose,
  onConfirm,
}: RemoveComputerDialogProps) {
  const [unlink, setUnlink] = useState(isFreePlan);
  const [uninstall, setUninstall] = useState(false);

  if (!computer) return null;

  return (
    <ConfirmDialog
      body={
        <div className="flex flex-col gap-3">
          <label
            className="flex cursor-pointer items-start gap-2.5"
            htmlFor="remove-computer-unlink"
          >
            <Checkbox
              checked={unlink}
              className="mt-0.5"
              id="remove-computer-unlink"
              onCheckedChange={next => setUnlink(Boolean(next))}
            />
            <span className="flex flex-col gap-0.5">
              <span className="text-small-body font-medium text-fg">
                Also unlink from my account
              </span>
              <span className="text-meta text-muted-foreground">
                {isFreePlan
                  ? "Frees your free computer slot immediately."
                  : "Removes this computer from On your account and deletes its hosted route."}
              </span>
            </span>
          </label>

          {computer.originKind === "ssh" && computer.sshTarget ? (
            <label
              className="flex cursor-pointer items-start gap-2.5"
              htmlFor="remove-computer-uninstall"
            >
              <Checkbox
                checked={uninstall}
                className="mt-0.5"
                id="remove-computer-uninstall"
                onCheckedChange={next => setUninstall(Boolean(next))}
              />
              <span className="flex flex-col gap-0.5">
                <span className="text-small-body font-medium text-fg">
                  Also uninstall Compozy from{" "}
                  <span className="font-mono text-mono-id">{computer.sshTarget}</span>
                </span>
                <span className="text-meta text-muted-foreground">
                  Stops the remote Compozy service and removes its binary from the host.
                </span>
              </span>
            </label>
          ) : null}
        </div>
      }
      cancelLabel="Cancel"
      confirmLabel="Remove"
      description={`Agents already running on ${computer.name} keep running. This app forgets its routes, credentials, and session list.`}
      icon={Trash2}
      onConfirm={() => {
        onConfirm(computer, { unlink, uninstall });
      }}
      onOpenChange={open => {
        if (!open) onClose();
      }}
      open
      title={`Remove ${computer.name}?`}
      tone="destructive"
    />
  );
}

export type RevokeDeviceDialogProps = AccountRevokeDeviceDialogProps;

export function RevokeDeviceDialog(props: RevokeDeviceDialogProps) {
  return <AccountRevokeDeviceDialog {...props} />;
}

export interface DisableLocalAgentsDialogProps {
  open: boolean;
  local: LocalComputerFixture;
  onClose: () => void;
  onConfirm: () => void;
}

export function DisableLocalAgentsDialog({
  open,
  local,
  onClose,
  onConfirm,
}: DisableLocalAgentsDialogProps) {
  if (!open) return null;
  const count = local.activeLocalSessions;
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmLabel="Stop running agents"
      description={`${count} ${count === 1 ? "session is" : "sessions are"} running here. They keep running until they finish; new sessions go to your other computers.`}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open
      title="Stop running agents on this computer?"
      tone="warning"
    />
  );
}
