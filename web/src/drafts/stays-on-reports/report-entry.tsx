/**
 * S11: what an agent leaves in its conversation while you're away. Nothing here starts a turn.
 * Two weights: a proposal taps you on the shoulder and asks; a notice says what happened and what
 * you can do. Neither looks like a log.
 */
import { ArrowRight, Check } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Button,
  ChoiceKey,
  ChoiceList,
  ChoiceRow,
  CodeBlock,
  Dock,
  Pill,
  Receipt,
} from "@compozy/ui";

import { NoticeCard } from "@/systems/session/components/stays-on/reports/report-entry";

import { goToState } from "../stays-on-agents-shared/go-to-state";
import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AgentState } from "../stays-on-agents-shared/agent-state";
import type { DraftAgent } from "../stays-on-agents-shared/fixtures";

export interface ProposalChoice {
  title: string;
  body: string;
}

function Byline({ agent, when }: { agent: DraftAgent; when: string }) {
  return (
    <p className="flex items-baseline gap-2">
      <span className="text-body font-medium text-fg">{agent.label}</span>
      <span className="text-eyebrow text-subtle tabular-nums">{when}</span>
    </p>
  );
}

function Face({ agent, state, size }: { agent: DraftAgent; state: AgentState; size: number }) {
  return (
    <AgentAvatar
      appearance={agent.appearance}
      className="mt-0.5"
      label={agent.label}
      provider={agent.provider}
      quietIdle
      size={size}
      state={state}
    />
  );
}

/** The agent asks, in a sentence, and you answer with one click. */
export function ShoulderTap({
  agent,
  when,
  ask,
  choices,
  onChoose,
  chosen,
  footnote,
}: {
  agent: DraftAgent;
  when: string;
  ask: string;
  choices: readonly ProposalChoice[];
  onChoose?: (choice: ProposalChoice) => void;
  /** The title that was picked; the ask folds down to a record of it. */
  chosen?: string;
  footnote?: ReactNode;
}) {
  if (chosen) {
    return (
      <Receipt icon={<Check aria-hidden="true" />} tone="allowed">
        You picked <b>{chosen}</b>
      </Receipt>
    );
  }
  return (
    <article aria-label={`${agent.label} asks`} className="flex gap-4 py-1">
      <Face agent={agent} size={48} state="needs-you" />
      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <div className="flex flex-col gap-1">
          <Byline agent={agent} when={when} />
          <h3 className="text-detail-h1 font-medium text-fg-strong">{ask}</h3>
        </div>
        <ChoiceList className="mt-0 gap-1.5">
          {choices.map((choice, index) => (
            <ChoiceRow
              className="group/choice min-h-0 gap-3 rounded-lg bg-card px-3.5 py-3 shadow-card"
              key={choice.title}
              onClick={() => onChoose?.(choice)}
            >
              <ChoiceKey>{index + 1}</ChoiceKey>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="text-item-title font-medium text-fg-strong">{choice.title}</span>
                <span className="text-small-body text-muted-foreground">{choice.body}</span>
              </span>
              <ArrowRight
                aria-hidden="true"
                className="size-4 shrink-0 text-subtle transition-transform duration-base group-hover/choice:translate-x-0.5 motion-reduce:transition-none"
              />
            </ChoiceRow>
          ))}
        </ChoiceList>
        <p className="text-small-body text-subtle">
          {footnote ?? "Nothing happens until you pick one. You can also just reply below."}
        </p>
      </div>
    </article>
  );
}

/** Something happened that you should know about, and what you can do about it. */
export function Notice({
  agent,
  when,
  state = "idle",
  title,
  body,
  detail,
  actions,
  className,
}: {
  agent: DraftAgent;
  when: string;
  state?: AgentState;
  title: string;
  body: ReactNode;
  /** The evidence: the command, the pattern of check-ins. */
  detail?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <NoticeCard
      actions={actions}
      body={body}
      className={className}
      detail={detail}
      face={<Face agent={agent} size={36} state={state} />}
      title={title}
      when={when}
    />
  );
}

/**
 * A request that ended without an answer, in the live approval's anatomy so it reads as the same
 * thing, settled: who asked, the exact command or file, what came of it, and the way forward.
 */
export function SettledAsk({
  agent,
  title,
  session,
  when,
  outcome,
  why,
  command,
  file,
  actions,
}: {
  agent: DraftAgent;
  title: string;
  /** The session the request came from. */
  session: string;
  when: string;
  outcome: string;
  why: string;
  command?: string;
  file?: string;
  actions: ReactNode;
}) {
  return (
    <Dock aria-label={title} role="article">
      <Dock.Head className="flex-nowrap">
        <AgentAvatar
          appearance={agent.appearance}
          label={agent.label}
          provider={agent.provider}
          quietIdle
          size={32}
          surfaceClassName="ring-card"
        />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <Dock.Title>{title}</Dock.Title>
          <span className="truncate text-small-body text-muted-foreground">
            In {session} · {when}
          </span>
        </span>
        <Pill size="sm">{outcome}</Pill>
      </Dock.Head>
      {command ? (
        <CodeBlock
          caption=""
          code={command}
          copyLabel="Copy command"
          density="compact"
          language="bash"
          showPrompt
          wrapLines
        />
      ) : null}
      {file ? <Dock.Pre>{file}</Dock.Pre> : null}
      <p className="text-body text-muted-foreground">{why}</p>
      <Dock.Actions className="justify-end border-t border-line pt-3">{actions}</Dock.Actions>
    </Dock>
  );
}

export function SessionLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Button onClick={() => goToState(to)} size="sm" type="button" variant="ghost">
      {children}
    </Button>
  );
}
