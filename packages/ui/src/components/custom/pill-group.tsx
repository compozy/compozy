"use client";

import type { VariantProps } from "class-variance-authority";
import * as React from "react";

import { cn } from "../../lib/utils";
import { pillGroupSegmentVariants } from "./pill-group-variants";

export type PillGroupSize = NonNullable<VariantProps<typeof pillGroupSegmentVariants>["size"]>;

export interface PillGroupItem<V extends string = string> {
  value: V;
  label: React.ReactNode;
  /** Optional unread / count badge rendered inside the segment. */
  badge?: number;
  disabled?: boolean;
  testId?: string;
}

export interface PillGroupProps<V extends string = string> extends Omit<
  React.ComponentProps<"div">,
  "onChange"
> {
  items: ReadonlyArray<PillGroupItem<V>>;
  value: V;
  onChange: (next: V) => void;
  size?: PillGroupSize;
}

function PillGroup<V extends string = string>({
  items,
  value,
  onChange,
  size = "md",
  className,
  ...props
}: PillGroupProps<V>) {
  return (
    <div
      data-slot="pill-group"
      role="group"
      className={cn(
        "inline-flex items-center gap-(--space-pill-group-track-gap) rounded-md bg-canvas-soft p-(--space-pill-group-track-padding)",
        className
      )}
      {...props}
    >
      {items.map(item => {
        const isActive = item.value === value;
        return (
          <button
            key={item.value}
            type="button"
            data-slot="pill-group-item"
            data-value={item.value}
            data-active={isActive}
            data-testid={item.testId}
            aria-pressed={isActive}
            disabled={item.disabled}
            onClick={() => {
              if (isActive) return;
              onChange(item.value);
            }}
            className={pillGroupSegmentVariants({ active: isActive, size })}
          >
            <span className="inline-flex items-center gap-1.5">{item.label}</span>
            {typeof item.badge === "number" && item.badge > 0 ? (
              <span
                data-slot="pill-group-badge"
                className="inline-flex h-(--size-pill-group-badge) min-w-(--size-pill-group-badge) items-center justify-center rounded-mono-badge bg-badge-fill px-(--space-pill-group-badge-x) text-pill-group-badge font-medium tabular-nums text-muted"
              >
                {item.badge}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

export { PillGroup };
