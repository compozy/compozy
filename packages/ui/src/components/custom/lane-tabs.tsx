"use client";

import * as React from "react";

import { Tabs, TabsList, TabsTrigger } from "../tabs";

export interface LaneTabsItem<T extends string> {
  value: T;
  label: React.ReactNode;
  count?: number;
  liveLabel?: React.ReactNode;
  testId?: string;
}

export interface LaneTabsProps<T extends string> extends Omit<
  React.ComponentProps<"div">,
  "onChange"
> {
  items: ReadonlyArray<LaneTabsItem<T>>;
  value: T;
  onChange: (next: T) => void;
  ariaLabel?: string;
  listClassName?: string;
}

function LaneTabs<T extends string>({
  items,
  value,
  onChange,
  ariaLabel,
  listClassName,
  className,
  children,
  ...props
}: LaneTabsProps<T>) {
  return (
    <Tabs
      data-slot="lane-tabs"
      value={value}
      onValueChange={next => onChange(next as T)}
      className={className}
      {...props}
    >
      <TabsList aria-label={ariaLabel} className={listClassName}>
        {items.map(item => (
          <TabsTrigger
            key={item.value}
            value={item.value}
            count={item.count}
            liveLabel={item.liveLabel}
            data-testid={item.testId}
          >
            {item.label}
          </TabsTrigger>
        ))}
      </TabsList>
      {children}
    </Tabs>
  );
}

export { LaneTabs };
