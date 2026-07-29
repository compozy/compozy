"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { DockActions } from "./dock-actions";
import { DockBody } from "./dock-body";
import { DockCount } from "./dock-count";
import { DockDeadline } from "./dock-deadline";
import { DockEyebrow } from "./dock-eyebrow";
import { DockHead } from "./dock-head";
import { DockKey } from "./dock-key";
import { DockMeta } from "./dock-meta";
import { DockPre } from "./dock-pre";
import { DockStatus, type DockStatusProps } from "./dock-status";
import { DockTitle } from "./dock-title";

export interface DockProps extends React.ComponentProps<"div"> {
  children: React.ReactNode;
}

/**
 * Decision panel fused to the composer top — permission and clarification
 * surfaces leave the transcript and dock here. The shell shares its border
 * with the composer below (open bottom, top radius only); emphasis comes from
 * geometry and the eyebrow, never from a tinted wash.
 */
function DockRoot({ children, className, ...props }: DockProps) {
  return (
    <div
      data-slot="dock"
      className={cn(
        "rounded-t-lg border border-b-0 border-line-strong bg-canvas-soft",
        "px-3.5 pt-[11px] pb-3",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

const Dock = Object.assign(DockRoot, {
  Head: DockHead,
  Eyebrow: DockEyebrow,
  Title: DockTitle,
  Count: DockCount,
  Deadline: DockDeadline,
  Body: DockBody,
  Pre: DockPre,
  Meta: DockMeta,
  Actions: DockActions,
  Status: DockStatus,
  Key: DockKey,
});

export { Dock, type DockStatusProps };
