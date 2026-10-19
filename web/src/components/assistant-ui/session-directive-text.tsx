import { use, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { SubagentNavigationContext } from "@/systems/session/contexts/session-subagents-context-value";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";

import { SessionMessageText } from "./session-message-parts";
import { SessionMentionChip } from "./session-composer-mention-chip";
import type {
  SessionMentionDirective,
  SessionSkillInvocationDirective,
} from "./session-directive-registry";

function DirectiveToken({ directive }: { directive: SessionSkillInvocationDirective }) {
  return (
    <span
      data-testid="session-skill-directive"
      data-command-id={directive.commandId}
      data-raw-token={directive.token}
      aria-label={directive.label ?? directive.token}
      className={cn(
        "inline-flex max-w-full align-baseline rounded-xs border border-line px-1 py-px",
        "font-mono text-form-label text-info"
      )}
    >
      {directive.token}
    </span>
  );
}

/**
 * A verified mention in a sent message (S4). File and folder chips only show
 * their path; a session chip opens that session through the window's
 * navigation, where one exists.
 */
function MentionToken({ mention }: { mention: SessionMentionDirective }) {
  const navigation = use(SubagentNavigationContext);
  const workspaceId = useSessionRuntimeRenderContext()?.workspaceId ?? "";
  const onOpen =
    mention.kind === "session" && navigation
      ? (options: { newWindow: boolean }) =>
          navigation.openSession({ sessionId: mention.target, agentName: "", workspaceId }, options)
      : undefined;
  return (
    <SessionMentionChip
      kind={mention.kind}
      target={mention.target}
      label={mention.label}
      onOpen={onOpen}
    />
  );
}

function MessageTextFragment({ text, streaming }: { text: string; streaming: boolean }) {
  if (text.trim().length === 0) {
    return <>{text}</>;
  }
  const leadingWhitespace = text.match(/^\s*/u)?.[0] ?? "";
  const trailingWhitespace = text.match(/\s*$/u)?.[0] ?? "";
  const content = text.slice(leadingWhitespace.length, text.length - trailingWhitespace.length);

  return (
    <>
      {leadingWhitespace}
      {content ? <SessionMessageText text={content} streaming={streaming} /> : null}
      {trailingWhitespace}
    </>
  );
}

type TextDirective =
  | {
      kind: "skill";
      start: number;
      end: number;
      token: string;
      skill: SessionSkillInvocationDirective;
    }
  | {
      kind: "mention";
      start: number;
      end: number;
      token: string;
      mention: SessionMentionDirective;
    };

const NO_MENTIONS: readonly SessionMentionDirective[] = [];

/** CompozyOS-owned directive registry for persisted session invocation metadata. */
export function SessionDirectiveText({
  text,
  directives,
  mentions = NO_MENTIONS,
  streaming = false,
}: {
  text: string;
  directives: readonly SessionSkillInvocationDirective[];
  /** Daemon-verified `@` mentions; unverified tokens stay text. */
  mentions?: readonly SessionMentionDirective[];
  streaming?: boolean;
}) {
  if (directives.length === 0 && mentions.length === 0) {
    return <SessionMessageText text={text} streaming={streaming} />;
  }

  const candidates: TextDirective[] = [
    ...directives.map(skill => ({
      kind: "skill" as const,
      start: skill.start,
      end: skill.end,
      token: skill.token,
      skill,
    })),
    ...mentions.map(mention => ({
      kind: "mention" as const,
      start: mention.start,
      end: mention.end,
      token: mention.token,
      mention,
    })),
  ];
  const ordered: TextDirective[] = [];
  for (const directive of candidates.sort((a, b) => a.start - b.start || a.end - b.end)) {
    const previous = ordered.at(-1);
    if (
      directive.end > text.length ||
      text.slice(directive.start, directive.end) !== directive.token ||
      (previous && directive.start < previous.end)
    ) {
      continue;
    }
    ordered.push(directive);
  }
  if (ordered.length === 0) {
    return <SessionMessageText text={text} streaming={streaming} />;
  }

  const parts: ReactNode[] = [];
  let cursor = 0;
  for (const directive of ordered) {
    const fragment = text.slice(cursor, directive.start);
    if (fragment) {
      parts.push(
        <MessageTextFragment
          key={`text:${cursor}:${directive.start}`}
          text={fragment}
          streaming={streaming}
        />
      );
    }
    parts.push(
      directive.kind === "skill" ? (
        <DirectiveToken
          key={`${directive.skill.commandId}:${directive.start}:${directive.end}`}
          directive={directive.skill}
        />
      ) : (
        <MentionToken
          key={`mention:${directive.start}:${directive.end}`}
          mention={directive.mention}
        />
      )
    );
    cursor = directive.end;
  }
  const remainder = text.slice(cursor);
  if (remainder) {
    parts.push(
      <MessageTextFragment
        key={`text:${cursor}:${text.length}`}
        text={remainder}
        streaming={streaming}
      />
    );
  }

  return <>{parts}</>;
}
