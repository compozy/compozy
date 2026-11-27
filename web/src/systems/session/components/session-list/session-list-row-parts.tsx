import { useContext, type ReactNode } from "react";

import { Pin } from "@compozy/ui/icons";
import { Checkbox, Icon, Time, formatAbsoluteTime } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { ComputerMetaLabel } from "@/systems/computers/components/computer-chip";
import { ProfileOwnerTag } from "@/systems/profiles/components/profile-owner-tag";
import type { ProfileOwner } from "@/systems/profiles/lib/profile-scope";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace/lib/workspace-badge-identity";

import type { useKeyboardFocusToken } from "../../hooks/use-keyboard-focus-token";
import type { useSessionEntryPreload } from "../../hooks/use-session-entry-preload";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import type { SessionRowComputerInfo } from "../../lib/session-list-computer";
import { SessionListPinnedSectionContext } from "../../lib/session-list-pinned";
import { sessionWaitingOn } from "../../lib/session-pull-request";
import type { RowMeta } from "../../lib/session-row-meta";
import { hasRunningDescendant } from "../../lib/session-running";
import {
  sessionCheckboxGesture,
  sessionSelectionGesture,
} from "../../lib/session-selection-gesture";
import { sessionIsUnrestricted } from "../../lib/session-unrestricted";
import type { SessionPayload } from "../../types";
import { SessionBadgeMark } from "../session-badge-mark";
import { SessionUnrestrictedBadge } from "../session-unrestricted-badge";
import {
  SESSION_ROW_END_SLOT_FRAME,
  SESSION_ROW_META_FRAME,
  SESSION_ROW_TARGET_FRAME,
  SESSION_ROW_TRAILING_FRAME,
  SESSION_ROW_WHO_FRAME,
} from "./session-list-row-frame";
import { SessionListRowWho } from "./session-list-row-meta";

/** Line 1's title target; its `::after` stretches over the row so the whole row navigates. */
export function SessionRowTarget({
  session,
  title,
  meta,
  emphasized,
  current,
  selected,
  selection,
  badgeMark,
  testId,
  onClick,
  focusHandlers,
  preloadHandlers,
}: {
  session: SessionPayload;
  title: string;
  meta: RowMeta;
  emphasized: boolean;
  current: boolean;
  selected: boolean;
  selection: SessionRowSelection | undefined;
  /** Line 1's leading status mark. */
  badgeMark: ReactNode;
  testId: string;
  onClick: (event: React.MouseEvent<HTMLElement>) => void;
  focusHandlers: ReturnType<typeof useKeyboardFocusToken>["focusHandlers"];
  preloadHandlers: ReturnType<typeof useSessionEntryPreload>;
}) {
  const inPinnedSection = useContext(SessionListPinnedSectionContext);
  const pinnedMark = Boolean(session.pinned_at) && !inPinnedSection;
  return (
    <button
      type="button"
      className={cn(
        SESSION_ROW_TARGET_FRAME,
        "outline-none after:absolute after:inset-0 after:rounded-md",
        emphasized && "font-medium"
      )}
      data-slot="session-row-target"
      data-sidebar-stop="session"
      data-session-id={session.id}
      title={[formatAbsoluteTime(session.updated_at), meta.reason?.detail]
        .filter(Boolean)
        .join("\n")}
      data-status={session.badge}
      data-testid={testId}
      aria-current={current ? "true" : undefined}
      aria-pressed={selection?.mode ? selected : undefined}
      data-selected={selected || undefined}
      onClick={onClick}
      {...preloadHandlers}
      onFocus={event => {
        focusHandlers.onFocus(event);
        preloadHandlers.onFocus();
      }}
      onBlur={() => {
        focusHandlers.onBlur();
        preloadHandlers.onBlur();
      }}
    >
      {badgeMark}
      {pinnedMark ? (
        <span className="flex min-w-0 items-center gap-1">
          <span className="min-w-0 truncate">{title}</span>
          <Icon as={Pin} size="xs" className="shrink-0 text-faint" aria-hidden="true" />
          <span className="sr-only">, pinned</span>
        </span>
      ) : (
        <span className="min-w-0 truncate">{title}</span>
      )}
      <span className="sr-only">{meta.spoken}</span>
    </button>
  );
}

/**
 * Line 2: where and who (project badge, owner, agent face, provider mark), its
 * phrase, and the trailing cluster. The project name gives way before the phrase.
 */
