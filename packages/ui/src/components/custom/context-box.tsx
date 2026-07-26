"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Eyebrow } from "./eyebrow";

export interface ContextBoxEntry {
  label: React.ReactNode;
  value: React.ReactNode;
}

export interface ContextBoxProps extends Omit<React.ComponentProps<"dl">, "children" | "title"> {
  entries: ReadonlyArray<ContextBoxEntry>;
  /** Optional title rendered above the grid. */
  title?: React.ReactNode;
}

function ContextBox({ entries, title, className, ...props }: ContextBoxProps) {
  return (
    <div data-slot="context-box-root" className={cn("flex flex-col gap-2", className)}>
      {title ? (
        <Eyebrow data-slot="context-box-title" className="text-muted">
          {title}
        </Eyebrow>
      ) : null}
      <dl
        data-slot="context-box"
        className={cn(
          "grid grid-cols-[minmax(0,var(--width-kv-label))_minmax(0,1fr)] gap-x-3 gap-y-1.5 rounded-md border border-line bg-canvas-soft px-3 py-2.5"
        )}
        {...props}
      >
        {entries.map(entry => (
          <React.Fragment key={`${String(entry.label)}-${String(entry.value)}`}>
            <dt data-slot="context-box-label" className="eyebrow text-muted">
              {entry.label}
            </dt>
            <dd data-slot="context-box-value" className="text-form-label text-fg">
              {entry.value}
            </dd>
          </React.Fragment>
        ))}
      </dl>
    </div>
  );
}

export { ContextBox };
