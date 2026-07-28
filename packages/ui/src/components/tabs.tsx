"use client";

import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import * as React from "react";

import { cn } from "../lib/utils";

function Tabs({ className, orientation = "horizontal", ...props }: TabsPrimitive.Root.Props) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      data-orientation={orientation}
      orientation={orientation}
      className={cn("group/tabs flex gap-2 data-horizontal:flex-col", className)}
      {...props}
    />
  );
}

function TabsList({ activateOnFocus = true, className, ...props }: TabsPrimitive.List.Props) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      activateOnFocus={activateOnFocus}
      className={cn(
        "group/tabs-list inline-flex items-center gap-1 border-b border-line group-data-horizontal/tabs:h-tabs-list group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col group-data-vertical/tabs:border-b-0 group-data-vertical/tabs:border-r",
        className
      )}
      {...props}
    />
  );
}

export interface TabsTriggerProps extends TabsPrimitive.Tab.Props {
  count?: number;
  liveLabel?: React.ReactNode;
}

function TabsTrigger({ className, children, count, liveLabel, ...props }: TabsTriggerProps) {
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-trigger"
      className={cn(
        "relative inline-flex h-full items-center gap-1.5 px-2 text-form-label font-medium tracking-eyebrow whitespace-nowrap text-muted transition-colors duration-base ease-out hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start has-data-[icon=inline-end]:pr-1 has-data-[icon=inline-start]:pl-1 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        "data-active:bg-transparent data-active:text-fg-strong",
        "after:absolute after:bg-fg-strong after:opacity-0 after:transition-opacity group-data-horizontal/tabs:after:right-0 group-data-horizontal/tabs:after:-bottom-px group-data-horizontal/tabs:after:left-0 group-data-horizontal/tabs:after:h-tab-underline group-data-vertical/tabs:after:inset-y-0 group-data-vertical/tabs:after:-right-1 group-data-vertical/tabs:after:w-px data-active:after:opacity-100",
        className
      )}
      {...props}
    >
      <span data-slot="tabs-trigger-label" className="inline-flex min-w-0 items-center">
        {children}
      </span>
      {count !== undefined ? (
        <span
          data-slot="tabs-trigger-count"
          className="inline-flex h-pill-xs min-w-count-chip-sm items-center justify-center rounded-mono-badge bg-canvas-soft px-1 font-mono text-badge font-medium tabular-nums text-muted"
        >
          {count}
        </span>
      ) : null}
      {liveLabel ? (
        <span
          aria-live="polite"
          data-slot="tabs-trigger-live"
          className="eyebrow inline-flex h-4 items-center gap-1 rounded-sm bg-accent-tint px-1.5 text-accent"
        >
          <span aria-hidden="true" className="size-1.5 rounded-full bg-accent" />
          {liveLabel}
        </span>
      ) : null}
    </TabsPrimitive.Tab>
  );
}

function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props) {
  return (
    <TabsPrimitive.Panel
      data-slot="tabs-content"
      className={cn("flex-1 text-small-body outline-none", className)}
      {...props}
    />
  );
}

export { Tabs, TabsContent, TabsList, TabsTrigger };
