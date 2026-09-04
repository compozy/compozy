import { Brain, Check, Star } from "lucide-react";

import {
  cn,
  KindIcon,
  providerKindIconRegistry,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import type { RuntimeModelOption } from "./types";

export interface ModelRowProps {
  /** DOM id used for the combobox `aria-activedescendant` relationship. */
  id: string;
  model: RuntimeModelOption;
  /** Provider display name — spoken (sr-only) so same-id rows stay distinct. */
  providerName: string;
  /** Icon key from the owning provider option (`runtime_provider` or id). */
  iconKind: string;
  /**
   * Cross-provider sections (pinned recents/favorites) show the provider mark;
   * provider-grouped rows drop it — the group head already carries identity.
   */
  showGlyph: boolean;
  selected: boolean;
  favorite: boolean;
  highlighted: boolean;
  onSelect: (provider: string, id: string) => void;
  /** Pointer hover makes this the active row (so Alt+F targets it). */
  onHover: () => void;
  /** Pointer path for the row's favorite star (keyboard path is Alt+F). */
  onToggleFavorite: () => void;
}

/**
 * One `role="option"` in the models listbox — a compact 28px line (design ref:
 * runtime-selector-variations.html, variation A): name, a faint brain glyph
 * when the model reasons, and a favorite star on the trailing edge. A listbox
 * option MUST NOT wrap a focusable control, so the star is `aria-hidden` and
 * never enters the Tab order: pointer clicks are intercepted before row
 * selection, while keyboard/AT users favorite the active row with Alt+F
 * (announced by the popup's polite status region). Its hit area stays ≥24px.
 */
export function ModelRow({
  id,
  model,
  providerName,
  iconKind,
  showGlyph,
  selected,
  favorite,
  highlighted,
  onSelect,
  onHover,
  onToggleFavorite,
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
        "group flex h-7 w-full items-center gap-2 rounded-md pr-0.5 pl-2 text-left transition-colors",
        disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer hover:bg-row-hover",
        highlighted && !disabled && "bg-row-hover ring-1 ring-line-strong ring-inset",
        selected && "bg-accent-tint"
      )}
      onMouseEnter={disabled ? undefined : onHover}
      onClick={event => {
        if (disabled) return;
        event.preventDefault();
        onSelect(model.provider, model.id);
      }}
    >
      {showGlyph ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <KindIcon
                kind={iconKind}
                registry={providerKindIconRegistry}
                size="sm"
                tone="default"
                className="shrink-0"
              />
            }
          />
          <TooltipContent>{providerName}</TooltipContent>
        </Tooltip>
      ) : null}
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <span
          className={cn(
            "truncate text-small-body font-medium",
            selected ? "text-fg-strong" : "text-fg"
          )}
        >
          {model.name}
        </span>
        <span className="sr-only">from {providerName}</span>
        {reasons ? (
          <span
            data-reasoning-indicator="true"
            title="Supports reasoning"
            className="grid shrink-0 place-items-center text-faint"
          >
            <Brain aria-hidden="true" className="size-3" />
            <span className="sr-only">, supports reasoning</span>
          </span>
        ) : null}
        {favorite ? <span className="sr-only">, favorited</span> : null}
      </span>
      <span className="flex shrink-0 items-center gap-1.5">
        {disabled ? (
          <span
            className="text-badge font-medium whitespace-nowrap text-warning"
            {...(model.disabled_detail ? { title: model.disabled_detail } : {})}
          >
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
        {/* Pointer-only favorite affordance: aria-hidden (never focusable, never
            in the a11y tree) with a 24px hit target; the keyboard/AT path is the
            Alt+F shortcut acting on the active row. */}
        <span
          aria-hidden="true"
          data-favorite-indicator={favorite ? "true" : "false"}
          className={cn(
            "grid size-6 shrink-0 place-items-center rounded transition-opacity",
            favorite
              ? "text-warning opacity-100"
              : "text-faint opacity-0 hover:text-fg-strong group-hover:opacity-100 group-data-[highlighted=true]:opacity-100 [@media(pointer:coarse)]:opacity-100"
          )}
          onClick={event => {
            if (disabled) return;
            event.stopPropagation();
            event.preventDefault();
            onToggleFavorite();
          }}
        >
          <Star aria-hidden="true" className={cn("size-3.5", favorite && "fill-current")} />
        </span>
      </span>
    </div>
  );
}
