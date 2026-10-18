"use client";

import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import * as React from "react";

import { cn } from "../../lib/utils";
import { pillGroupSegmentVariants } from "./pill-group-variants";

/**
 * Peer sister-route navigation — the leading pill segments of the tools strip
 * on every route with sibling views (ADR-007/D3; the head stays two-element).
 * Real links inside a labeled `nav` with no track; the segment look is visual
 * only — `Tabs` stay for in-route panels and `PillGroup` stays a mode selector.
 */
function RouteNav({ className, ...props }: React.ComponentProps<"nav">) {
  return (
    <nav
      data-slot="route-nav"
      className={cn("inline-flex shrink-0 items-center gap-1", className)}
      {...props}
    />
  );
}

/**
 * One sister-route link, drawn as a PillGroup segment. Router links mark the
 * active route with `aria-current="page"`; the lifted segment keys off it.
 */
function RouteNavLink({ className, render, ...props }: useRender.ComponentProps<"a">) {
  return useRender({
    defaultTagName: "a",
    props: mergeProps<"a">(
      {
        className: cn(
          pillGroupSegmentVariants({ size: "md" }),
          "aria-[current=page]:bg-surface-2 aria-[current=page]:text-fg",
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

/** Quiet count beside a route-nav label (e.g. Inbox 2), as in PillGroup. */
function RouteNavCount({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="route-nav-count"
      className={cn("font-normal tabular-nums text-subtle", className)}
      {...props}
    />
  );
}

const RouteNavCompound = Object.assign(RouteNav, {
  Link: RouteNavLink,
  Count: RouteNavCount,
});

export { RouteNavCompound as RouteNav };
