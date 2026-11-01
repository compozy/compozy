/** S11: report entries, set apart from turns. They hold no model turn and never start one. */
import { ArrowRight, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Eyebrow, Marker, cn } from "@compozy/ui";

import { goToState } from "../persistent-agents-shared/go-to-state";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { DraftAgent } from "../persistent-agents-shared/fixtures";

export interface Proposal {
  title: string;
  body: string;
}

const TONE = {
  neutral: "text-muted",
  danger: "text-danger",
  warning: "text-warning",
} as const;

export function ReportEntry({
  agent,
  kind,
  when,
  icon: Icon,
  tone = "neutral",
  title,
  body,
  proposals,
  actions,
  footnote,
}: {
  agent: DraftAgent;
  kind: string;
  when: string;
  icon?: LucideIcon;
  tone?: keyof typeof TONE;
  title?: string;
  body?: ReactNode;
  proposals?: readonly Proposal[];
  actions?: ReactNode;
  footnote?: ReactNode;
}) {
  return (
    <article className="flex flex-col gap-3 rounded-lg bg-card p-3.5 shadow-card">
      <header className="flex items-center gap-2">
        <AgentAvatar
          appearance={agent.appearance}
          label={agent.label}
          quietIdle
          size={20}
          surfaceClassName="ring-card"
        />
        <span className="text-small-body font-medium text-fg">{agent.label}</span>
        <Eyebrow>{kind}</Eyebrow>
        <span className="ml-auto text-eyebrow text-subtle">{when}</span>
      </header>
      {title ? (
        <div className="flex items-start gap-2">
          {Icon ? (
            <Icon aria-hidden="true" className={cn("mt-0.5 size-4 shrink-0", TONE[tone])} />
          ) : null}
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="text-small-body font-medium text-fg">{title}</span>
            {body ? <span className="text-small-body text-muted">{body}</span> : null}
          </div>
        </div>
      ) : null}
      {proposals ? (
        <ul className="flex flex-col gap-1">
          {proposals.map(proposal => (
            <li key={proposal.title}>
              <button
                className="group flex w-full items-center gap-3 rounded-md bg-sunken px-3 py-2.5 text-left outline-none hover:bg-hover focus-visible:shadow-focus-ring"
                type="button"
              >
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-small-body font-medium text-fg">{proposal.title}</span>
                  <span className="text-small-body text-muted">{proposal.body}</span>
                </span>
                <ArrowRight aria-hidden="true" className="size-4 shrink-0 text-subtle" />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
      {footnote ? <p className="text-eyebrow text-subtle">{footnote}</p> : null}
    </article>
  );
}

export function SessionLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Button onClick={() => goToState(to)} size="sm" type="button" variant="ghost">
      {children}
    </Button>
  );
}

export { Marker };
