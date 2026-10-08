import { useId, type ComponentProps } from "react";

import { Switch, cn } from "@compozy/ui";

interface AutomationEnableSwitchProps extends Omit<ComponentProps<"div">, "children"> {
  enabled: boolean;
  pending: boolean;
  onEnabledChange: (enabled: boolean) => void;
  labelTestId: string;
  switchTestId: string;
}

/**
 * The labeled enable switch shown in job and trigger detail heads.
 *
 * While the PATCH is in flight the track keeps the state the daemon last
 * confirmed; the label announces the transition instead. An optimistic flip
 * would claim a state the runtime has not agreed to yet.
 */
export function AutomationEnableSwitch({
  enabled,
  pending,
  onEnabledChange,
  labelTestId,
  switchTestId,
  className,
  ...props
}: AutomationEnableSwitchProps) {
  const labelId = useId();
  const label = pending ? (enabled ? "Disabling…" : "Enabling…") : enabled ? "Enabled" : "Disabled";
  return (
    <div
      aria-busy={pending || undefined}
      className={cn(
        "mt-0.5 flex shrink-0 items-center gap-2 rounded-md px-1.5 py-1",
        pending && "pointer-events-none opacity-55",
        className
      )}
      {...props}
    >
      <span
        className={cn(
          "text-form-label font-medium transition-colors duration-fast ease-out",
          enabled ? "text-fg" : "text-muted"
        )}
        data-testid={labelTestId}
        id={labelId}
      >
        {label}
      </span>
      <Switch
        aria-labelledby={labelId}
        checked={enabled}
        data-testid={switchTestId}
        disabled={pending}
        onCheckedChange={next => {
          if (!pending) onEnabledChange(next);
        }}
      />
    </div>
  );
}

interface AutomationRowSwitchProps extends Omit<
  ComponentProps<typeof Switch>,
  "checked" | "onCheckedChange"
> {
  name: string;
  enabled: boolean;
  pending: boolean;
  onEnabledChange: (enabled: boolean) => void;
}

/**
 * Unlabeled row variant: dimmed and inert while the PATCH is in flight, keeping
 * the state the daemon last confirmed until it answers.
 */
export function AutomationRowSwitch({
  name,
  enabled,
  pending,
  disabled,
  onEnabledChange,
  className,
  ...props
}: AutomationRowSwitchProps) {
  return (
    <Switch
      aria-busy={pending || undefined}
      aria-label={`Turn ${name} on or off`}
      checked={enabled}
      className={cn(pending && "opacity-55", className)}
      disabled={disabled || pending}
      onCheckedChange={next => {
        if (!pending) onEnabledChange(next);
      }}
      {...props}
    />
  );
}
