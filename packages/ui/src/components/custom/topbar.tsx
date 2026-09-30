"use client";

import { ChevronLeft, MoreHorizontal } from "lucide-react";
import * as React from "react";

import { Popover, PopoverContent, PopoverTrigger } from "../popover";
import { KindIcon } from "./kind-icon";

import { cn } from "../../lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../dropdown-menu";
import {
  createTopbarSlotStore,
  TopbarSlotContext,
  TopbarSlotSettersContext,
  type TopbarCrumb,
  type TopbarSlotStore,
  type TopbarSlotValue,
  useTopbarSlotValue,
} from "./hooks/use-topbar-slot";

interface TopbarSlotProviderProps {
  children: React.ReactNode;
  /** External store handle so a shell can read this surface's slot (deck labels). */
  store?: TopbarSlotStore;
}

function TopbarSlotProvider({ children, store }: TopbarSlotProviderProps) {
  const [fallback] = React.useState(createTopbarSlotStore);
  const active = store ?? fallback;
  return (
    <TopbarSlotSettersContext.Provider value={active}>
      <TopbarSlotContext.Provider value={active}>{children}</TopbarSlotContext.Provider>
    </TopbarSlotSettersContext.Provider>
  );
}

interface TopbarProps extends Omit<React.ComponentProps<"header">, "title"> {
  /**
   * Window controls anchored at the end edge, after the published trail and a
   * hairline divider (identity · trail · controls).
   */
  controls?: React.ReactNode;
  /** Current route / leaf identity rendered as the shell-level H1. */
  title: React.ReactNode;
  /** Ref used by the shell to transfer focus after path navigation. */
  titleRef?: React.Ref<HTMLHeadingElement>;
  /** Root identity glyph (rendered in the identity well) when the publisher has not supplied `slot.glyph`. */
  glyph?: React.ReactNode;
}

const MAX_VISIBLE_PARENT_CRUMBS = 2;

function collapseCrumbs(crumbs: readonly TopbarCrumb[]): {
  visible: readonly TopbarCrumb[];
  hidden: readonly TopbarCrumb[];
} {
  if (crumbs.length <= MAX_VISIBLE_PARENT_CRUMBS) {
    return { visible: crumbs, hidden: [] };
  }
  return {
    visible: [crumbs[0]!, crumbs[crumbs.length - 1]!],
    hidden: crumbs.slice(1, -1),
  };
}

