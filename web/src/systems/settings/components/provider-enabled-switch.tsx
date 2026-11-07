import { useId } from "react";

import { cn, Switch, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { isProviderEnabled } from "@/systems/providers";

import { useSetProviderEnabled } from "../hooks/use-settings-mutations";
import type { SettingsProviderEntry } from "../types";

const DEFAULT_PROVIDER_LOCK_COPY = "Choose another default provider before disabling this one";

interface ProviderEnabledSwitchProps {
  provider: SettingsProviderEntry;
  /** Show the "Enabled" caption beside the switch; otherwise it is the accessible name only. */
  showLabel?: boolean;
  className?: string;
  "data-testid"?: string;
}

/**
 * The provider's on/off switch. It flips as soon as it is pressed, reverts when the save fails
 * (with the reason underneath), and stays inactive on the default provider, which the daemon
 * refuses to disable.
 */
export function ProviderEnabledSwitch({
  provider,
  showLabel = false,
  className,
  "data-testid": testId = `settings-providers-enabled-${provider.name}`,
}: ProviderEnabledSwitchProps) {
  const mutation = useSetProviderEnabled();
  const errorId = useId();
  const saved = isProviderEnabled(provider.settings);
  const checked = mutation.isPending ? mutation.variables.enabled : saved;
  const locked = provider.default && saved;
  const toggle = (enabled: boolean) => mutation.mutate({ name: provider.name, enabled });

  const control = (
    <label className="flex items-center gap-2 text-meta text-muted-foreground">
      <span className={showLabel ? undefined : "sr-only"}>Enabled</span>
      <Switch
        aria-busy={mutation.isPending || undefined}
        aria-describedby={mutation.isError ? errorId : undefined}
        checked={checked}
        data-testid={testId}
        disabled={locked || mutation.isPending}
        onCheckedChange={toggle}
        size="sm"
      />
    </label>
  );

  return (
    <span className={cn("flex flex-col items-end gap-1", className)}>
      {locked ? (
        <Tooltip>
          <TooltipTrigger render={<span data-testid={`${testId}-locked`} tabIndex={0} />}>
            {control}
          </TooltipTrigger>
          <TooltipContent>{DEFAULT_PROVIDER_LOCK_COPY}</TooltipContent>
        </Tooltip>
      ) : (
        control
      )}
      {mutation.isError ? (
        <span className="text-meta text-destructive" data-testid={`${testId}-error`} id={errorId}>
          {mutation.error.message}
        </span>
      ) : null}
    </span>
  );
}
