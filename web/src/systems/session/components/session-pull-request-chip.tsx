import {
  CheckIcon,
  ChevronDownIcon,
  CircleHelpIcon,
  CircleSlashIcon,
  ClockIcon,
  GitBranchIcon,
  GitMergeIcon,
  GitPullRequestClosedIcon,
  GitPullRequestDraftIcon,
  GitPullRequestIcon,
  type LucideIcon,
} from "@compozy/ui/icons";
import { useState } from "react";

import {
  cn,
  DropdownMenu,
  DropdownMenuTrigger,
  Pill,
  Spinner,
  StateGlyph,
  Time,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import {
  pullRequestChipModel,
  type PullRequestChipModel,
  type PullRequestShape,
  type PullRequestSignalPart,
} from "../lib/pull-request-signal";
import type { PullRequestExitPlanView } from "../lib/pull-request-actions";
import type { SessionPullRequestFacts } from "../lib/session-pull-request";
import {
  SessionPullRequestMenu,
  type SessionPullRequestPromptKind,
} from "./session-pull-request-menu";

const SHAPE_ICON: Record<PullRequestShape, LucideIcon> = {
  open: GitPullRequestIcon,
  draft: GitPullRequestDraftIcon,
  merged: GitMergeIcon,
  closed: GitPullRequestClosedIcon,
};

/** Worktree-chip geometry (`SessionWorktreeBindingChip`): 24 px surface-2 pill, badge type. */
const CHIP_CLASS =
  "inline-flex h-6 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-pill bg-surface-2 py-0 pr-1.5 pl-2.5 text-badge font-medium text-muted-foreground outline-none transition-colors duration-fast ease-out hover:bg-selected hover:text-fg focus-visible:shadow-focus-ring data-popup-open:bg-selected data-popup-open:text-fg [&_svg]:size-3 [&_svg]:shrink-0";

export interface SessionPullRequestChipProps {
  facts: SessionPullRequestFacts;
  /** User-controllable sessions get exit actions and agent prompts in the menu. */
  controllable: boolean;
  /** The bound managed worktree's exit plan; absent for workspace-root sessions. */
  exitPlan?: PullRequestExitPlanView;
  exitRunning?: boolean;
  refreshing?: boolean;
  onExitAction?: (action: string) => void;
  onRefresh?: () => void;
  onPrompt?: (kind: SessionPullRequestPromptKind, text: string) => void;
}

/**
 * The session's branch or pull request in the topbar status zone (`_uiux.md` S1),
 * with the next delivery action in its menu (S2). No branch renders nothing.
 */
export function SessionPullRequestChip({
  facts,
  controllable,
  exitPlan,
  exitRunning = false,
  refreshing = false,
  onExitAction,
  onRefresh,
  onPrompt,
}: SessionPullRequestChipProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const model = pullRequestChipModel(facts);
  if (model === null) return null;

  return (
    <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
      <Tooltip disabled={menuOpen}>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              aria-label={model.ariaLabel}
              className={CHIP_CLASS}
              data-kind={model.kind}
              data-slot="session-pull-request-chip"
              data-stale={model.kind === "pull-request" && model.stale ? "true" : undefined}
              data-testid="session-pull-request-chip"
              render={<button type="button" />}
            />
          }
        >
          <ChipBody model={model} refreshing={refreshing} />
          <ChevronDownIcon aria-hidden="true" className="text-subtle" />
        </TooltipTrigger>
        <TooltipContent
          align="start"
          className="max-w-75 min-w-60 flex-col items-stretch gap-0 px-2.5 py-2 text-left whitespace-normal"
          data-testid="session-pull-request-tooltip"
          side="bottom"
        >
          <ChipTooltip model={model} />
        </TooltipContent>
      </Tooltip>
      <SessionPullRequestMenu
        controllable={controllable}
        exitPlan={exitPlan}
        exitRunning={exitRunning}
        observedAt={facts.pull_request_lookup?.observed_at}
        onExitAction={onExitAction}
        onPrompt={onPrompt}
        onRefresh={onRefresh}
        pullRequest={facts.pull_request}
        refreshing={refreshing}
      />
    </DropdownMenu>
  );
}

function ChipGlyph({ icon: Icon, refreshing }: { icon: LucideIcon; refreshing: boolean }) {
  return (
    <span className="grid size-3 shrink-0 place-items-center text-subtle" data-slot="chip-glyph">
      {refreshing ? <Spinner aria-label="Refreshing" className="size-3" /> : <Icon aria-hidden />}
    </span>
  );
}

