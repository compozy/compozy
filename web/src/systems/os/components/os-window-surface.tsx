import {
  Button,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  Topbar,
  TopbarSlotProvider,
  useTopbarSlotValue,
  type TopbarSlotStore,
} from "@compozy/ui";
import { X } from "@compozy/ui/icons";
import * as React from "react";

import { cn } from "@/lib/utils";

function OsWindowToolbar() {
  const slot = useTopbarSlotValue();
  if (!slot?.toolbar) return null;
  return (
    <div
      data-slot="os-window-toolbar"
      className="no-scrollbar flex h-window-toolbar shrink-0 items-center gap-2.5 overflow-x-auto border-b border-line bg-canvas px-3 [&_[data-slot=listing-toolbar]]:w-full [&_[data-slot=listing-toolbar]]:flex-nowrap"
    >
      {slot.toolbar}
    </div>
  );
}

function OsWindowBody({
  children,
  onScrolled,
  transitionName,
}: {
  children: React.ReactNode;
  onScrolled: (scrolled: boolean) => void;
  transitionName?: string;
}) {
  const handleScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const scrollTarget = event.target;
    if (!(scrollTarget instanceof HTMLElement)) return;
    onScrolled(scrollTarget.scrollTop > 2);
  };

  return (
    <div
      data-slot="os-window-body"
      className="flex min-h-0 flex-1 flex-col overflow-auto bg-canvas"
      // Named only while a window-manager drill runs (tokens-runtime.css): a permanent name made
      // every in-window <ViewTransition> also capture and cross-fade the whole body.
      data-view-transition-body={transitionName}
      onScrollCapture={handleScroll}
    >
      {children}
    </div>
  );
}

/** One quiet icon control in the window head, labelled by its tooltip. */
function OsWindowHeadControl({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick?: () => void;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="icon-sm"
            aria-label={label}
            data-slot="os-window-control"
            onClick={onClick}
            onPointerDown={event => event.stopPropagation()}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

/**
 * One window's head, toolbar and body. The head carries the window's identity and its controls;
 * a window is a plain box in its tab's split, so the only control here is Close.
 */
export interface OsWindowSurfaceProps extends Omit<React.ComponentProps<"section">, "title"> {
  title: React.ReactNode;
  glyph?: React.ReactNode;
  /** Host context before the identity: the item's project in the All projects desktop. */
  leading?: React.ReactNode;
  focused?: boolean;
  /** Closes this window; omitted renders a head without controls. */
  onClose?: () => void;
  headClassName?: string;
  slotStore?: TopbarSlotStore;
  /**
   * `view-transition-name` the body takes during a window-manager drill (`drill-in` /
   * `drill-out` transition types) and only then. Only the focused, visible window may
   * carry one — names must be unique per document or the transition aborts.
   */
  bodyTransitionName?: string;
}

export function OsWindowSurface({
  title,
  glyph,
  leading,
  focused = true,
  onClose,
  headClassName,
  slotStore,
  bodyTransitionName,
  className,
  children,
  ...props
}: OsWindowSurfaceProps) {
  const [scrolled, setScrolled] = React.useState(false);

  return (
    <section
      data-slot="os-window-surface"
      className={cn("flex min-h-0 flex-1 flex-col overflow-hidden bg-canvas", className)}
      {...props}
    >
      <TopbarSlotProvider store={slotStore}>
        <Topbar
          data-slot="os-window-head"
          data-scrolled={scrolled ? "" : undefined}
          // S5 adds the window head's split controls (Move to new tab) beside Close.
          controls={
            onClose ? (
              <OsWindowHeadControl label="Close window" onClick={onClose}>
                <X aria-hidden="true" className="size-3.75" />
              </OsWindowHeadControl>
            ) : undefined
          }
          title={title}
          glyph={glyph}
          leading={leading}
          className={cn(
            scrolled && "border-line-strong",
            !focused &&
              "[&_[data-slot=topbar-identity]]:opacity-55 [&_[data-slot=topbar-trailing]]:opacity-55",
            headClassName
          )}
        />
        <OsWindowToolbar />
        <OsWindowBody onScrolled={setScrolled} transitionName={bodyTransitionName}>
          {children}
        </OsWindowBody>
      </TopbarSlotProvider>
    </section>
  );
}
