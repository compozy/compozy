import { NativeSelect, NativeSelectOption, Switch } from "@compozy/ui";

import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsAdvancedFold } from "@/systems/settings/components/settings-advanced-fold";

import {
  LOAD_BALANCING_WEIGHT_OPTIONS,
  type ComputerFixture,
  type LoadBalancingWeight,
  type LocalComputerFixture,
} from "../mocks/remote-computers-types";
import { ComputerLoadMeter } from "./computer-load-meter";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface LoadBalancingSectionProps {
  local: LocalComputerFixture;
  computers: readonly ComputerFixture[];
  enabled: boolean;
  defaultOpen?: boolean;
  isPending?: boolean;
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
  const labelFor = (weight: LoadBalancingWeight) =>
    LOAD_BALANCING_WEIGHT_OPTIONS.find(option => option.value === weight)?.shortLabel ?? "normal";

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
 * Folded optional section shown only with at least 2 connected computers (including this computer
 * when local execution is on), carrying a live summary when collapsed and per-computer
 * `ComputerLoadMeter` + Prefer / Normal / Less often / Manual only selects inside.
 */
export function LoadBalancingSection({
  local,
  computers,
  enabled,
  defaultOpen = false,
  isPending = false,
  onToggleEnabled,
  onChangeLocalWeight,
  onChangeComputerWeight,
}: LoadBalancingSectionProps) {
  const connected = computers.filter(
    computer =>
      computer.enabled && (computer.status === "connected" || computer.status === "needs_attention")
  );
  const connectedTotal = connected.length + (local.localRunAgents ? 1 : 0);

  if (connectedTotal < 2 && !enabled) return null;

  const summary = foldedSummaryText(enabled, local, connected);

  return (
    <SettingsAdvancedFold
      data-testid="computers-load-balancing"
      defaultOpen={defaultOpen}
      label={
        <span className="flex flex-1 items-center justify-between gap-3">
          <span className="font-medium text-fg">Load balancing</span>
          <span
            className="text-meta font-normal text-muted-foreground"
            data-testid="computers-load-balancing-summary"
          >
            {summary}
          </span>
        </span>
      }
    >
      <div className="bg-card">
        <SettingRow
          control={
            <Switch
              aria-label="Enable load balancing"
              checked={enabled}
              disabled={isPending}
              onCheckedChange={checked => onToggleEnabled(checked)}
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
                  data-testid="load-balancing-weight-local"
                  disabled={!enabled || isPending}
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
                  data-testid={`load-balancing-weight-${computer.id}`}
                  disabled={!enabled || isPending}
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
  );
}
