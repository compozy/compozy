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

/** Boolean state rendered as an explicit `"true"`/`"false"` data attribute. */
function boolAttr(value: boolean): "true" | "false" {
  return value ? "true" : "false";
}

function modelRowClass(disabled: boolean, highlighted: boolean, selected: boolean): string {
  return cn(
    "group flex h-7 w-full items-center gap-2 rounded-md pr-0.5 pl-2 text-left transition-colors",
    disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer hover:bg-surface-2",
    highlighted && !disabled && "bg-surface-2 ring-1 ring-line-strong ring-inset",
    selected && "bg-selected"
  );
}

function modelReasons(model: RuntimeModelOption): boolean {
  return model.efforts.length > 0 || Boolean(model.supports_reasoning);
}

function ProviderGlyph({ iconKind, providerName }: { iconKind: string; providerName: string }) {
  return (
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
  );
}

function ModelRowLabel({
  model,
  providerName,
  selected,
  favorite,
}: Pick<ModelRowProps, "model" | "providerName" | "selected" | "favorite">) {
  return (
    <span className="flex min-w-0 flex-1 items-center gap-2">
      <span
        className={cn(
          "truncate text-small-body font-medium",
          selected ? "font-semibold text-fg" : "text-fg"
        )}
      >
        {model.name}
      </span>
      <span className="sr-only">from {providerName}</span>
      {modelReasons(model) ? (
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
  );
}

function ModelRowTrailing({
  model,
  disabled,
  selected,
  favorite,
  onToggleFavorite,
}: Pick<ModelRowProps, "model" | "selected" | "favorite" | "onToggleFavorite"> & {
  disabled: boolean;
}) {
  return (
    <span className="flex shrink-0 items-center gap-1.5">
      {disabled ? (
        <span className="text-badge font-medium whitespace-nowrap text-warning">
          {model.disabled_reason ?? "Unavailable"}
        </span>
      ) : null}
      {/* Non-color structural cue for selection (in addition to aria-selected + row tint). */}
      {selected ? (
        <Check
          aria-hidden="true"
          className="size-3.5 shrink-0 text-fg"
          data-selected-check="true"
        />
      ) : null}
      {/* Pointer-only favorite affordance: aria-hidden (never focusable, never
          in the a11y tree) with a 24px hit target; the keyboard/AT path is the
          Alt+F shortcut acting on the active row. */}
      <span
        aria-hidden="true"
        data-favorite-indicator={boolAttr(favorite)}
        className={cn(
          "grid size-6 shrink-0 place-items-center rounded transition-opacity",
          favorite
            ? "text-warning opacity-100"
            : "text-faint opacity-0 hover:text-fg group-hover:opacity-100 group-data-[highlighted=true]:opacity-100 [@media(pointer:coarse)]:opacity-100"
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
  );
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

  return (
    <div
      role="option"
      id={id}
      aria-selected={selected}
      aria-disabled={disabled || undefined}
      tabIndex={-1}
      data-provider={model.provider}
      data-model={model.id}
      data-selected={boolAttr(selected)}
      data-disabled={boolAttr(disabled)}
      data-highlighted={boolAttr(highlighted)}
      data-favorite={boolAttr(favorite)}
      className={modelRowClass(disabled, highlighted, selected)}
      onMouseEnter={disabled ? undefined : onHover}
      onClick={event => {
        if (disabled) return;
        event.preventDefault();
        onSelect(model.provider, model.id);
      }}
    >
      {showGlyph ? <ProviderGlyph iconKind={iconKind} providerName={providerName} /> : null}
      <ModelRowLabel
        favorite={favorite}
        model={model}
        providerName={providerName}
        selected={selected}
      />
      <ModelRowTrailing
        disabled={disabled}
        favorite={favorite}
        model={model}
        onToggleFavorite={onToggleFavorite}
        selected={selected}
      />
    </div>
  );
}
