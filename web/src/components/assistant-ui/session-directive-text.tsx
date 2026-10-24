import { use, type ClipboardEvent } from "react";

import { composeMarkdownSlots, type StreamMarkdownSlot } from "@compozy/ui";

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
      token={mention.token}
    />
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
const MENTION_TOKEN_SELECTOR = "[data-mention-token]";

/** Directives whose token sits exactly at its offsets, in text order, without overlaps. */
function verifiedDirectives(
  text: string,
  directives: readonly SessionSkillInvocationDirective[],
  mentions: readonly SessionMentionDirective[]
): TextDirective[] {
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
  return ordered;
}

/**
 * Copying a selection that crosses a mention chip yields the chip's raw token,
 * so pasted text still mentions the same target. Selections without chips
 * keep the browser's default copy.
 */
function copySelectionWithMentionTokens(event: ClipboardEvent<HTMLElement>) {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) return;
  const fragment = selection.getRangeAt(0).cloneContents();
  const chips = fragment.querySelectorAll<HTMLElement>(MENTION_TOKEN_SELECTOR);
  if (chips.length === 0) return;
  for (const chip of chips) {
    chip.replaceWith(chip.dataset.mentionToken ?? "");
  }
  // innerText keeps the line breaks between blocks, but only for rendered nodes.
  const host = document.createElement("div");
  host.style.cssText = "position:fixed;left:-10000px;top:0;white-space:pre-wrap";
  host.append(fragment);
  document.body.append(host);
  const plain = typeof host.innerText === "string" ? host.innerText : (host.textContent ?? "");
  const html = host.innerHTML;
  host.remove();
  event.clipboardData.setData("text/plain", plain);
  event.clipboardData.setData("text/html", html);
  event.preventDefault();
}

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
  const ordered =
    directives.length === 0 && mentions.length === 0
      ? []
      : verifiedDirectives(text, directives, mentions);
  if (ordered.length === 0) {
    return <SessionMessageText text={text} streaming={streaming} authored />;
  }

  // Each verified token becomes an inline slot of one markdown document, so the
  // markdown around a chip (bold, lists, headings) forms as written (VC-18).
  // A token the markdown renders inside code, a link label, or raw HTML reads
  // as its literal text there.
  const slots: Record<string, StreamMarkdownSlot> = {};
  const placements = ordered.map((directive, index) => {
    const id = `compozy-directive-${index}`;
    slots[id] = {
      node:
        directive.kind === "skill" ? (
          <DirectiveToken directive={directive.skill} />
        ) : (
          <MentionToken mention={directive.mention} />
        ),
      text: directive.token,
    };
    return { id, start: directive.start, end: directive.end };
  });
  const source = composeMarkdownSlots(text, placements);

  return (
    <div className="contents" onCopy={copySelectionWithMentionTokens}>
      <SessionMessageText text={source} streaming={streaming} slots={slots} authored />
    </div>
  );
}
