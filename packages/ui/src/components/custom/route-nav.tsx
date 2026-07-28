"use client";

import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import * as React from "react";

import { cn } from "../../lib/utils";

/**
 * Peer sister-route navigation for the topbar `nav` zone — rendered in the
 * 44px head immediately after identity (`data-slot="topbar-nav"`), never in
 * the 38px tools strip (pagehead-redesign §02). Real links inside a labeled
 * `nav`; the segmented appearance is visual only — `Tabs` stay for in-route
 * panels and `PillGroup` stays a mode selector.
 */
function RouteNav({ className, ...props }: React.ComponentProps<"nav">) {
  return (
    <nav
      data-slot="route-nav"
      className={cn(
        "inline-flex shrink-0 items-center gap-px rounded-md bg-canvas-soft p-0.5",
        className
      )}
      {...props}
    />
  );
}

/**
 * One sister-route link. Router links mark the active route with
 * `aria-current="page"`; the elevated segment styling keys off it.
 */
function RouteNavLink({ className, render, ...props }: useRender.ComponentProps<"a">) {
  return useRender({
    defaultTagName: "a",
    props: mergeProps<"a">(
      {
        className: cn(
          "inline-flex h-6 items-center gap-1.5 whitespace-nowrap rounded-xs px-2.5 text-form-label font-medium tracking-eyebrow text-subtle transition-colors duration-base ease-out hover:text-fg aria-[current=page]:bg-elevated aria-[current=page]:text-fg-strong aria-[current=page]:shadow-highlight focus-visible:outline-none focus-visible:shadow-focus-ring",
          className
        ),
      },
      props
    ),
    render,
    state: {
      slot: "route-nav-link",
    },
  });
}

/** Mono count adornment inside a route-nav link (e.g. Inbox 2). */
function RouteNavCount({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="route-nav-count"
      className={cn("font-mono text-micro tabular-nums text-muted", className)}
      {...props}
    />
  );
}

const RouteNavCompound = Object.assign(RouteNav, {
  Link: RouteNavLink,
  Count: RouteNavCount,
});

export { RouteNavCompound as RouteNav };
