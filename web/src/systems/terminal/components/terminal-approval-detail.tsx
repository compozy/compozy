import { Folder, SquareTerminal, TriangleAlert } from "lucide-react";

import { CodeBlock, cn, MonoId } from "@compozy/ui";

import type { TerminalPermissionDetail } from "../lib/terminal-permission";

export interface TerminalApprovalDetailProps {
  detail: TerminalPermissionDetail;
}

/**
 * What the session's decision surface cannot say on its own.
 *
 * The decision itself stays where every tool decision lives — this only adds the
 * terminal facts that surface has no way to infer. What runs is what you read:
 * no summary, no paraphrase, no client-side risk guess.
 */
export function TerminalApprovalDetail({ detail }: TerminalApprovalDetailProps) {
  if (detail.kind === "open") {
    return <TerminalOpenApprovalDetail detail={detail} />;
  }
  return <TerminalExecApprovalDetail detail={detail} />;
}

function TerminalOpenApprovalDetail({
  detail,
}: {
  detail: Extract<TerminalPermissionDetail, { kind: "open" }>;
}) {
  return (
    <div className="flex items-start gap-2" data-testid="terminal-open-approval-detail">
      <SquareTerminal aria-hidden="true" className="mt-0.5 size-3 shrink-0 text-warning" />
      <div className="flex min-w-0 flex-col gap-1.5">
        {detail.title ? <span className="text-form-input text-fg">Open {detail.title}</span> : null}
        {detail.cwd || detail.shell ? (
          <span className="flex items-center gap-1.25 font-mono text-eyebrow text-subtle">
            {detail.cwd ? (
              <>
                <Folder aria-hidden="true" className="size-3" />
                {detail.cwd}
              </>
            ) : null}
            {detail.shell ? <span>{detail.cwd ? `· ${detail.shell}` : detail.shell}</span> : null}
          </span>
        ) : null}
      </div>
    </div>
  );
}

function TerminalExecApprovalDetail({
  detail,
}: {
  detail: Extract<TerminalPermissionDetail, { kind: "exec" }>;
}) {
  const danger = detail.risk === "irreversible";
  return (
    <div className="flex min-w-0 flex-col gap-2" data-testid="terminal-approval-detail">
      {danger ? (
        <span
          className="flex items-center gap-1.5 font-semibold text-danger text-transcript-meta"
          data-testid="terminal-approval-irreversible"
        >
          <TriangleAlert aria-hidden="true" className="size-3 shrink-0" />
          This can&apos;t be undone
        </span>
      ) : null}
      <CodeBlock
        caption=""
        className={cn(danger && "border-transparent bg-danger-tint shadow-danger-inset")}
        code={detail.command}
        copyLabel="Copy command"
        data-testid="terminal-approval-command"
        density="compact"
        language="bash"
        showPrompt
        wrapLines
      />
      {detail.cwd || detail.terminalId ? (
        <span className="flex items-center gap-1.25 font-mono text-eyebrow text-subtle">
          {detail.cwd ? (
            <>
              <Folder aria-hidden="true" className="size-3" />
              {detail.cwd}
            </>
          ) : null}
          {detail.terminalId ? (
            <MonoId
              className={detail.cwd ? "ml-1.5" : undefined}
              size="sm"
              value={detail.terminalId}
            />
          ) : null}
        </span>
      ) : null}
      {detail.risk === "unclassifiable" ? (
        <p className="text-eyebrow text-faint" data-testid="terminal-approval-unclassifiable">
          Couldn&apos;t be classified, so it always asks.
        </p>
      ) : null}
    </div>
  );
}
