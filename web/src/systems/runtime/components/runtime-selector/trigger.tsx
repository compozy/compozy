import { ChevronDown, TriangleAlert } from "lucide-react";
import { useId, type ComponentProps } from "react";

import { cn, IntensityMeter, KindIcon, providerKindIconRegistry } from "@agh/ui";

import {
  reasoningEffortLabel,
  reasoningEffortPosition,
  resolveReasoningState,
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
  /** id of the popup dialog the trigger controls (announced via aria-controls). */
  popupId?: string;
  /** id of the surface's visible field caption; names the trigger when set. */
  ariaLabelledby?: string;
  onPress?: () => void;
}

function providerKind(
  provider: RuntimeProviderOption | undefined,
  value: RuntimeSelectorValue
): string {
  return provider?.runtime_provider ?? provider?.id ?? value.provider;
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
  popupId,
  ariaLabelledby,
  onPress,
  className,
  ref,
  ...props
}: RuntimeSelectorTriggerProps) {
  const compact = variant === "compact";
  const inert = disabled || readOnly;
  const reasoning = resolveReasoningState(model);
  const showReasoning = reasoning.mode === "levels" && !compact;
  // The meter mirrors the slider: the model default fills the bars while the
  // wire value is ""; only a level-less model renders the hollow zero state.
  const currentEffort = value.reasoning_effort || reasoning.defaultEffort;
  const meterUnset = currentEffort === "";
  const showWarning = needsAuth || model?.availability === "unavailable";
  const warningLabel = needsAuth ? "Provider needs sign in" : "Model unavailable";
  const providerName = provider?.name || value.provider;
  // `||` not `??`: an unset model is "" (not nullish), so the placeholder must
  // still win — otherwise the trigger renders blank in the no-model state.
  const modelName = model?.name || value.model || modelPlaceholder;

  const summaryParts = [compact ? providerName : `${providerName} / ${modelName}`];
  if (showReasoning) {
    summaryParts.push(
      `reasoning ${meterUnset ? "provider default" : reasoningEffortLabel(currentEffort)}`
    );
  }
  if (showWarning) summaryParts.push(warningLabel.toLowerCase());
  const valueSummary = summaryParts.join(", ");

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
      aria-label={ariaLabelledby ? undefined : `Runtime: ${valueSummary}`}
      data-open={open ? "true" : "false"}
      data-variant={variant}
      className={cn(
        "inline-flex select-none items-center rounded-md border outline-none transition-colors",
        triggerSurfaceClass(variant, open, inert),
        className
      )}
      onClick={() => {
        if (!disabled && !readOnly) onPress?.();
      }}
      {...props}
    >
      <span id={valueSummaryId} className="sr-only">
        {valueSummary}
      </span>
      <KindIcon
        kind={providerKind(provider, value)}
        registry={providerKindIconRegistry}
        size="sm"
        tone="default"
        className="shrink-0"
      />
      {compact ? null : (
        <span
          className={cn(
            "max-w-[150px] truncate text-small-body font-medium",
            variant === "composer"
              ? "text-subtle transition-colors group-data-[open=true]:text-fg-strong"
              : "text-fg-strong",
            variant === "composer" && !inert && "group-hover:text-fg-strong"
          )}
        >
          {modelName}
        </span>
      )}
      {showReasoning ? (
        <IntensityMeter
          position={meterUnset ? 0 : reasoningEffortPosition(currentEffort)}
          hollow={meterUnset}
          className="shrink-0"
        />
      ) : null}
      {showWarning ? (
        <span
          role="img"
          aria-label={warningLabel}
          title={warningLabel}
          className="grid shrink-0 place-items-center text-warning"
        >
          <TriangleAlert aria-hidden="true" className="size-3.5" />
        </span>
      ) : null}
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
