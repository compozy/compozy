import type { MouseEvent } from "react";

import { StateGlyph, cn } from "@compozy/ui";

import { useAgentIdentityView } from "@/systems/agent";

import { useSubagentDrawerSelected } from "../../hooks/use-subagent-drawer-selected";
import { SESSION_TONE_WORD_CLASS } from "../../lib/session-badge-classes";
import { SubagentElapsed } from "../subagents/subagent-elapsed";
import { subagentDisplay, type SubagentDisplay } from "../subagents/subagent-format";
import { subagentRuntimeWords } from "../subagents/subagent-row-format";
import type { SubagentView } from "../subagents/types";
import { BRANCH_NODE } from "./session-list-branch-rail";
import { SessionListAgentMark } from "./session-list-agent-mark";
import { SessionListProviderMark } from "./session-list-provider-mark";
import { SESSION_ROW_MARK_TOOLTIP_DELAY, SUBAGENT_ROW_FRAME } from "./session-list-row-frame";

/** One direct subagent on its parent's rail; `SessionListSubagentBranch` pages and opens them. */
export function SessionListSubagentRow({
  subagent,
  current,
  onOpen,
  workspace,
  profile,
}: {
  subagent: SubagentView;
  current: boolean;
  workspace?: string;
  profile?: string;
  onOpen?: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const selected = useSubagentDrawerSelected(subagent.id);
  const display = subagentDisplay(subagent);
  const word = display.word;
  const identity = useAgentIdentityView(subagent.runtime.agent ?? "", { workspace, profile });
  const runtime = subagentRuntimeWords(subagent, identity.label);
  const body = (
    <>
      {/* The mark sits on the title line, where the branch elbow lands, not between both lines. */}
      <span className="flex h-5 items-center justify-center self-start">
        <StateGlyph size="sm" state={display.glyph} />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="truncate">{subagent.title}</span>
        <SubagentRowWho
          display={display}
          subagent={subagent}
          identity={identity}
          runtime={runtime}
        />
      </span>
      <SubagentElapsed className="self-start leading-5" clock={display.clock} />
    </>
  );
  const rowClass = cn(SUBAGENT_ROW_FRAME, (current || selected) && "bg-selected");

  return (
    <div
      className={BRANCH_NODE}
      data-slot="session-subagent-row"
      data-status={subagent.status}
      data-state={display.glyph}
    >
      {onOpen ? (
        <button
          type="button"
          className={cn(
            rowClass,
            "transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring"
          )}
          aria-label={`Open ${subagent.title} · ${runtime.label}`}
          aria-description={word}
          aria-current={current || selected ? "true" : undefined}
          onClick={onOpen}
        >
          {body}
        </button>
      ) : (
        <div className={rowClass}>
          {body}
          <span className="sr-only">{word}</span>
        </div>
      )}
    </div>
  );
}

/**
 * Line 2 in the session rows' grammar: the agent's face and the provider mark, which name
 * themselves on a resting hover, then the model, or the needs-you words in its place.
 */
function SubagentRowWho({
  subagent,
  display,
  identity,
  runtime,
}: {
  subagent: SubagentView;
  display: SubagentDisplay;
  identity: ReturnType<typeof useAgentIdentityView>;
  runtime: ReturnType<typeof subagentRuntimeWords>;
}) {
  const { agent, provider } = subagent.runtime;
  const { modelLabel, label } = runtime;
  const needsYou = display.tone === "attention" || display.tone === "destructive";
  return (
    <span className="flex min-w-0 items-center gap-1 text-transcript-meta text-muted-foreground">
      {agent ? (
        <SessionListAgentMark
          appearance={identity.appearance}
          label={identity.label}
          sessionId={subagent.id}
        />
      ) : null}
      <SessionListProviderMark
        delay={SESSION_ROW_MARK_TOOLTIP_DELAY}
        runtime={provider ? { provider, label } : null}
        sessionId={subagent.id}
      />
      <span className="sr-only">{label}</span>
      {needsYou ? (
        <NeedsYouWord display={display} />
      ) : modelLabel ? (
        <span aria-hidden="true" className="min-w-0 shrink truncate text-fg-2">
          {modelLabel}
        </span>
      ) : null}
    </span>
  );
}

/** The words a row keeps, in the model's place: the subagent waits for you or failed. */
function NeedsYouWord({ display }: { display: SubagentDisplay }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "min-w-0 truncate",
        SESSION_TONE_WORD_CLASS[display.tone === "attention" ? "brand" : "destructive"]
      )}
    >
      {display.word}
    </span>
  );
}
