import { useState } from "react";
import { AlertCircle, AlertTriangle, Check, ChevronDown, Search } from "lucide-react";

import { Button, cn, Eyebrow, Pill, Spinner } from "@compozy/ui";

import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import {
  isBlockingIssue,
  lintDockCounters,
  type LoopLintDockCounters,
  type LoopLintState,
} from "../../lib/loop-editor-lint";
import type { LoopValidationIssue } from "../../types";

interface LoopLinterDockProps {
  lint: LoopLintState;
  /** True when the last (passive or manual) validate could not reach CompozyOS. */
  validateFailed: boolean;
  onReveal: (nodeId: string) => void;

  collapsed?: boolean;
  onToggleCollapsed?: () => void;
}

export function LoopLinterDock({
  lint,
  validateFailed,
  onReveal,
  collapsed: collapsedProp,
  onToggleCollapsed,
}: LoopLinterDockProps) {
  const [localCollapsed, setLocalCollapsed] = useState(true);
  const collapsed = collapsedProp ?? localCollapsed;
  const toggleCollapsed = onToggleCollapsed ?? (() => setLocalCollapsed(value => !value));
  const counters = lintDockCounters(lint, validateFailed);

  return (
    <div
      className="flex max-h-48 flex-none flex-col border-t border-line bg-sunken"
      data-testid="loop-linter-dock"
    >
      <button
        type="button"
        onClick={toggleCollapsed}
        className="flex h-9 items-center gap-2 px-4"
        aria-expanded={!collapsed}
        data-testid="loop-linter-toggle"
      >
        <Eyebrow className="text-subtle">Validation</Eyebrow>
        <LinterDockCounterPills counters={counters} />
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
          <LinterDockBody issues={lint.issues} onReveal={onReveal} state={counters.state} />
        </div>
      )}
    </div>
  );
}

function LinterDockCounterPills({ counters }: { counters: LoopLintDockCounters }) {
  return (
    <>
      {counters.state === "pending" ? (
        <Pill size="xs" tone="neutral" data-testid="loop-linter-count">
          Checking…
        </Pill>
      ) : null}
      {counters.state === "unavailable" ? (
        <Pill size="xs" tone="danger" data-testid="loop-linter-count">
          Unavailable
        </Pill>
      ) : null}
      {counters.errors !== undefined ? (
        <Pill size="xs" tone="danger" data-testid="loop-linter-error-count">
          {counters.errors} error{counters.errors === 1 ? "" : "s"}
        </Pill>
      ) : null}
      {counters.warnings !== undefined ? (
        <Pill size="xs" tone="warning" data-testid="loop-linter-warning-count">
          {counters.warnings} warning{counters.warnings === 1 ? "" : "s"}
        </Pill>
      ) : null}
    </>
  );
}

function LinterDockBody({
  state,
  issues,
  onReveal,
}: {
  state: LoopLintDockCounters["state"];
  issues: readonly LoopValidationIssue[];
  onReveal: (nodeId: string) => void;
}) {
  if (state === "unavailable") {
    return (
      <p
        className="flex items-center gap-2 px-4 py-3 text-small-body text-danger"
        data-testid="loop-linter-unavailable"
      >
        <AlertCircle aria-hidden="true" className="size-4" />
        Couldn&apos;t check this Loop. Click Validate to try again.
      </p>
    );
  }
  if (state === "pending") {
    return (
      <p className="flex items-center gap-2 px-4 py-3 text-small-body text-subtle">
        <Spinner aria-hidden="true" className="size-4" />
        Checking…
      </p>
    );
  }
  if (state === "clean") {
    return (
      <p className="flex items-center gap-2 px-4 py-3 text-small-body text-success">
        <Check aria-hidden="true" className="size-4" />
        No problems found. Ready to publish.
      </p>
    );
  }
  return withOccurrenceKeys(
    issues,
    issue =>
      `${issue.node_id ?? ""}\u0000${issue.code}\u0000${issue.severity}\u0000${issue.message}`
  ).map(({ item: issue, key }) => <IssueRow key={key} issue={issue} onReveal={onReveal} />);
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
      className="flex items-start gap-3 border-t border-line-soft px-4 py-2"
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
          <span className="font-medium text-fg-strong">{issue.node_id || "Whole Loop"}</span> ·{" "}
          {issue.message}
        </p>
        <p className="mt-0.5 text-form-hint text-subtle" title={issue.code}>
          {blocking ? "Must be fixed before publishing" : "Won’t block publishing"}
        </p>
      </div>
      {issue.node_id ? (
        <Button
          type="button"
          variant="quiet"
          size="xs"
          onClick={() => onReveal(issue.node_id!)}
          className="shrink-0"
          data-testid="loop-linter-reveal"
        >
          <Search aria-hidden="true" data-icon="inline-start" />
          Show step
        </Button>
      ) : null}
    </div>
  );
}
