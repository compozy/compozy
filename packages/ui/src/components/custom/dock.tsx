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
 * Decision card above the composer — permission and clarification surfaces
 * leave the transcript and dock here. The approval-card anatomy: a standalone
 * `card` surface with `shadow-card`, an identity-well head, a sunken subject
 * block and a pill action row. The root owns the vertical rhythm (`gap-3`), so
 * direct children drop their own top margins.
 */
function DockRoot({ children, className, ...props }: DockProps) {
  return (
    <div
      data-slot="dock"
      className={cn("flex flex-col gap-3 rounded-lg bg-card p-3.5 shadow-card *:mt-0", className)}
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
