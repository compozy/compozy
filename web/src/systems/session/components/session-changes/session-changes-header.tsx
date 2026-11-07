import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@compozy/ui";
import {
  ChevronDown,
  ChevronsDownUp,
  ChevronsUpDown,
  Columns2,
  GitPullRequest,
  ListTree,
  RefreshCw,
  Rows3,
} from "@compozy/ui/icons";

import type {
  SessionChangesScope,
  SessionChangesSummary,
} from "../../adapters/session-changes-api";
import { changesHeadLabel, formatChangeCount } from "../../lib/session-changes";
import type { SessionDiffMode } from "../../hooks/use-session-review-panel-state";
import { BaseRefSelector } from "./base-ref-selector";
import { ChangesIconAction } from "./session-changes-states";

const SCOPE_LABEL: Record<SessionChangesScope, string> = {
  branch: "Branch",
  uncommitted: "Uncommitted",
};

function ScopeMenu({
  scope,
  branchAvailable,
  onScopeChange,
}: {
  scope: SessionChangesScope;
  branchAvailable: boolean;
  onScopeChange: (scope: SessionChangesScope) => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="xs"
            aria-label={`Scope: ${SCOPE_LABEL[scope]}`}
            data-testid="changes-scope"
          />
        }
      >
        {SCOPE_LABEL[scope]}
        <ChevronDown aria-hidden data-icon="inline-end" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-36">
        <DropdownMenuRadioGroup
          value={scope}
          onValueChange={value => onScopeChange(value as SessionChangesScope)}
        >
          {/* No resolvable base: only Uncommitted is offered (US-008.EC-2). */}
          {branchAvailable ? (
            <DropdownMenuRadioItem value="branch" closeOnClick data-testid="changes-scope-branch">
              Branch
            </DropdownMenuRadioItem>
          ) : null}
          <DropdownMenuRadioItem
            value="uncommitted"
            closeOnClick
            data-testid="changes-scope-uncommitted"
          >
            Uncommitted
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ChangeTotals({ totals }: { totals: SessionChangesSummary["totals"] }) {
  const partial = totals.partial === true;
  const counts = (
    <span
      className="flex shrink-0 items-center gap-1.5 font-mono text-xs tabular-nums"
      data-testid="changes-totals"
    >
      <span className="text-success">+{formatChangeCount(totals.additions, partial)}</span>
      <span className="text-destructive">−{formatChangeCount(totals.deletions, partial)}</span>
    </span>
  );
  if (!partial) return counts;
  return (
    <Tooltip>
      <TooltipTrigger render={<span />}>
        {counts}
        <span className="sr-only">
          (lower bound: large or binary untracked files aren't fully counted)
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-64">
        Large or binary untracked files aren't fully counted, so these totals are a lower bound.
      </TooltipContent>
    </Tooltip>
  );
}

export interface SessionChangesHeaderProps {
  workspaceId: string;
  sessionId: string;
  summary: SessionChangesSummary | undefined;
  /** The scope the header shows before a summary arrives. */
  scope: SessionChangesScope;
  /** The compared base; also shown before a summary when the chosen base was not found. */
  baseRef: string | undefined;
  branchAvailable: boolean;
  baseOpen: boolean;
  onBaseOpenChange: (open: boolean) => void;
  onScopeChange: (scope: SessionChangesScope) => void;
  onBaseChange: (ref: string) => void;
  refreshing: boolean;
  onRefresh: () => void;
  diffMode: SessionDiffMode;
  /** Narrow windows force unified diffs; the toggle is then unavailable. */
  diffModeLocked: boolean;
  onDiffModeToggle: () => void;
  allCollapsed: boolean;
  onCollapseAllToggle: () => void;
  treeOpen: boolean;
  onTreeToggle: () => void;
}

/**
 * The Changes header row (`_uiux.md` S2): scope menu, totals, `head → base ▾`, the PR, and the
 * icon toolbar (Refresh, split/unified, collapse all, files tree).
 */
export function SessionChangesHeader(props: SessionChangesHeaderProps) {
  const { summary } = props;
  const scope = summary?.scope === "uncommitted" ? "uncommitted" : props.scope;
  const pullRequest = summary?.pull_request;
  return (
    <div
      className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line-soft px-2"
      data-testid="changes-header"
    >
      <ScopeMenu
        scope={scope}
        branchAvailable={props.branchAvailable}
        onScopeChange={props.onScopeChange}
      />
      {summary ? <ChangeTotals totals={summary.totals} /> : null}
      {scope === "branch" && props.baseRef ? (
        <span className="flex min-w-0 items-center gap-0.5 text-xs text-muted-foreground">
          <span className="min-w-0 truncate font-mono text-fg-2" data-testid="changes-head">
            {summary ? changesHeadLabel(summary.head) : ""}
          </span>
          <span aria-hidden>→</span>
          <BaseRefSelector
            workspaceId={props.workspaceId}
            sessionId={props.sessionId}
            value={props.baseRef}
            open={props.baseOpen}
            onOpenChange={props.onBaseOpenChange}
            onSelect={props.onBaseChange}
          />
        </span>
      ) : null}
      {pullRequest ? (
        <a
          href={pullRequest.url}
          target="_blank"
          rel="noreferrer"
          className="inline-flex shrink-0 items-center gap-1 rounded-pill px-1.5 font-mono text-xs text-fg-2 hover:bg-surface-2"
          data-testid="changes-pull-request"
        >
          <GitPullRequest aria-hidden className="size-3" />#{pullRequest.number}
        </a>
      ) : null}
      <div className="ml-auto flex shrink-0 items-center gap-0.5">
        <ChangesIconAction
          label="Refresh"
          testId="changes-refresh"
          disabled={props.refreshing}
          onClick={props.onRefresh}
        >
          <RefreshCw aria-hidden className={cn("size-3.5", props.refreshing && "animate-spin")} />
        </ChangesIconAction>
        <ChangesIconAction
          label={props.diffMode === "split" ? "Unified diff" : "Split diff"}
          testId="changes-diff-mode"
          pressed={props.diffMode === "split"}
          disabled={props.diffModeLocked}
          onClick={props.onDiffModeToggle}
        >
          {props.diffMode === "split" ? (
            <Rows3 aria-hidden className="size-3.5" />
          ) : (
            <Columns2 aria-hidden className="size-3.5" />
          )}
        </ChangesIconAction>
        <ChangesIconAction
          label={props.allCollapsed ? "Expand all" : "Collapse all"}
          testId="changes-collapse-all"
          onClick={props.onCollapseAllToggle}
        >
          {props.allCollapsed ? (
            <ChevronsUpDown aria-hidden className="size-3.5" />
          ) : (
            <ChevronsDownUp aria-hidden className="size-3.5" />
          )}
        </ChangesIconAction>
        <ChangesIconAction
          label={props.treeOpen ? "Hide files" : "Show files"}
          testId="changes-tree-toggle"
          pressed={props.treeOpen}
          onClick={props.onTreeToggle}
        >
          <ListTree aria-hidden className="size-3.5" />
        </ChangesIconAction>
      </div>
    </div>
  );
}
