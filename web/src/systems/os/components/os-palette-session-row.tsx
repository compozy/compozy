import { Time } from "@compozy/ui";

import { cn } from "@/lib/utils";
import {
  getSessionDisplayTitle,
  SessionBadgeGlyph,
  sessionBadgeSignal,
  sessionBadgeWordClass,
  type SessionPayload,
} from "@/systems/session";

import { attentionOrderKey } from "../lib/attention-order";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface OsPaletteSessionRowProps {
  /**
   * The session's profile, supplied only under the aggregate lens — the
   * palette speaks the same owner vocabulary as the listing rows.
   */
  owner?: ProfileOwner;
  session: SessionPayload;
  /** The workspace name, shown only once the list reaches past this workspace. */
  workspaceLabel?: string;
}

/**
 * One session inside the palette Sessions view: the 18 px roundel, the title,
 * the exact state word beside its agent, and how long ago that state changed.
 *
 * The state word is always spelled out in the daemon's own vocabulary, so the
 * roundel's tone is never the only thing carrying the state.
 */
export function OsPaletteSessionRow({ session, owner, workspaceLabel }: OsPaletteSessionRowProps) {
  const signal = sessionBadgeSignal(session.badge);
  const agentName = session.agent_name?.trim() ?? "";
  return (
    <>
      <SessionBadgeGlyph badge={session.badge} />
      <span className="flex min-w-0 flex-col gap-1">
        <span className="truncate text-small-body leading-none font-medium text-fg-strong">
          {getSessionDisplayTitle(session)}
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-micro leading-snug text-subtle">
          <span className={cn("shrink-0", sessionBadgeWordClass(session.badge))}>
            {signal.label}
          </span>
          {agentName === "" ? null : <span className="truncate">{agentName}</span>}
          {workspaceLabel === undefined ? null : (
            <span className="shrink-0 font-mono text-micro text-faint">{workspaceLabel}</span>
          )}
        </span>
      </span>
      {owner ? <ProfileOwnerTag className="ml-auto shrink-0" owner={owner} /> : null}
      <Time
        iso={attentionOrderKey(session)}
        className={cn("shrink-0 font-mono text-micro text-subtle", owner ? "" : "ml-auto")}
      />
    </>
  );
}
