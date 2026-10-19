import {
  ComposerPrimitive,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import { File, Folder, MessagesSquare } from "lucide-react";
import { Fragment, useState } from "react";

import { formatRelativeTime } from "@compozy/ui";

import {
  pushSessionMentionRecent,
  readSessionMentionRecents,
  useSessionFileSearch,
  useSessionMentionSessions,
  type SessionMentionRecent,
} from "@/systems/session";

import {
  TriggerMenuDivider,
  TriggerMenuFooter,
  TriggerMenuGroupHeader,
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
          <mark key={index} className="bg-transparent font-semibold text-fg-strong">
            {segment.text}
          </mark>
        ) : (
          <Fragment key={index}>{segment.text}</Fragment>
        )
      )}
    </>
  );
}

function MentionMenuRow({
  item,
  flatIndex,
  query,
}: {
  item: Unstable_TriggerItem;
  flatIndex: number;
  query: string;
}) {
  const kind = item.metadata?.kind;
  if (kind === "session") {
    const titled = item.metadata?.titled === true;
    const updatedAt = typeof item.metadata?.updatedAt === "string" ? item.metadata.updatedAt : "";
    return (
      <TriggerMenuRow
        item={item}
        flatIndex={flatIndex}
        icon={MessagesSquare}
        data-testid="composer-mention-item"
        data-mention-kind="session"
      >
        <span
          className={
            titled
              ? "max-w-[70%] truncate font-medium"
              : "max-w-[70%] truncate font-mono font-medium"
          }
        >
          <MatchedText text={item.label} query={query} />
        </span>
        {/* Agent and update time tell same-titled sessions apart (US-003.EC-4). */}
        <TriggerMenuRowSecondary>
          <MatchedText text={item.description ?? ""} query={query} />
          {updatedAt ? ` · ${formatRelativeTime(updatedAt)}` : null}
        </TriggerMenuRowSecondary>
      </TriggerMenuRow>
    );
  }
  const nameMatches = emphasizeMatch(item.label, query).some(segment => segment.match);
  return (
    <TriggerMenuRow
      item={item}
      flatIndex={flatIndex}
      icon={kind === "folder" ? Folder : File}
      data-testid="composer-mention-item"
      data-mention-kind={kind === "folder" ? "folder" : "file"}
    >
      <span className="max-w-[70%] truncate font-medium">
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

function MentionMenuGroupView({ group, query }: { group: MentionMenuGroup; query: string }) {
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
          />
        ))
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
          <MentionMenuGroupView group={group} query={query} />
        </Fragment>
      ))}
    </div>
  );
}

/**
 * The open listbox renders inside assistant-ui's popover. A settled picker
 * with nothing to pick closes the listbox — so Enter sends the prompt as typed
 * (US-004.AC-3) — and keeps showing each group's message on the same surface.
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

function MentionDirectiveBehavior({
  onInserted,
}: {
  onInserted: (item: Unstable_TriggerItem) => void;
}) {
  const { close } = useTriggerPopoverScopeContext();
  return (
    <ComposerPrimitive.Unstable_TriggerPopover.Directive
      formatter={mentionDirectiveFormatter}
      onInserted={item => {
        onInserted(item);
        close();
      }}
    />
  );
}

const NO_CATEGORIES: never[] = [];

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
}: {
  workspaceId: string;
  sessionId: string;
  /** The live, undismissed `@` occurrence; `null` keeps the picker closed. */
  trigger: SessionMentionTrigger | null;
}) {
  const active = trigger !== null && workspaceId !== "";
  const query = trigger?.query ?? "";
  const [recents, setRecents] = useState<readonly SessionMentionRecent[]>(() =>
    readSessionMentionRecents(workspaceId)
  );
  const files = useSessionFileSearch({ workspaceId, sessionId, query, enabled: active });
  const sessions = useSessionMentionSessions({ workspaceId, sessionId, query, enabled: active });
  const model = buildMentionMenuModel({ query, files, sessions, recents });
  const listboxOpen = active && (model.items.length > 0 || model.loading);
  const adapter = listboxOpen
    ? {
        categories: () => NO_CATEGORIES,
        categoryItems: () => NO_CATEGORIES,
        search: () => model.items,
      }
    : undefined;

  const rememberPick = (item: Unstable_TriggerItem) => {
    const kind = item.metadata?.kind;
    const target = item.metadata?.target;
    if ((kind !== "file" && kind !== "folder") || typeof target !== "string") return;
    setRecents(pushSessionMentionRecent(workspaceId, { path: target, kind }));
  };

  return (
    <ComposerPrimitive.Unstable_TriggerPopover
      char={MENTION_TRIGGER_CHARACTER}
      adapter={adapter}
      isLoading={model.loading}
      aria-label="Mentions"
      data-testid="composer-mention-menu"
      className={TRIGGER_MENU_POPOVER_CLASS}
    >
      <MentionDirectiveBehavior onInserted={rememberPick} />
      <MentionMenuBody model={model} query={query} visible={active} />
    </ComposerPrimitive.Unstable_TriggerPopover>
  );
}
