"use client";

import { ChevronLeft, MoreHorizontal } from "lucide-react";
import * as React from "react";

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
  type TopbarSlotValue,
  useTopbarSlot,
  useTopbarSlotValue,
} from "./hooks/use-topbar-slot";

export interface TopbarSlotProviderProps {
  children: React.ReactNode;
}

function TopbarSlotProvider({ children }: TopbarSlotProviderProps) {
  const [store] = React.useState(createTopbarSlotStore);
  return (
    <TopbarSlotSettersContext.Provider value={store}>
      <TopbarSlotContext.Provider value={store}>{children}</TopbarSlotContext.Provider>
    </TopbarSlotSettersContext.Provider>
  );
}

export interface TopbarProps extends Omit<React.ComponentProps<"header">, "title"> {
  /**
   * Optional leading zone content anchored at the start edge (e.g. OS window
   * controls). When present the head uses the unified OS anatomy (left-aligned
   * identity + trailing status/actions).
   */
  leading?: React.ReactNode;
  /** Current route / leaf identity rendered as the shell-level H1. */
  title: React.ReactNode;
  /** Ref used by the shell to transfer focus after path navigation. */
  titleRef?: React.Ref<HTMLHeadingElement>;
  /** Quiet root glyph when the publisher has not supplied `slot.glyph`. */
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

  if (drillIn) {
    return (
      <div data-slot="topbar-identity" className="flex min-w-0 items-center gap-1">
        {slot?.onBack ? (
          <button
            type="button"
            data-slot="topbar-back"
            aria-label="Back one level"
            onClick={slot.onBack}
            className="inline-flex size-6 shrink-0 items-center justify-center rounded-sm text-muted hover:bg-hover hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring"
          >
            <ChevronLeft aria-hidden="true" className="size-3.5" />
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
          <h1
            ref={titleRef}
            tabIndex={-1}
            data-slot="topbar-title"
            data-testid="topbar-title-text"
            className="min-w-0 truncate pl-0.5 text-ws-name font-semibold tracking-tight text-fg-strong outline-none focus-visible:shadow-focus-ring"
          >
            {leaf}
          </h1>
        </nav>
      </div>
    );
  }

  return (
    <div data-slot="topbar-identity" className="flex min-w-0 items-center gap-2">
      {mark ? (
        <span
          data-slot="topbar-glyph"
          data-presentation={slot?.glyphPresentation ?? "icon"}
          aria-hidden="true"
          className={cn(
            "inline-flex size-topbar-glyph shrink-0 items-center justify-center",
            slot?.glyphPresentation === "state"
              ? "text-accent"
              : "rounded border border-line bg-badge-fill text-muted [&_svg]:size-3.5"
          )}
        >
          {mark}
        </span>
      ) : null}
      <h1
        ref={titleRef}
        tabIndex={-1}
        data-slot="topbar-title"
        data-testid="topbar-title-text"
        className="min-w-0 truncate text-ws-name font-semibold tracking-tight text-fg-strong outline-none focus-visible:shadow-focus-ring"
      >
        {leaf}
      </h1>
      {slot?.count !== undefined && slot.count !== null ? (
        <span data-slot="topbar-count" className="font-mono text-mono-id tabular-nums text-faint">
          {slot.count}
        </span>
      ) : null}
    </div>
  );
}

function Topbar({ leading, title, titleRef, glyph, className, ...props }: TopbarProps) {
  const slot = useTopbarSlotValue();
  const hasLeading = leading != null;
  const hasTrail = Boolean(slot?.status) || Boolean(slot?.actions) || Boolean(slot?.overflow);

  return (
    <header
      data-slot="topbar"
      className={cn(
        "flex h-11 min-w-0 shrink-0 items-center gap-2.5 overflow-hidden border-b border-line bg-canvas px-3",
        className
      )}
      {...props}
    >
      {hasLeading ? (
        <div data-slot="topbar-leading" className="flex shrink-0 items-center">
          {leading}
        </div>
      ) : null}
      <TopbarIdentity title={title} titleRef={titleRef} glyph={glyph} slot={slot} />
      {slot?.nav ? (
        <div data-slot="topbar-nav" className="no-scrollbar min-w-0 shrink-0 overflow-x-auto">
          {slot.nav}
        </div>
      ) : null}
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
          {slot?.status && (slot.actions || slot.overflow) ? (
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
          {slot?.overflow ? (
            <div
              data-slot="topbar-overflow"
              data-testid="topbar-overflow"
              className="inline-flex shrink-0 items-center"
            >
              {slot.overflow}
            </div>
          ) : null}
        </div>
      ) : null}
    </header>
  );
}

const TopbarOverflowIcon = MoreHorizontal;

export { Topbar, TopbarOverflowIcon, TopbarSlotProvider, useTopbarSlot, useTopbarSlotValue };
export type { TopbarCrumb, TopbarSlotValue };
