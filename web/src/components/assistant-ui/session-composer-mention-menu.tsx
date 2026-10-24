import {
  ComposerPrimitive,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import { File, Folder, MessagesSquare } from "lucide-react";
import { Fragment, useState } from "react";

import { formatRelativeTime } from "@compozy/ui";

import {
  useSessionFileSearch,
  useSessionMentionSessions,
  type SessionMentionRecent,
} from "@/systems/session";

import {
  TriggerMenuDivider,
  TriggerMenuFooter,
  TriggerMenuGroupHeader,
  TriggerMenuLoading,
  TriggerMenuMessage,
  TriggerMenuRow,
  TriggerMenuRowSecondary,
} from "./session-composer-trigger-menu";
import {
  TRIGGER_MENU_ITEMS_CLASS,
  TRIGGER_MENU_POPOVER_CLASS,
} from "./session-composer-trigger-menu-classes";
import {
  buildMentionMenuModel,
  emphasizeMatch,
  MENTION_TRIGGER_CHARACTER,
  mentionDirectiveFormatter,
  type MentionMenuGroup,
  type MentionMenuModel,
  type SessionMentionTrigger,
} from "./session-mention-model";

const TRUNCATED_FOOTER = "Showing results from the first 25,000 files";

function MatchedText({ text, query }: { text: string; query: string }) {
  return (
    <>
      {emphasizeMatch(text, query).map((segment, index) =>
        segment.match ? (
          <mark key={index} className="bg-transparent font-[650] text-fg-strong">
            {segment.text}
          </mark>
        ) : (
          <Fragment key={index}>{segment.text}</Fragment>
        )
      )}
    </>
  );
}

interface MentionRowProps {
  item: Unstable_TriggerItem;
  flatIndex: number;
  query: string;
  /** False while the row answers an earlier query: shown and clickable, never highlighted. */
  highlightable: boolean;
}

function SessionMentionRow({ item, flatIndex, query, highlightable }: MentionRowProps) {
  const titled = item.metadata?.titled === true;
  const updatedAt = typeof item.metadata?.updatedAt === "string" ? item.metadata.updatedAt : "";
  const agent = item.description ?? "";
  const updated = updatedAt ? formatRelativeTime(updatedAt) : "";
  return (
    <TriggerMenuRow
      item={item}
      flatIndex={flatIndex}
      highlightable={highlightable}
      icon={MessagesSquare}
      data-testid="composer-mention-item"
      data-mention-kind="session"
    >
      <span
        className={
          titled
            ? "max-w-[70%] shrink-0 truncate font-medium"
            : "max-w-[70%] shrink-0 truncate font-mono text-meta font-medium"
        }
      >
        <MatchedText text={item.label} query={query} />
      </span>
      {/* Agent and update time tell same-titled sessions apart (US-003.EC-4);
          the separator only joins two present parts. */}
      {agent || updated ? (
        <TriggerMenuRowSecondary>
          <MatchedText text={agent} query={query} />
          {agent && updated ? " · " : null}
          {updated}
        </TriggerMenuRowSecondary>
      ) : null}
    </TriggerMenuRow>
  );
}

function PathMentionRow({ item, flatIndex, query, highlightable }: MentionRowProps) {
  const folder = item.metadata?.kind === "folder";
  // The match is emphasized where it hit: the name first, else the parent path.
  const nameMatches = emphasizeMatch(item.label, query).some(segment => segment.match);
  return (
    <TriggerMenuRow
      item={item}
      flatIndex={flatIndex}
      highlightable={highlightable}
      icon={folder ? Folder : File}
      data-testid="composer-mention-item"
      data-mention-kind={folder ? "folder" : "file"}
    >
      <span className="max-w-[70%] shrink-0 truncate font-medium">
        <MatchedText text={item.label} query={nameMatches ? query : ""} />
      </span>
      {item.description ? (
        <TriggerMenuRowSecondary>
          <MatchedText text={item.description} query={nameMatches ? "" : query} />
        </TriggerMenuRowSecondary>
      ) : null}
    </TriggerMenuRow>
  );
}

function MentionMenuRow(props: MentionRowProps) {
  return props.item.metadata?.kind === "session" ? (
    <SessionMentionRow {...props} />
  ) : (
    <PathMentionRow {...props} />
  );
}

function MentionMenuGroupView({
  group,
  query,
  highlightable,
}: {
  group: MentionMenuGroup;
  query: string;
  highlightable: boolean;
}) {
  return (
    <div role="group" aria-label={group.label} data-testid={`composer-mention-group-${group.id}`}>
      <TriggerMenuGroupHeader label={group.label} />
      {group.message === null ? (
        group.rows.map(row => (
          <MentionMenuRow
            key={row.item.id}
            item={row.item}
            flatIndex={row.flatIndex}
            query={query}
            highlightable={highlightable}
          />
        ))
      ) : group.status === "loading" ? (
        // Its first answer is on its way: placeholder rows, the loading copy announced.
        <TriggerMenuLoading data-testid="composer-mention-message" rows={2}>
          {group.message}
        </TriggerMenuLoading>
      ) : (
        <TriggerMenuMessage data-testid="composer-mention-message">
          {group.message}
        </TriggerMenuMessage>
      )}
      {group.truncated ? <TriggerMenuFooter>{TRUNCATED_FOOTER}</TriggerMenuFooter> : null}
    </div>
  );
}

function MentionMenuGroups({ model, query }: { model: MentionMenuModel; query: string }) {
  return (
    <div className={TRIGGER_MENU_ITEMS_CLASS}>
      {model.groups.map((group, index) => (
        <Fragment key={group.id}>
          {index > 0 ? <TriggerMenuDivider /> : null}
          <MentionMenuGroupView group={group} query={query} highlightable={!model.stale} />
        </Fragment>
      ))}
    </div>
  );
}

/**
 * The open listbox renders inside assistant-ui's popover. A picker with
 * nothing to pick — still loading its first answer, empty, or failed — keeps
 * the listbox closed so Enter sends the prompt as typed (US-004.AC-3), and
 * shows each group's loading, empty or error row on the same surface.
 */
function MentionMenuBody({
  model,
  query,
  visible,
}: {
  model: MentionMenuModel;
  query: string;
  visible: boolean;
}) {
  const { open } = useTriggerPopoverScopeContext();
  if (open) return <MentionMenuGroups model={model} query={query} />;
  if (!visible) return null;
  return (
    <div
      role="status"
      aria-label="Mentions"
      data-testid="composer-mention-menu"
      data-state="empty"
      className={TRIGGER_MENU_POPOVER_CLASS}
    >
      <MentionMenuGroups model={model} query={query} />
    </div>
  );
}

/**
 * Inserts the picked target as a chip. Picks reach the composer's directive
 * select handler, which records file and folder recents.
 */
function MentionDirectiveBehavior() {
  const { close } = useTriggerPopoverScopeContext();
  return (
    <ComposerPrimitive.Unstable_TriggerPopover.Directive
      formatter={mentionDirectiveFormatter}
      onInserted={close}
    />
  );
}

const NO_CATEGORIES: never[] = [];

function sameMentionItems(
  left: readonly Unstable_TriggerItem[],
  right: readonly Unstable_TriggerItem[]
): boolean {
  return (
    left.length === right.length &&
    left.every(
      (item, index) =>
        item.id === right[index]?.id &&
        item.label === right[index]?.label &&
        item.description === right[index]?.description
    )
  );
}

/**
 * The `@` picker (S1): a second trigger beside `/` with "Files" (files and
 * folders under the session's working directory, recents first for a bare
 * `@`) and "Sessions" (same workspace, current session excluded). The editor
 * reports the live `@` occurrence; assistant-ui owns the listbox, keyboard,
 * ARIA and chip insertion.
 */
export function SessionComposerMentionMenu({
  workspaceId,
  sessionId,
  trigger,
  recents,
}: {
  workspaceId: string;
  sessionId: string;
  /** The live, undismissed `@` occurrence; `null` keeps the picker closed. */
  trigger: SessionMentionTrigger | null;
  /** This workspace's recent file/folder mentions, shown first for a bare `@`. */
  recents: readonly SessionMentionRecent[];
}) {
  const active = trigger !== null && workspaceId !== "";
  const query = trigger?.query ?? "";
  const files = useSessionFileSearch({ workspaceId, sessionId, query, enabled: active });
  const sessions = useSessionMentionSessions({ workspaceId, sessionId, query, enabled: active });
  const model = buildMentionMenuModel({ query, files, sessions, recents });
  // Only rows open the listbox: an open listbox with no row would swallow
  // Enter during the first load (US-004.AC-3, VC-04). Stale rows keep it open
  // with nothing highlightable, so Enter waits for the current query's rows
  // instead of sending the prompt or picking a row from an earlier query.
  const listboxOpen = active && model.rowCount > 0;
  // assistant-ui resets the highlight whenever the item list changes identity,
  // so an unrelated re-render (a group settling, a catalog refresh) keeps the
  // same list while its rows are unchanged and the arrow position holds.
  const [items, setItems] = useState(model.items);
  if (!sameMentionItems(items, model.items)) setItems(model.items);
  const adapter = listboxOpen
    ? {
        categories: () => NO_CATEGORIES,
        categoryItems: () => NO_CATEGORIES,
        search: () => items,
      }
    : undefined;

  return (
    <ComposerPrimitive.Unstable_TriggerPopover
      char={MENTION_TRIGGER_CHARACTER}
      adapter={adapter}
      isLoading={model.loading}
      aria-label="Mentions"
      aria-busy={model.stale || undefined}
      data-testid="composer-mention-menu"
      className={TRIGGER_MENU_POPOVER_CLASS}
    >
      <MentionDirectiveBehavior />
      <MentionMenuBody model={model} query={query} visible={active} />
    </ComposerPrimitive.Unstable_TriggerPopover>
  );
}
