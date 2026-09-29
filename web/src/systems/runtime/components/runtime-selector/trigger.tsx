import { ChevronDown, TriangleAlert, Zap } from "lucide-react";
import { useId, type ComponentProps } from "react";

import { type RuntimeSpeed } from "@/lib/api-contract";
import { cn, IntensityMeter, KindIcon, providerKindIconRegistry } from "@compozy/ui";

import { runtimeTriggerView } from "./trigger-model";
import {
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
  type RuntimeSelectorVariant,
} from "./types";

export interface RuntimeSelectorTriggerProps extends Omit<
  ComponentProps<"button">,
  "value" | "onClick"
> {
  value: RuntimeSelectorValue;
  provider: RuntimeProviderOption | undefined;
  model: RuntimeModelOption | undefined;
  variant?: RuntimeSelectorVariant;
  open?: boolean;
  needsAuth?: boolean;
  readOnly?: boolean;
  modelPlaceholder?: string;
  /** Session-level ACP speed request; "fast" adds the bolt mark (undefined = unwired). */
  speed?: RuntimeSpeed;
  /** id of the popup dialog the trigger controls (announced via aria-controls). */
  popupId?: string;
  /** id of the surface's visible field caption; names the trigger when set. */
  ariaLabelledby?: string;
  onPress?: () => void;
}

/** Keeps the composer trigger chromeless inside the prompt frame. */
function triggerSurfaceClass(
  variant: RuntimeSelectorVariant,
  open: boolean,
  inert: boolean
): string {
  if (variant === "composer") {
    return cn(
      "group h-button-lg gap-2 border-0 bg-transparent px-2 shadow-none",
      "hover:bg-transparent focus-visible:shadow-focus-ring",
      inert && "cursor-not-allowed opacity-60"
    );
  }
  return cn(
    "bg-elevated shadow-highlight hover:bg-btn-default-hover focus-visible:ring-2 focus-visible:ring-accent",
    variant === "small" ? "h-button-lg gap-2 px-2.5" : "h-[34px] gap-2.5 px-3",
    open ? "border-accent-dim" : "border-line-strong",
    inert && "cursor-not-allowed opacity-60 hover:bg-transparent"
  );
}

function TriggerModelName({
  name,
  variant,
  inert,
}: {
  name: string;
  variant: RuntimeSelectorVariant;
  inert: boolean;
}) {
  const composer = variant === "composer";
  return (
    <span
      className={cn(
        "max-w-[150px] truncate text-small-body font-medium",
        composer
          ? "text-subtle transition-colors group-data-[open=true]:text-fg-strong"
          : "text-fg-strong",
        composer && !inert && "group-hover:text-fg-strong"
      )}
    >
      {name}
    </span>
  );
}

function TriggerFastMark() {
  return (
    <span
      title="Fast speed requested"
      data-slot="runtime-selector-fast"
      className="grid shrink-0 place-items-center text-accent-strong"
    >
      <Zap aria-hidden="true" className="size-[11px] fill-current" />
    </span>
  );
}

function TriggerWarningMark({ label }: { label: string }) {
  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className="grid shrink-0 place-items-center text-warning"
    >
      <TriangleAlert aria-hidden="true" className="size-3.5" />
    </span>
  );
}

/**
 * One click surface: the whole closed selector is a single button that opens
 * (or closes) the popup — no per-segment deep links, no divider chrome. It
 * shows only the provider mark, the model name, and the reasoning meter; the
 * words (provider name, effort label) live in the popup and the aria summary.
 */
export function RuntimeSelectorTrigger({
  value,
  provider,
  model,
  variant = "default",
  open = false,
  needsAuth = false,
  disabled = false,
  readOnly = false,
  modelPlaceholder = "Select model",
  speed,
  popupId,
  ariaLabelledby,
  onPress,
  className,
  ref,
  ...props
}: RuntimeSelectorTriggerProps) {
  const inert = disabled || readOnly;
  const view = runtimeTriggerView({
    value,
    provider,
    model,
    variant,
    needsAuth,
    modelPlaceholder,
    speed,
  });

  // With an external caption the accessible name composes caption + value
  // (label-then-value, like a native select); standalone, the summary is the
  // whole name.
  const valueSummaryId = useId();

  return (
    <button
      ref={ref}
      type="button"
      disabled={disabled}
      aria-disabled={readOnly || undefined}
      aria-haspopup="dialog"
      aria-expanded={open}
      aria-controls={open ? popupId : undefined}
      aria-labelledby={ariaLabelledby ? `${ariaLabelledby} ${valueSummaryId}` : undefined}
      aria-label={ariaLabelledby ? undefined : `Runtime: ${view.valueSummary}`}
      data-open={open ? "true" : "false"}
      data-variant={variant}
      className={cn(
        "inline-flex select-none items-center rounded-md border outline-none transition-colors",
        triggerSurfaceClass(variant, open, inert),
        className
      )}
      onClick={() => {
        if (!inert) onPress?.();
      }}
      {...props}
    >
      <span id={valueSummaryId} className="sr-only">
        {view.valueSummary}
      </span>
      <KindIcon
        kind={view.providerKind}
        registry={providerKindIconRegistry}
        size="sm"
        tone="default"
        className="shrink-0"
      />
      {view.compact ? null : (
        <TriggerModelName name={view.modelName} variant={variant} inert={inert} />
      )}
      {view.meter ? (
        <IntensityMeter
          position={view.meter.position}
          hollow={view.meter.hollow}
          className="shrink-0"
        />
      ) : null}
      {view.showFast ? <TriggerFastMark /> : null}
      {view.warningLabel ? <TriggerWarningMark label={view.warningLabel} /> : null}
      <ChevronDown
        aria-hidden="true"
        data-slot="runtime-selector-chevron"
        className={cn(
          "size-3.5 shrink-0 text-faint transition-[transform,color]",
          variant === "composer" && !inert && "group-hover:text-fg",
          open && "rotate-180 text-fg"
        )}
      />
    </button>
  );
}
