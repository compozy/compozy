import { Folder, Keyboard, OctagonAlert, SquareTerminal } from "lucide-react";

import { cn, MonoId } from "@compozy/ui";

import type { TerminalPermissionDetail } from "../lib/terminal-permission";

export interface TerminalApprovalDetailProps {
  detail: TerminalPermissionDetail;
  /** The terminal's title, when the catalog already knows it. */
  terminalTitle?: string;
}

/**
 * What the session's decision surface cannot say on its own.
 *
 * The decision itself stays where every tool decision lives — this only adds the
 * terminal facts that surface has no way to infer: the exact command, where it
 * would run, and why the runtime is asking. What runs is what you read: no
 * summary, no paraphrase.
 */
export function TerminalApprovalDetail({ detail, terminalTitle }: TerminalApprovalDetailProps) {
  if (detail.kind === "typing") {
    return <TerminalTypingGrantDetail detail={detail} terminalTitle={terminalTitle} />;
  }
  if (detail.kind === "open") {
    return (
      <div className="flex flex-col gap-1.5" data-testid="terminal-open-approval-detail">
        <span className="flex items-center gap-1.5 text-form-input text-fg">
          <SquareTerminal aria-hidden="true" className="size-3 text-warning" />
          Open {detail.title}
        </span>
        <span className="flex items-center gap-1.25 font-mono text-micro text-subtle">
          <Folder aria-hidden="true" className="size-2.5" />
          {detail.cwd}
          {detail.shell ? <span>· {detail.shell}</span> : null}
        </span>
      </div>
    );
  }
  const danger = detail.risk === "irreversible";
  return (
    <div className="flex flex-col gap-1.5" data-testid="terminal-approval-detail">
      {danger ? (
        <span
          className="flex items-center gap-1.5 font-semibold text-form-input text-danger"
          data-testid="terminal-approval-irreversible"
        >
          <OctagonAlert aria-hidden="true" className="size-3" />
          This can&apos;t be undone
        </span>
      ) : null}
      <div
        className={cn(
          "rounded-xs px-2.5 py-2 font-mono text-form-input leading-relaxed break-all whitespace-pre-wrap text-fg",
          danger
            ? "bg-danger-tint shadow-danger-inset"
            : "bg-chat-fill-code"
        )}
        data-testid="terminal-approval-command"
      >
        {detail.command}
      </div>
      <span className="flex items-center gap-1.25 font-mono text-micro text-subtle">
        <Folder aria-hidden="true" className="size-2.5" />
        {detail.cwd}
        {detail.terminalId ? (
          <MonoId className="ml-1.5" size="sm" value={detail.terminalId} />
        ) : null}
      </span>
      {detail.risk === "unclassifiable" ? (
        <p className="text-micro text-faint" data-testid="terminal-approval-unclassifiable">
          Couldn&apos;t be classified, so it always asks.
        </p>
      ) : null}
    </div>
  );
}

/**
 * Typing is its own permission.
 *
 * No autonomy level covers it, and the grant ends when you take over, the run
 * ends, or you revoke it — so the surface says which terminal it covers and what
 * ends it, rather than implying a standing right.
 */
function TerminalTypingGrantDetail({
  detail,
  terminalTitle,
}: {
  detail: Extract<TerminalPermissionDetail, { kind: "typing" }>;
  terminalTitle?: string;
}) {
  return (
    <div className="flex flex-col gap-1.5" data-testid="terminal-typing-grant-detail">
      <span className="flex items-center gap-1.5 text-form-input text-muted">
        <Keyboard aria-hidden="true" className="size-3 text-warning" />
        Typing in {terminalTitle ?? "this terminal"}
        {detail.terminalId ? <MonoId size="sm" value={detail.terminalId} /> : null}
      </span>
      <p className="text-micro text-faint">
        Ends when you take over, the run ends, or you revoke it.
      </p>
    </div>
  );
}
