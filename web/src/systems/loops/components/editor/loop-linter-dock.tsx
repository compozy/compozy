import { useState } from "react";
import { AlertCircle, AlertTriangle, Check, ChevronDown, Search } from "lucide-react";

import { cn, Eyebrow, Spinner } from "@agh/ui";

import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import { isBlockingIssue, type LoopLintState } from "../../lib/loop-editor-lint";
import type { LoopValidationIssue } from "../../types";

interface LoopLinterDockProps {
  lint: LoopLintState;
  /** True when the last (passive or manual) validate could not reach the daemon. */
  validateFailed: boolean;
  onReveal: (nodeId: string) => void;
}

/**
 * The linter dock (design §4.6): a collapsible panel pinned under the canvas showing the
 * shared linter's issues with their deterministic codes and a "Reveal node" jump. Blocking
 * errors render danger + "publish returns 422"; non-blocking warnings render warning-tinted
 * and never claim a 422 gate (daemon truth wins). Before the first verdict the dock shows a
 * neutral pending/failed state, never a claimed pass. The dock reports — it never computes.
 */
export function LoopLinterDock({ lint, validateFailed, onReveal }: LoopLinterDockProps) {
  const [collapsed, setCollapsed] = useState(true);
  const failed = !lint.validated && validateFailed;
  const pending = !lint.validated && !validateFailed;
  const clean = lint.validated && lint.issues.length === 0;
  const errorCount = lint.errorCount;

  const badgeTone = pending
    ? "bg-badge-fill text-subtle"
    : failed
      ? "bg-danger-tint text-danger"
      : errorCount > 0
        ? "bg-danger-tint text-danger"
        : clean
          ? "bg-success-tint text-success"
          : "bg-warning-tint text-warning";
  const badgeText = pending
    ? "checking…"
    : failed
      ? "unavailable"
      : errorCount > 0
        ? `${errorCount} issue${errorCount === 1 ? "" : "s"}`
        : clean
          ? "0 issues"
          : `${lint.issues.length} warning${lint.issues.length === 1 ? "" : "s"}`;

  return (
    <div
      className="flex max-h-48 flex-none flex-col border-t border-line bg-canvas-soft"
      data-testid="loop-linter-dock"
    >
      <button
        type="button"
        onClick={() => setCollapsed(value => !value)}
        className="flex h-9 items-center gap-2.5 px-3.5"
        aria-expanded={!collapsed}
        data-testid="loop-linter-toggle"
      >
        <Eyebrow className="text-subtle">Validation</Eyebrow>
        <span
          className={cn("rounded-xs px-1.5 py-0.5 font-mono text-badge", badgeTone)}
          data-testid="loop-linter-count"
        >
          {badgeText}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "ml-auto size-4 text-muted transition-transform",
            collapsed && "-rotate-90"
          )}
        />
      </button>
      {collapsed ? null : (
        <div className="min-h-0 overflow-y-auto pb-2">
          {failed ? (
            <p
              className="flex items-center gap-2.5 px-3.5 py-3 text-small-body text-danger"
              data-testid="loop-linter-unavailable"
            >
              <AlertCircle aria-hidden="true" className="size-4" />
              Couldn&apos;t reach the shared linter. Use Validate to retry.
            </p>
          ) : pending ? (
            <p className="flex items-center gap-2.5 px-3.5 py-3 text-small-body text-subtle">
              <Spinner aria-hidden="true" className="size-4" />
              Validating against the shared linter…
            </p>
          ) : clean ? (
            <p className="flex items-center gap-2.5 px-3.5 py-3 text-small-body text-success">
              <Check aria-hidden="true" className="size-4" />
              All invariants pass. Publish compiles the resolved form and saves with an
              expected_version compare-and-swap.
            </p>
          ) : (
            withOccurrenceKeys(
              lint.issues,
              issue =>
                `${issue.node_id ?? ""}\u0000${issue.code}\u0000${issue.severity}\u0000${issue.message}`
            ).map(({ item: issue, key }) => (
              <IssueRow key={key} issue={issue} onReveal={onReveal} />
            ))
          )}
        </div>
      )}
    </div>
  );
}

function IssueRow({
  issue,
  onReveal,
}: {
  issue: LoopValidationIssue;
  onReveal: (nodeId: string) => void;
}) {
  const blocking = isBlockingIssue(issue);
  return (
    <div
      className="flex items-start gap-3 border-t border-line-soft px-3.5 py-2.5"
      data-testid="loop-linter-issue"
      data-severity={blocking ? "error" : "warning"}
    >
      {blocking ? (
        <AlertCircle aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-danger" />
      ) : (
        <AlertTriangle aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
      )}
      <div className="min-w-0 flex-1">
        <p className="text-small-body leading-snug text-fg">
          <span className="font-mono text-mono-id font-medium text-fg-strong">
            {issue.node_id || "graph"}
          </span>{" "}
          · {issue.message}
        </p>
        <p className="mt-0.5 font-mono text-mono-id text-subtle">
          code: {issue.code} ·{" "}
          {blocking ? "publish returns 422 until resolved" : "warning · does not block Publish"}
        </p>
      </div>
      {issue.node_id ? (
        <button
          type="button"
          onClick={() => onReveal(issue.node_id!)}
          className="flex shrink-0 items-center gap-1.5 rounded-md border border-line-soft bg-btn-fill px-2.5 py-1 text-form-hint text-muted hover:border-line-strong hover:text-fg-strong"
          data-testid="loop-linter-reveal"
        >
          <Search aria-hidden="true" className="size-3" />
          Reveal node
        </button>
      ) : null}
    </div>
  );
}