export function SessionRowMetaLine({
  session,
  model,
  nextModel,
  badge,
  computer,
  owner,
  meta,
  mark,
  trailing,
}: {
  session: SessionPayload;
  model: string;
  nextModel: string;
  badge: WorkspaceBadgeIdentity | undefined;
  computer: SessionRowComputerInfo | undefined;
  owner: ProfileOwner | undefined;
  meta: RowMeta;
  mark: ReactNode;
  trailing: ReactNode;
}) {
  return (
    <span className={SESSION_ROW_META_FRAME}>
      <span className={SESSION_ROW_WHO_FRAME}>
        {badge ? (
          <WorkspaceBadge
            identity={badge}
            className="max-w-40 shrink-[1000]"
            data-testid={`session-row-project-${session.id}`}
          />
        ) : null}
        {computer ? (
          <ComputerMetaLabel
            iconKind={computer.iconKind}
            name={computer.name}
            offline={computer.offline}
            platform={computer.platform}
          />
        ) : null}
        {owner ? <ProfileOwnerTag compact size="xs" owner={owner} /> : null}
        {mark}
        <SessionListRowWho model={model} nextModel={nextModel} session={session} meta={meta} />
        {sessionIsUnrestricted(session) ? (
          <SessionUnrestrictedBadge data-testid={`session-row-unrestricted-${session.id}`} />
        ) : null}
      </span>
      {trailing}
    </span>
  );
}

/** Line 1's end slot: the compact time at rest, the row actions on hover or focus. */
export function SessionRowEndSlot({
  session,
  actions,
}: {
  session: SessionPayload;
  actions: ReactNode;
}) {
  return (
    <span className={SESSION_ROW_END_SLOT_FRAME}>
      <Time
        iso={session.updated_at}
        mode="compact"
        className={cn(
          "font-mono text-micro text-subtle transition-opacity duration-fast motion-reduce:transition-none",
          actions !== null &&
            "group-focus-within/session-row:opacity-0 group-hover/session-row:opacity-0 group-has-data-popup-open/session-row:opacity-0"
        )}
      />
      {actions !== null ? (
        <span className="relative z-10 -my-1 opacity-0 transition-opacity duration-fast group-focus-within/session-row:opacity-100 group-hover/session-row:opacity-100 group-has-data-popup-open/session-row:opacity-100 motion-reduce:transition-none">
          {actions}
        </span>
      ) : null}
    </span>
  );
}

/** The meta line's trailing cluster, lifted above the row target; none without controls. */
export function SessionRowTrailing({ children }: { children: ReactNode }) {
  if (children === undefined) return null;
  return <span className={SESSION_ROW_TRAILING_FRAME}>{children}</span>;
}

/** Line 1's status mark; running work below lifts it (#1926), the row's own subagents by default. */
export function SessionRowBadgeMark({
  session,
  selection,
  descendantsRunning = hasRunningDescendant(session, []),
}: {
  session: SessionPayload;
  selection: SessionRowSelection | undefined;
  descendantsRunning?: boolean;
}) {
  return (
    <SessionBadgeMark
      badge={session.badge}
      waitingOn={sessionWaitingOn(session)}
      descendantsRunning={descendantsRunning}
      className={badgeMarkClass(selection)}
    />
  );
}

function badgeMarkClass(selection: SessionRowSelection | undefined): string | undefined {
  if (!selection) return undefined;
  return cn(
    "transition-opacity duration-fast motion-reduce:transition-none group-hover/session-row:opacity-0 group-focus-within/session-row:opacity-0",
    selection.mode && "opacity-0"
  );
}

export function activateSessionRow(
  event: React.MouseEvent<HTMLElement>,
  sessionId: string,
  selection: SessionRowSelection | undefined,
  onSelect: () => void
) {
  const gesture = selection ? sessionSelectionGesture(event, selection.mode) : "open";
  if (gesture === "range") selection?.toggleRange(sessionId);
  else if (gesture === "toggle") selection?.toggle(sessionId);
  else onSelect();
}

export function SessionRowCheckbox({
  title,
  sessionId,
  selected,
  selection,
}: {
  title: string;
  sessionId: string;
  selected: boolean;
  selection: SessionRowSelection;
}) {
  return (
    <Checkbox
      aria-label={`Select ${title}`}
      checked={selected}
      className={cn(
        "absolute top-2 left-1 z-10 transition-opacity duration-fast motion-reduce:transition-none after:inset-0",
        selection.mode
          ? "opacity-100"
          : "pointer-events-none opacity-0 group-hover/session-row:pointer-events-auto group-hover/session-row:opacity-100 group-focus-within/session-row:pointer-events-auto group-focus-within/session-row:opacity-100"
      )}
      onCheckedChange={(_checked, details) => {
        if (sessionCheckboxGesture(details.event) === "range") selection.toggleRange(sessionId);
        else selection.toggle(sessionId);
      }}
    />
  );
}
