/** S1: one declared check-in source under "What it looks at", with how its last read went. */
import { Plug, SquareTerminal } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Pill, Tooltip, TooltipContent, TooltipTrigger, type PillTone } from "@compozy/ui";

import type { SourceStatusView } from "../../lib/agent-check-in-sources";

const PILL_TONE: Record<SourceStatusView["tone"], PillTone> = {
  neutral: "neutral",
  muted: "neutral",
  destructive: "destructive",
  warning: "warning",
};

export function CheckInSourceStatus({ status }: { status: SourceStatusView }) {
  const pill = (
    <Pill
      className={status.tone === "muted" ? "text-subtle" : undefined}
      form="plain"
      tone={PILL_TONE[status.tone]}
    >
      {status.tone === "muted" ? null : <Pill.Dot />}
      {status.label}
    </Pill>
  );
  if (!status.code) return pill;
  return (
    <Tooltip>
      <TooltipTrigger render={<span className="min-w-0" />}>{pill}</TooltipTrigger>
      <TooltipContent>
        <span className="font-mono">{status.code}</span>
      </TooltipContent>
    </Tooltip>
  );
}

export interface CheckInSourceRowProps {
  title: string;
  /** A short parameter summary, such as `compozy/compozy`. */
  params?: string;
  script: boolean;
  status: SourceStatusView;
  /** Row controls, such as an Edit and Remove menu. */
  actions?: ReactNode;
}

/** The switch row's layout without the switch: sources are removed, not toggled. */
export function CheckInSourceRow({
  title,
  params,
  script,
  status,
  actions,
}: CheckInSourceRowProps) {
  const Icon = script ? SquareTerminal : Plug;
  return (
    <div className="flex min-h-10 items-center gap-3 py-1.5" data-testid="check-in-source-row">
      <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-small-body text-fg">{title}</span>
        {params ? (
          <span className="truncate font-mono text-eyebrow text-muted-foreground">{params}</span>
        ) : null}
      </span>
      <CheckInSourceStatus status={status} />
      {actions}
    </div>
  );
}
