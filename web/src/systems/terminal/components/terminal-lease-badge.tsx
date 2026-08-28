import { Eye, Keyboard } from "lucide-react";

import { cn, OwnerAvatar, Pill } from "@compozy/ui";

import { terminalLeaseDataAttr } from "../lib/terminal-lease-attr";
import type { TerminalLeaseView } from "../lib/terminal-lease";

export interface TerminalLeaseBadgeProps {
  view: TerminalLeaseView;
  /** How many people are watching. Absent when the count is unknown. */
  viewers?: number;
  className?: string;
}

/**
 * Who holds the terminal, in words.
 *
 * The cursor carries the same fact visually, but it is never the only carrier:
 * this chip names the controller in text so the state survives a screen reader,
 * a still frame, and a colour-blind reader alike.
 */
export function TerminalLeaseBadge({ view, viewers, className }: TerminalLeaseBadgeProps) {
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <Pill
        data-lease={terminalLeaseDataAttr(view.read)}
        data-testid="terminal-lease-badge"
        size="sm"
        tone={view.read === "you" || view.read === "nobody" ? "neutral" : "info"}
      >
        {view.read === "agent" && view.controllerName ? (
          <OwnerAvatar
            name={view.controllerName}
            ownerId={view.controllerName}
            ownerKind="agent"
            size="sm"
          />
        ) : view.read === "you" ? (
          <Keyboard aria-hidden="true" className="size-3" />
        ) : null}
        {/* Only the sentence is a live region. Control changing hands is worth
            announcing; the glyph, the chip and the viewer count around it are
            not, and marking the whole row live would repeat all of it on every
            render. */}
        <span aria-live="polite" data-testid="terminal-lease-label" role="status">
          {view.label}
        </span>
      </Pill>
      {viewers === undefined ? null : (
        <Pill
          aria-label={`${viewers} ${viewers === 1 ? "viewer" : "viewers"}`}
          data-testid="terminal-viewers"
          mono
          size="sm"
          tone="info"
        >
          <Eye aria-hidden="true" className="size-3" />
          {viewers}
        </Pill>
      )}
    </span>
  );
}
