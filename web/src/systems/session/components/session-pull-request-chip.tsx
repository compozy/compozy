import {
  CheckIcon,
  ChevronDownIcon,
  CircleHelpIcon,
  CircleSlashIcon,
  ClockIcon,
  GitBranchIcon,
  type LucideIcon,
} from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import {
  cn,
  DropdownMenu,
  DropdownMenuTrigger,
  Pill,
  Spinner,
  StateGlyph,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import {
  PULL_REQUEST_SHAPE_ICON,
  SESSION_CHIP_CLASS,
  SESSION_PULL_REQUEST_DETAILS_CLASS,
} from "./session-pull-request-chip-style";
import { SessionPullRequestDetails } from "./session-pull-request-details";
import {
  pullRequestChipModel,
  type PullRequestChipModel,
  type PullRequestSignalPart,
} from "../lib/pull-request-signal";
import type { PullRequestExitPlanView } from "../lib/pull-request-actions";
import type { SessionPullRequestFacts } from "../lib/session-pull-request";
import {
  SessionPullRequestMenu,
  type SessionPullRequestPromptKind,
} from "./session-pull-request-menu";

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
  /** Rows after the delivery actions in the menu. */
  menuFooter?: ReactNode;
  /** Opens with the menu shown (prototypes and stories). */
  defaultMenuOpen?: boolean;
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
  menuFooter,
  defaultMenuOpen = false,
}: SessionPullRequestChipProps) {
  const [menuOpen, setMenuOpen] = useState(defaultMenuOpen);
  const model = pullRequestChipModel(facts);
  if (model === null) return null;

  return (
    <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
      <Tooltip disabled={menuOpen}>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              aria-label={model.ariaLabel}
              className={SESSION_CHIP_CLASS}
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
          className={SESSION_PULL_REQUEST_DETAILS_CLASS}
          data-testid="session-pull-request-tooltip"
          side="bottom"
        >
          <SessionPullRequestDetails model={model} />
        </TooltipContent>
      </Tooltip>
      <SessionPullRequestMenu
        controllable={controllable}
        exitPlan={exitPlan}
        exitRunning={exitRunning}
        footer={menuFooter}
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
      <ChipGlyph icon={PULL_REQUEST_SHAPE_ICON[model.shape]} refreshing={refreshing} />
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
