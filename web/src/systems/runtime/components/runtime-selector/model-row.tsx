import { Brain, Check, Star } from "lucide-react";

import { cn, KindIcon, providerKindIconRegistry } from "@agh/ui";

import type { RuntimeModelOption } from "./types";

export interface ModelRowProps {
  /** DOM id used for the combobox `aria-activedescendant` relationship. */
  id: string;
  model: RuntimeModelOption;
  /** Provider display name — spoken (sr-only) so same-id rows stay distinct. */
  providerName: string;
  /** Icon key from the owning provider option (`runtime_provider` or id). */
  iconKind: string;
  selected: boolean;
  favorite: boolean;
  highlighted: boolean;
  onSelect: (provider: string, id: string) => void;
  /** Pointer hover makes this the active row (so the external favorite action targets it). */
  onHover: () => void;
}

/**
 * One `role="option"` in the models listbox — a single line: bare provider
 * mark, model name, and a faint brain glyph when the model reasons. Context,
 * cost, and tool metadata deliberately do not render here; the catalog is a
 * picker, not a spec sheet. A listbox option MUST NOT wrap a focusable or
 * interactive control, so the row stays pure: selection is its only action.
 * The favorite star is a NON-interactive `aria-hidden` indicator — the real
 * favorite control is the footer button (+ Alt+F) acting on the active row,
 * and pointer hover activates the row so that control targets whatever the
 * cursor is over.
 */
export function ModelRow({
  id,
  model,
  providerName,
  iconKind,
  selected,
  favorite,
  highlighted,
  onSelect,
  onHover,
}: ModelRowProps) {
  const disabled = Boolean(model.disabled);
  const reasons = model.efforts.length > 0 || Boolean(model.supports_reasoning);

  return (
    <div
      role="option"
      id={id}
      aria-selected={selected}
      aria-disabled={disabled || undefined}
      tabIndex={-1}
      data-provider={model.provider}
      data-model={model.id}
      data-selected={selected ? "true" : "false"}
      data-disabled={disabled ? "true" : "false"}
      data-highlighted={highlighted ? "true" : "false"}
      data-favorite={favorite ? "true" : "false"}
      className={cn(
        "group flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors",
        disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer hover:bg-row-hover",
        highlighted && !disabled && "bg-row-hover ring-1 ring-line-strong ring-inset",
        selected && "bg-row-selected"
      )}
      onMouseEnter={disabled ? undefined : onHover}
      onClick={event => {
        if (disabled) return;
        event.preventDefault();
        onSelect(model.provider, model.id);
      }}
    >
      <KindIcon
        kind={iconKind}
        registry={providerKindIconRegistry}
        size="md"
        tone="default"
        className="shrink-0"
      />
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <span className="truncate text-small-body font-medium text-fg-strong">{model.name}</span>
        <span className="sr-only">from {providerName}</span>
        {reasons ? (
          <span
            data-reasoning-indicator="true"
            title="Supports reasoning"
            className="grid shrink-0 place-items-center text-faint"
          >
            <Brain aria-hidden="true" className="size-3.5" />
            <span className="sr-only">, supports reasoning</span>
          </span>
        ) : null}
        {favorite ? <span className="sr-only">, favorited</span> : null}
      </span>
      <span className="flex shrink-0 items-center gap-2.5">
        {disabled ? (
          <span className="text-badge font-medium whitespace-nowrap text-warning">
            {model.disabled_reason ?? "Unavailable"}
          </span>
        ) : null}
        {/* Non-color structural cue for selection (in addition to aria-selected + row tint). */}
        {selected ? (
          <Check
            aria-hidden="true"
            className="size-3.5 shrink-0 text-accent-strong"
            data-selected-check="true"
          />
        ) : null}
        {/* Decorative favorite-state indicator only — never interactive. The real
            favorite control is the footer button + Alt+F acting on the active row. */}
        <span
          aria-hidden="true"
          data-favorite-indicator={favorite ? "true" : "false"}
          className={cn(
            "pointer-events-none grid size-5 place-items-center rounded text-faint transition-opacity",
            favorite ? "text-warning opacity-100" : "opacity-0"
          )}
        >
          <Star aria-hidden="true" className={cn("size-3.5", favorite && "fill-current")} />
        </span>
      </span>
    </div>
  );
}