const MONO_ID = "font-mono text-mono-id tracking-mono-id text-fg-2";

function ChipBody({ model, refreshing }: { model: PullRequestChipModel; refreshing: boolean }) {
  if (model.kind === "branch") {
    return (
      <>
        <ChipGlyph icon={GitBranchIcon} refreshing={refreshing} />
        <span className={MONO_ID} data-slot="chip-branch">
          {model.branchLabel}
        </span>
        <BranchMarker marker={model.marker} />
      </>
    );
  }
  return (
    <>
      <ChipGlyph icon={SHAPE_ICON[model.shape]} refreshing={refreshing} />
      <span
        className={cn(
          MONO_ID,
          "font-medium tabular-nums",
          model.stale && "underline decoration-faint decoration-dotted underline-offset-3"
        )}
        data-slot="chip-number"
      >
        #{model.number}
      </span>
      <span className="text-muted-foreground" data-slot="chip-word">
        {model.word}
      </span>
      {model.signals.map(signal => (
        <ChipSignal key={signal.id} signal={signal} />
      ))}
      {model.stale ? (
        <span className="grid place-items-center text-subtle" data-slot="chip-stale">
          <ClockIcon aria-label="Stale" />
        </span>
      ) : null}
    </>
  );
}

function BranchMarker({ marker }: { marker: "pending" | "unavailable" | "incomplete" | null }) {
  if (marker === null) return null;
  return (
    <span
      className="grid place-items-center text-subtle"
      data-marker={marker}
      data-slot="chip-marker"
    >
      {marker === "pending" ? (
        <StateGlyph label="Looking up" size="sm" state="queued" />
      ) : marker === "unavailable" ? (
        <CircleSlashIcon aria-label="Unavailable" />
      ) : (
        <CircleHelpIcon aria-label="No confirmed match" />
      )}
    </span>
  );
}

const SIGNAL_TONE_CLASS = { destructive: "text-destructive", success: "text-success" } as const;

function ChipSignal({ signal }: { signal: PullRequestSignalPart }) {
  const toneClass =
    signal.tone === "destructive" || signal.tone === "success"
      ? SIGNAL_TONE_CLASS[signal.tone]
      : "";
  return (
    <>
      <span aria-hidden="true" className="text-faint">
        ·
      </span>
      <span
        className={cn(
          "inline-flex items-center gap-1 text-muted-foreground tabular-nums",
          toneClass
        )}
        data-signal={signal.id}
        data-slot="chip-signal"
        data-tone={signal.tone ?? undefined}
      >
        {signal.mark === "dot" ? (
          <Pill.Dot size="sm" tone={signal.tone ?? "neutral"} />
        ) : signal.mark === "spinner" ? (
          <Spinner aria-hidden="true" className="size-2.5" role={undefined} />
        ) : signal.mark === "check" ? (
          <CheckIcon aria-hidden="true" />
        ) : null}
        {signal.label}
      </span>
    </>
  );
}

function ChipTooltip({ model }: { model: PullRequestChipModel }) {
  if (model.kind === "branch") {
    return (
      <>
        <div className="font-mono text-mono-id break-all text-fg-2">{model.branch}</div>
        <div className="mt-1">{model.detail}</div>
      </>
    );
  }
  return (
    <>
      <div className="font-mono text-mono-id break-all text-fg-2">{model.branch}</div>
      {model.rows.length > 0 ? (
        <dl className="mt-1.5 grid gap-0.5">
          {model.rows.map(row => (
            <div className="flex justify-between gap-4 whitespace-nowrap" key={row.label}>
              <dt className="font-normal text-subtle">{row.label}</dt>
              <dd>{row.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {model.failedNames.length > 0 ? (
        <ul className="mt-1.5 font-mono text-mono-id font-normal text-fg-2">
          {model.failedNames.map(name => (
            <li key={name}>{name}</li>
          ))}
        </ul>
      ) : null}
      <ObservedFoot model={model} />
    </>
  );
}

function ObservedFoot({
  model,
}: {
  model: Extract<PullRequestChipModel, { kind: "pull-request" }>;
}) {
  const prefix = model.causeText ? `${model.causeText} · ` : "";
  return (
    <div
      className="mt-1.5 border-t border-line-soft pt-1.5 font-normal text-subtle"
      data-slot="chip-observed"
    >
      {model.observedAt ? (
        <>
          {prefix}
          {model.stale ? "Last checked " : "Checked "}
          <Time iso={model.observedAt} />
        </>
      ) : (
        `${prefix}Not checked yet`
      )}
    </div>
  );
}
