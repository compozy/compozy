import { LayoutGrid, Settings, Star } from "lucide-react";
import { useRef, type KeyboardEvent, type ReactNode } from "react";

import { cn, KindIcon, providerKindIconRegistry } from "@agh/ui";

import type { RailFilter } from "./use-runtime-selector";
import type { RuntimeProviderOption } from "./types";

const RAIL_ITEM_CLASS =
  "relative mx-auto grid size-9 place-items-center rounded-md text-muted outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:text-fg-strong focus-visible:ring-2 focus-visible:ring-accent disabled:pointer-events-none data-[active=true]:bg-row-selected data-[active=true]:text-fg-strong";

function getRailOrder(providers: RuntimeProviderOption[]): RailFilter[] {
  return ["all", "fav", ...providers.map(provider => provider.id)];
}

function ActiveMark({ active }: { active: boolean }) {
  if (!active) return null;
  return (
    <span
      aria-hidden="true"
      className="absolute top-2 bottom-2 -left-1.5 w-0.5 rounded-full bg-accent"
    />
  );
}

export interface ProviderRailProps {
  providers: RuntimeProviderOption[];
  railFilter: RailFilter;
  searching: boolean;
  onRail: (target: RailFilter) => void;
  onOpenSettings?: () => void;
}

/**
 * Local mutually-exclusive model filter (All / Favorites / one provider). This is
 * a `radiogroup`, NOT tabs: the rail filters the list in place; it does not swap
 * out a tabpanel, so a `tablist`/`tab`→`listbox` relationship would be a false
 * claim. Roving tabindex + arrow keys move and select in one step (the WAI-ARIA
 * radio pattern). A non-empty search disables the whole group semantically —
 * search always spans every provider. Provider Settings sits OUTSIDE the group: it
 * is an escape hatch, not a filter, so it must not read as a fourth radio.
 */
export function ProviderRail({
  providers,
  railFilter,
  searching,
  onRail,
  onOpenSettings,
}: ProviderRailProps) {
  const railRef = useRef<HTMLDivElement>(null);

  const moveFocus = (current: RailFilter, direction: 1 | -1) => {
    // Ordered roving targets: All, Favorites, then each provider. Build the
    // order only for keyboard navigation instead of allocating a hook
    // dependency on every render.
    const order = getRailOrder(providers);
    const position = order.indexOf(current);
    const start = position < 0 ? 0 : position;
    const next = order[(start + direction + order.length) % order.length];
    onRail(next);
    const target = railRef.current?.querySelector<HTMLButtonElement>(
      `[data-rail="${CSS.escape(String(next))}"]`
    );
    target?.focus();
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (searching) return;
    if (event.key === "ArrowDown" || event.key === "ArrowRight") {
      event.preventDefault();
      moveFocus(railFilter, 1);
    } else if (event.key === "ArrowUp" || event.key === "ArrowLeft") {
      event.preventDefault();
      moveFocus(railFilter, -1);
    } else if (event.key === "Home") {
      event.preventDefault();
      const order = getRailOrder(providers);
      moveFocus(order[order.length - 1] ?? railFilter, 1);
    } else if (event.key === "End") {
      event.preventDefault();
      const order = getRailOrder(providers);
      moveFocus(order[0] ?? railFilter, -1);
    }
  };

  const railRadio = (target: RailFilter, label: string, content: ReactNode, dim = false) => {
    const active = railFilter === target;
    return (
      <button
        key={target}
        type="button"
        role="radio"
        aria-checked={active}
        aria-label={label}
        title={label}
        data-rail={target}
        data-active={active}
        data-dim={dim ? "true" : undefined}
        // Roving tabindex: only the checked radio is in the Tab sequence; arrows
        // move between filters. Suppressed entirely (not merely dimmed) while
        // searching so the filter can't be keyboard-activated mid-search.
        tabIndex={active && !searching ? 0 : -1}
        disabled={searching}
        className={cn(RAIL_ITEM_CLASS, dim && "opacity-45")}
        onClick={() => onRail(target)}
      >
        <ActiveMark active={active} />
        {content}
      </button>
    );
  };

  return (
    <div className="flex w-[52px] shrink-0 flex-col border-r border-line-soft bg-canvas">
      <div
        ref={railRef}
        role="radiogroup"
        aria-label="Filter models by provider"
        aria-orientation="vertical"
        aria-disabled={searching || undefined}
        onKeyDown={handleKeyDown}
        className={cn(
          "flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto py-2 transition-opacity",
          searching && "opacity-45"
        )}
      >
        {railRadio("all", "All models", <LayoutGrid aria-hidden="true" className="size-4" />)}
        {railRadio("fav", "Favorites", <Star aria-hidden="true" className="size-4" />)}
        <span aria-hidden="true" className="mx-2.5 my-0.5 h-px shrink-0 bg-line-soft" />
        {providers.map(provider =>
          railRadio(
            provider.id,
            provider.needs_auth ? `${provider.name} · needs sign in` : provider.name,
            <KindIcon
              kind={provider.runtime_provider ?? provider.id}
              registry={providerKindIconRegistry}
              size="sm"
              tone={railFilter === provider.id ? "default" : "muted"}
              className="size-5"
            />,
            Boolean(provider.needs_auth)
          )
        )}
      </div>
      {onOpenSettings ? (
        <div className="shrink-0 border-t border-line-soft py-1.5">
          <button
            type="button"
            aria-label="Provider settings"
            title="Provider settings"
            data-testid="runtime-selector-settings"
            className={RAIL_ITEM_CLASS}
            onClick={() => onOpenSettings()}
          >
            <Settings aria-hidden="true" className="size-4" />
          </button>
        </div>
      ) : null}
    </div>
  );
}
