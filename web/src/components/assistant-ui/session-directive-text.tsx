import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { SessionMessageText } from "./session-message-parts";
import type { SessionSkillInvocationDirective } from "./session-directive-registry";

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

/** CompozyOS-owned directive registry for persisted session invocation metadata. */
export function SessionDirectiveText({
  text,
  directives,
  streaming = false,
}: {
  text: string;
  directives: readonly SessionSkillInvocationDirective[];
  streaming?: boolean;
}) {
  if (directives.length === 0) {
    return <SessionMessageText text={text} streaming={streaming} />;
  }

  const ordered: SessionSkillInvocationDirective[] = [];
  for (const directive of [...directives].sort((a, b) => a.start - b.start || a.end - b.end)) {
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
      <DirectiveToken
        key={`${directive.commandId}:${directive.start}:${directive.end}`}
        directive={directive}
      />
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