/** Keeps identity controls visible while long titles disclose their complete text. */
function TopbarIdentity({
  title,
  titleRef,
  glyph,
  slot,
}: {
  title: React.ReactNode;
  titleRef?: React.Ref<HTMLHeadingElement>;
  glyph?: React.ReactNode;
  slot: TopbarSlotValue | null;
}) {
  const leaf = slot?.crumb ?? title;
  const parents = slot?.crumbs ?? [];
  const drillIn = Boolean(slot?.onBack) || parents.length > 0;
  const mark = slot?.glyph ?? glyph;
  const { visible, hidden } = collapseCrumbs(parents);

  // The document's own ⋯ menu belongs to its identity, right after the title
  // (prototype `.id .more`); status chips and actions stay in the trail.
  const overflow = slot?.overflow ? (
    <div
      data-slot="topbar-overflow"
      data-testid="topbar-overflow"
      // Drill-in identity packs back/crumbs at gap-1; keep the 9px identity gap before ⋯.
      className={cn("inline-flex shrink-0 items-center", drillIn && "pl-1.25")}
    >
      {slot.overflow}
    </div>
  ) : null;

  const count =
    slot?.count !== undefined && slot.count !== null ? (
      <span data-slot="topbar-count" className="font-mono text-mono-id tabular-nums text-faint">
        {slot.count}
      </span>
    ) : null;

  if (drillIn) {
    return (
      <div data-slot="topbar-identity" className="flex min-w-0 items-center gap-1">
        {slot?.onBack ? (
          <button
            type="button"
            data-slot="topbar-back"
            aria-label="Back one level"
            onClick={slot.onBack}
            className="inline-flex size-6.5 shrink-0 items-center justify-center rounded-xs text-subtle transition-colors duration-fast hover:bg-surface-2 hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
          >
            <ChevronLeft aria-hidden="true" className="size-4" />
          </button>
        ) : null}
        <nav
          data-slot="topbar-crumbs"
          aria-label="Window path"
          className="flex min-w-0 items-center gap-0.5"
        >
          {visible.map((crumb, index) => {
            const isFirst = index === 0;
            const showEllipsis = isFirst && hidden.length > 0;
            return (
              <React.Fragment key={crumb.id}>
                {index > 0 ? (
                  <span aria-hidden="true" className="px-0.5 text-small-body text-faint">
                    /
                  </span>
                ) : null}
                <button
                  type="button"
                  data-slot="topbar-crumb"
                  onClick={crumb.onSelect}
                  className="max-w-[150px] truncate rounded-sm px-1 py-px text-ws-name font-medium text-subtle hover:bg-row-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
                >
                  {crumb.label}
                </button>
                {showEllipsis ? (
                  <>
                    <span aria-hidden="true" className="px-0.5 text-small-body text-faint">
                      /
                    </span>
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        aria-label="Show hidden path levels"
                        data-slot="topbar-crumb-more"
                        render={
                          <button
                            type="button"
                            className="rounded-sm px-1 py-px text-ws-name font-medium text-faint hover:bg-row-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
                          />
                        }
                      >
                        <span aria-hidden="true">…</span>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="start" className="w-auto">
                        {hidden.map(hiddenCrumb => (
                          <DropdownMenuItem key={hiddenCrumb.id} onClick={hiddenCrumb.onSelect}>
                            {hiddenCrumb.label}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </>
                ) : null}
              </React.Fragment>
            );
          })}
          {visible.length > 0 ? (
            <span aria-hidden="true" className="px-0.5 text-small-body text-faint">
              /
            </span>
          ) : null}
          <TopbarTitle titleRef={titleRef} className="pl-0.5">
            {leaf}
          </TopbarTitle>
        </nav>
        {count}
        {overflow}
      </div>
    );
  }

  return (
    <div data-slot="topbar-identity" className="flex min-w-0 items-center gap-2.25">
      {mark && slot?.glyphPresentation === "state" ? (
        <span
          data-slot="topbar-glyph"
          data-presentation="state"
          aria-hidden="true"
          className="inline-flex shrink-0 items-center justify-center text-accent"
        >
          {mark}
        </span>
      ) : mark ? (
        <KindIcon
          aria-hidden="true"
          data-slot="topbar-glyph"
          data-presentation="icon"
          glyph={mark}
          size="sm"
          tone="well"
        />
      ) : null}
      <TopbarTitle titleRef={titleRef}>{leaf}</TopbarTitle>
      {count}
      {overflow}
    </div>
  );
}

/** Constrains the heading and exposes the full title to keyboard and pointer users. */
function TopbarTitle({
  children,
  titleRef,
  className,
}: {
  children: React.ReactNode;
  titleRef?: React.Ref<HTMLHeadingElement>;
  className?: string;
}) {
  const titleId = React.useId();
  return (
    <>
      <span id={`${titleId}-action`} hidden>
        Show full title:
      </span>
      <h1
        ref={titleRef}
        aria-labelledby={titleId}
        tabIndex={-1}
        data-slot="topbar-title"
        data-testid="topbar-title-text"
        className={cn(
          "min-w-0 max-w-xs text-card-title font-medium text-fg outline-none",
          className
        )}
      >
        <Popover>
          <PopoverTrigger
            aria-labelledby={`${titleId}-action ${titleId}`}
            className="block max-w-full truncate rounded-sm text-left focus-visible:outline-none focus-visible:shadow-focus-ring"
          >
            <span id={titleId}>{children}</span>
          </PopoverTrigger>
          <PopoverContent
            align="start"
            aria-label="Full title"
            className="max-h-64 max-w-[calc(100vw-2rem)] overflow-auto whitespace-pre-wrap select-text [overflow-wrap:anywhere]"
          >
            {children}
          </PopoverContent>
        </Popover>
      </h1>
    </>
  );
}

function Topbar({ controls, title, titleRef, glyph, className, ...props }: TopbarProps) {
  const slot = useTopbarSlotValue();
  const hasControls = controls != null;
  const hasTrail = Boolean(slot?.status) || Boolean(slot?.actions);

  return (
    <header
      data-slot="topbar"
      className={cn(
        "flex h-window-head min-w-0 shrink-0 items-center gap-2.5 overflow-hidden border-b border-line bg-canvas pr-2 pl-4",
        className
      )}
      {...props}
    >
      <TopbarIdentity title={title} titleRef={titleRef} glyph={glyph} slot={slot} />
      <div data-slot="topbar-flex" className="min-h-full min-w-2 flex-1 self-stretch" />
      {hasTrail ? (
        <div
          data-slot="topbar-trailing"
          className="flex min-w-0 shrink-0 items-center justify-end gap-2"
        >
          {slot?.status ? (
            <div data-slot="topbar-status" className="inline-flex shrink-0 items-center gap-1.5">
              {slot.status}
            </div>
          ) : null}
          {slot?.status && slot.actions ? (
            <span
              aria-hidden="true"
              data-slot="topbar-vsep"
              className="h-3.5 w-px shrink-0 bg-line-strong"
            />
          ) : null}
          {slot?.actions ? (
            <div data-slot="topbar-actions" className="flex min-w-0 items-center gap-1.5">
              {slot.actions}
            </div>
          ) : null}
        </div>
      ) : null}
      {hasControls ? (
        <>
          <span
            aria-hidden="true"
            data-slot="topbar-controls-vsep"
            className="mx-1.5 h-4.5 w-px shrink-0 bg-line"
          />
          <div data-slot="topbar-controls" className="flex shrink-0 items-center">
            {controls}
          </div>
        </>
      ) : null}
    </header>
  );
}

const TopbarOverflowIcon = MoreHorizontal;

export { Topbar, TopbarOverflowIcon, TopbarSlotProvider };
export type { TopbarProps, TopbarSlotProviderProps };
