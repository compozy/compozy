import { Topbar, TopbarSlotProvider, useTopbarSlotValue } from "@agh/ui";
import * as React from "react";

import { cn } from "@/lib/utils";

import { OsTrafficLights, type OsTrafficLightAction } from "./os-traffic-lights";

/**
 * Floating window frame: the shell chrome around one app's route subtree.
 * The window head owns route identity — 44px unified bar (traffic
 * lights · glyph/title or drill-in trail · status + actions) with an optional
 * 38px context strip for listing tools. Per-window `TopbarSlotProvider` lets
 * locations publish identity/tools via `useTopbarSlot`. Frame depth (border +
 * cast shadow) is the sanctioned shell carve-out; window-body content stays flat.
 *
 * Presentational only — drag, z-order, and focus come from the window
 * manager. `focused` selects the focused/unfocused depth and head-dim states.
 */
export interface OsWindowFrameProps extends Omit<React.ComponentProps<"section">, "title"> {
  /** Default window title when the location has not published a crumb override. */
  title: React.ReactNode;
  /** Quiet root glyph when the location has not published `slot.glyph`. */
  glyph?: React.ReactNode;
  /** Focused (sharp border, cast shadow) vs unfocused (dimmed head, lighter shadow). */
  focused?: boolean;
  /** Traffic-light activation. Omit to render the controls as presentation. */
  onTrafficLight?: (action: OsTrafficLightAction) => void;
  /** Wraps the zoom control with its hover menu (window manager wiring). */
  zoomMenu?: (button: React.ReactNode) => React.ReactNode;
  /** Extra classes on the head `Topbar` (the WM's drag-handle hook). */
  headClassName?: string;
  /**
   * Compact (<960px, os-v2.css mobile block): full-bleed stack surface — no
   * border/radius/cast shadow, zoom hidden, enlarged control hit areas.
   */
  presentation?: "floating" | "compact";
}

function OsWindowToolbar() {
  const slot = useTopbarSlotValue();
  if (!slot?.toolbar) return null;
  return (
    <div
      data-slot="os-window-toolbar"
      className="no-scrollbar flex h-[38px] shrink-0 items-center gap-2.5 overflow-x-auto border-b border-line bg-canvas px-3 py-0.5 [&_[data-slot=listing-toolbar]]:w-full [&_[data-slot=listing-toolbar]]:flex-nowrap"
    >
      {slot.toolbar}
    </div>
  );
}

function OsWindowBody({
  children,
  onScrolled,
}: {
  children: React.ReactNode;
  onScrolled: (scrolled: boolean) => void;
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
      onScrollCapture={handleScroll}
    >
      {children}
    </div>
  );
}

function targetsWindowChromeControl(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    target.closest('[data-slot="os-traffic-lights"] button[data-action]') !== null
  );
}

export function OsWindowFrame({
  title,
  glyph,
  focused = true,
  onTrafficLight,
  zoomMenu,
  headClassName,
  presentation = "floating",
  className,
  children,
  onPointerDownCapture,
  ...props
}: OsWindowFrameProps) {
  const [scrolled, setScrolled] = React.useState(false);
  const compact = presentation === "compact";

  return (
    <section
      data-slot="os-window-frame"
      data-focused={focused ? "" : undefined}
      data-presentation={compact ? "compact" : undefined}
      className={cn(
        "flex min-h-0 flex-col overflow-hidden bg-canvas",
        compact
          ? "rounded-none border-0 shadow-none"
          : [
              "rounded-window border",
              focused
                ? "border-line-focus shadow-window"
                : "border-line-strong shadow-window-unfocused",
            ],
        className
      )}
      onPointerDownCapture={event => {
        if (targetsWindowChromeControl(event.target)) return;
        onPointerDownCapture?.(event);
      }}
      {...props}
    >
      <TopbarSlotProvider>
        <Topbar
          data-slot="os-window-head"
          data-scrolled={scrolled ? "" : undefined}
          leading={
            <OsTrafficLights onSelect={onTrafficLight} compact={compact} wrapZoom={zoomMenu} />
          }
          title={title}
          glyph={glyph}
          className={cn(
            scrolled && "border-line-strong shadow-window-head-scrolled",
            !focused &&
              "[&_[data-slot=topbar-title]]:text-subtle [&_[data-slot=topbar-identity]]:opacity-60 [&_[data-slot=topbar-trailing]]:opacity-60",
            headClassName
          )}
        />
        <OsWindowToolbar />
        <OsWindowBody onScrolled={setScrolled}>{children}</OsWindowBody>
      </TopbarSlotProvider>
    </section>
  );
}
