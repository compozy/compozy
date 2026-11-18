import { ChevronLeft, ChevronRight, LayoutGrid, Settings, Star } from "@compozy/ui/icons";
import { useReducedMotionConfig } from "motion/react";
import { type KeyboardEvent, type ReactNode } from "react";

import {
  cn,
  KindIcon,
  providerKindIconRegistry,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { useRailOverflow } from "./use-rail-overflow";
import type { RailFilter } from "./use-runtime-selector";
import type { RuntimeProviderOption } from "./types";

const CHIP_CLASS =
  "grid size-7 shrink-0 place-items-center rounded-sm text-subtle outline-none transition-colors hover:bg-surface-2 hover:text-fg focus-visible:bg-surface-2 focus-visible:text-fg focus-visible:shadow-focus-ring disabled:pointer-events-none data-[active=true]:bg-selected data-[active=true]:text-fg data-[active=true]:ring-1 data-[active=true]:ring-line-strong data-[active=true]:ring-inset";

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

interface ChipRadioOptions {
  target: RailFilter;
  label: string;
  content: ReactNode;
  active: boolean;
  searching: boolean;
  dim?: boolean;
  tooltipText?: string;
  onRail: (target: RailFilter) => void;
}

function renderChipRadio({
  target,
  label,
  content,
  active,
  searching,
  dim = false,
  tooltipText,
  onRail,
}: ChipRadioOptions) {
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
}

interface RailScrollEdgeProps {
  side: "start" | "end";
  visible: boolean;
  onScroll: (direction: -1 | 1) => void;
}

/**
 * Fade + chevron at a hidden edge. Pointer-only: arrow keys already page the
 * rail by focusing the next chip, so an extra tab stop would only slow the
 * search → radiogroup → settings sequence. `mousedown` is cancelled to keep
 * focus (and the combobox's active descendant) where it was.
 */
function RailScrollEdge({ side, visible, onScroll }: RailScrollEdgeProps) {
  const Icon = side === "start" ? ChevronLeft : ChevronRight;

  return (
    <button
      type="button"
      aria-hidden="true"
      tabIndex={-1}
      data-rail-scroll={side}
      data-visible={visible ? "true" : "false"}
      className={cn(
        "absolute inset-y-0 z-2 grid w-6 place-items-center text-subtle transition-opacity hover:text-fg",
        side === "start"
          ? "start-0 bg-linear-to-r from-popover via-popover/85 to-transparent rtl:bg-linear-to-l"
          : "end-0 bg-linear-to-l from-popover via-popover/85 to-transparent rtl:bg-linear-to-r",
        visible ? "opacity-100" : "pointer-events-none opacity-0"
      )}
      onMouseDown={event => event.preventDefault()}
      onClick={() => onScroll(side === "start" ? -1 : 1)}
    >
      <Icon aria-hidden="true" className="size-3.5 rtl:rotate-180" />
    </button>
  );
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
 *
 * The 28px chips keep the line at 32px, so the search row above and the list
 * below never move; overflow is discoverable through edge fades and paging
 * affordances instead of a native scrollbar.
 */
export function ProviderChips({
  providers,
  railFilter,
  searching,
  onRail,
  onOpenSettings,
}: ProviderChipsProps) {
  const reducedMotion = useReducedMotionConfig() === true;
  const chipOrder = getChipOrder(providers);
  const { trackRef, overflow } = useRailOverflow(chipOrder.length);

  const scrollRail = (direction: -1 | 1) => {
    const track = trackRef.current;
    if (!track) return;
    const step = Math.max(72, Math.round(track.clientWidth * 0.6));
    const rtl = window.getComputedStyle(track).direction === "rtl";
    track.scrollBy({
      left: direction * step * (rtl ? -1 : 1),
      behavior: reducedMotion ? "auto" : "smooth",
    });
  };

  const moveFocus = (current: RailFilter, direction: 1 | -1) => {
    const position = chipOrder.indexOf(current);
    const start = position < 0 ? 0 : position;
    const next = chipOrder[(start + direction + chipOrder.length) % chipOrder.length];
    focusChip(trackRef.current, next, onRail);
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
      focusChip(trackRef.current, chipOrder[0] ?? railFilter, onRail);
    } else if (event.key === "End") {
      event.preventDefault();
      focusChip(trackRef.current, chipOrder[chipOrder.length - 1] ?? railFilter, onRail);
    }
  };

  return (
    <div className="flex h-8 shrink-0 items-center gap-1 border-b border-line-soft px-2">
      <div className={cn("relative min-w-0 flex-1 transition-opacity", searching && "opacity-45")}>
        <div
          ref={trackRef}
          role="radiogroup"
          aria-label="Filter models by provider"
          aria-orientation="horizontal"
          aria-disabled={searching || undefined}
          onKeyDown={handleKeyDown}
          className="flex w-full min-w-0 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {renderChipRadio({
            target: "all",
            label: "All models",
            content: <LayoutGrid aria-hidden="true" className="size-3.5" />,
            active: railFilter === "all",
            searching,
            onRail,
          })}
          {renderChipRadio({
            target: "fav",
            label: "Favorites",
            content: <Star aria-hidden="true" className="size-3.5" />,
            active: railFilter === "fav",
            searching,
            onRail,
          })}
          <span aria-hidden="true" className="mx-1 h-3.5 w-px shrink-0 bg-line" />
          {providers.map(provider => {
            const label = provider.disabled
              ? `${provider.name} · disabled`
              : provider.needs_auth
                ? `${provider.name} · needs sign in`
                : provider.name;
            return renderChipRadio({
              target: provider.id,
              label,
              content: (
                <KindIcon
                  kind={provider.runtime_provider ?? provider.id}
                  registry={providerKindIconRegistry}
                  size="sm"
                  tone={railFilter === provider.id ? "default" : "muted"}
                  className="size-3.5"
                />
              ),
              active: railFilter === provider.id,
              searching,
              dim: Boolean(provider.needs_auth),
              tooltipText: label,
              onRail,
            });
          })}
        </div>
        <RailScrollEdge
          side="start"
          visible={overflow === "start" || overflow === "start end"}
          onScroll={scrollRail}
        />
        <RailScrollEdge
          side="end"
          visible={overflow === "end" || overflow === "start end"}
          onScroll={scrollRail}
        />
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
