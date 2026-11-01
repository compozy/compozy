"use client";

import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import * as React from "react";

import { cn } from "../lib/utils";
import { SkeletonText } from "./skeleton";

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

/** A tab's count: the number once it is known, or `"pending"` while it is still being read. */
export type TabsTriggerCount = number | "pending";

export interface TabsTriggerProps extends TabsPrimitive.Tab.Props {
  /**
   * The count after the label. `"pending"` holds the slot with a placeholder, so the tab and the
   * tabs after it do not move when the number arrives; absent shows no count at all. Use
   * `"pending"` only for a count that has not been read yet, never for a refetch.
   */
  count?: TabsTriggerCount;
  /**
   * How many digits the placeholder of a pending count holds, two by default. Set it to the
   * count's usual size: the tab keeps its width when the number has that many digits, and moves by
   * the difference when it does not.
   */
  pendingCountDigits?: number;
  liveLabel?: React.ReactNode;
}

function TabsTrigger({
  className,
  children,
  count,
  pendingCountDigits = 2,
  liveLabel,
  ...props
}: TabsTriggerProps) {
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-trigger"
      className={cn(
        "relative inline-flex h-full items-center gap-1.5 px-2 text-body font-medium tracking-body whitespace-nowrap text-muted-foreground transition-colors duration-base ease-out hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start has-data-[icon=inline-end]:pr-1 has-data-[icon=inline-start]:pl-1 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
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
          data-pending={count === "pending" || undefined}
          className="inline-flex items-center font-normal tabular-nums text-subtle"
        >
          {count === "pending" ? (
            <SkeletonText style={{ width: `${Math.max(1, pendingCountDigits)}ch` }} />
          ) : (
            count
          )}
        </span>
      ) : null}
      {liveLabel ? (
        <span
          aria-live="polite"
          data-slot="tabs-trigger-live"
          className="eyebrow inline-flex h-4 items-center gap-1 rounded-pill bg-success-tint px-1.5 text-success"
        >
          <span aria-hidden="true" className="size-1.5 rounded-full bg-success" />
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
