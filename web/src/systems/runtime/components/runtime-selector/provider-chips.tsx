import { LayoutGrid, Settings, Star } from "lucide-react";
import { useRef, type KeyboardEvent, type ReactNode } from "react";

import {
  cn,
  KindIcon,
  providerKindIconRegistry,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import type { RailFilter } from "./use-runtime-selector";
import type { RuntimeProviderOption } from "./types";

const CHIP_CLASS =
  "grid size-6 shrink-0 place-items-center rounded-sm text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg focus-visible:bg-row-hover focus-visible:text-fg-strong focus-visible:ring-2 focus-visible:ring-accent disabled:pointer-events-none data-[active=true]:bg-row-selected data-[active=true]:text-fg-strong data-[active=true]:ring-1 data-[active=true]:ring-line-strong data-[active=true]:ring-inset";

function getChipOrder(providers: RuntimeProviderOption[]): RailFilter[] {
  return ["all", "fav", ...providers.map(provider => provider.id)];
}

function focusChip(
  group: HTMLDivElement | null,
  target: RailFilter,
  onRail: (target: RailFilter) => void
) {
  onRail(target);
  const chip = group?.querySelector<HTMLButtonElement>(
    `[data-rail="${CSS.escape(String(target))}"]`
  );
  chip?.focus();
}

export interface ProviderChipsProps {
  providers: RuntimeProviderOption[];
  railFilter: RailFilter;
  searching: boolean;
  onRail: (target: RailFilter) => void;
  onOpenSettings?: () => void;
}

/**
 * Horizontal provider filter (design ref: runtime-selector-variations.html,
 * variation A — the 52px vertical rail collapsed into one 32px chip line).
 * Same semantics as the old rail: a `radiogroup`, NOT tabs — chips filter the
 * list in place. Roving tabindex + arrow keys move and select in one step (the
 * WAI-ARIA radio pattern); a non-empty search disables the whole group because
 * search always spans every provider. Provider Settings sits OUTSIDE the group
 * at the far end: it is an escape hatch, not a filter.
 */
export function ProviderChips({
  providers,
  railFilter,
  searching,
  onRail,
  onOpenSettings,
}: ProviderChipsProps) {
  const groupRef = useRef<HTMLDivElement>(null);

  const moveFocus = (current: RailFilter, direction: 1 | -1) => {
    const order = getChipOrder(providers);
    const position = order.indexOf(current);
    const start = position < 0 ? 0 : position;
    const next = order[(start + direction + order.length) % order.length];
    focusChip(groupRef.current, next, onRail);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (searching) return;
    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      event.preventDefault();
      moveFocus(railFilter, 1);
    } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      event.preventDefault();
      moveFocus(railFilter, -1);
    } else if (event.key === "Home") {
      event.preventDefault();
      const order = getChipOrder(providers);
      focusChip(groupRef.current, order[0] ?? railFilter, onRail);
    } else if (event.key === "End") {
      event.preventDefault();
      const order = getChipOrder(providers);
      focusChip(groupRef.current, order[order.length - 1] ?? railFilter, onRail);
    }
  };

  const chipRadio = (
    target: RailFilter,
    label: string,
    content: ReactNode,
    dim = false,
    tooltipText?: string
  ) => {
    const active = railFilter === target;
    const chip = (
      <button
        key={target}
        type="button"
        role="radio"
        aria-checked={active}
        aria-label={label}
        title={tooltipText ? undefined : label}
        data-rail={target}
        data-active={active}
        data-dim={dim ? "true" : undefined}
        // Roving tabindex: only the checked radio is in the Tab sequence; arrows
        // move between filters. Suppressed entirely (not merely dimmed) while
        // searching so the filter can't be keyboard-activated mid-search.
        tabIndex={active && !searching ? 0 : -1}
        disabled={searching}
        className={cn(CHIP_CLASS, dim && "opacity-45")}
        onClick={() => onRail(target)}
      >
        {content}
      </button>
    );

    if (!tooltipText) return chip;

    return (
      <Tooltip key={target} disabled={searching}>
        <TooltipTrigger render={chip} />
        <TooltipContent>{tooltipText}</TooltipContent>
      </Tooltip>
    );
  };

  return (
    <div className="flex h-8 shrink-0 items-center gap-1 border-b border-line-soft px-2">
      <div
        ref={groupRef}
        role="radiogroup"
        aria-label="Filter models by provider"
        aria-orientation="horizontal"
        aria-disabled={searching || undefined}
        onKeyDown={handleKeyDown}
        className={cn(
          "flex min-w-0 flex-1 items-center gap-1 overflow-x-auto transition-opacity",
          searching && "opacity-45"
        )}
      >
        {chipRadio("all", "All models", <LayoutGrid aria-hidden="true" className="size-3.5" />)}
        {chipRadio("fav", "Favorites", <Star aria-hidden="true" className="size-3.5" />)}
        <span aria-hidden="true" className="mx-1 h-3.5 w-px shrink-0 bg-line" />
        {providers.map(provider => {
          const label = provider.needs_auth ? `${provider.name} · needs sign in` : provider.name;
          return chipRadio(
            provider.id,
            label,
            <KindIcon
              kind={provider.runtime_provider ?? provider.id}
              registry={providerKindIconRegistry}
              size="sm"
              tone={railFilter === provider.id ? "default" : "muted"}
              className="size-3.5"
            />,
            Boolean(provider.needs_auth),
            label
          );
        })}
      </div>
      {onOpenSettings ? (
        <button
          type="button"
          aria-label="Provider settings"
          title="Provider settings"
          data-testid="runtime-selector-settings"
          className={CHIP_CLASS}
          onClick={() => onOpenSettings()}
        >
          <Settings aria-hidden="true" className="size-3.5" />
        </button>
      ) : null}
    </div>
  );
}
