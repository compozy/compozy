import { StateGlyph, useTopbarSlot } from "@compozy/ui";

import { SessionPullRequestChip } from "../components/session-pull-request-chip";
import { SessionPrimaryAction } from "../components/session-primary-action";
import { SessionPanelToggle } from "../components/session-panel-toggle";

import type { WorktreePayload } from "@/systems/workspace";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionBadgeOf, sessionBadgeSignal } from "../lib/session-badge";
import type { SessionOriginView } from "../lib/session-origin";
import type { PullRequestExitPlanView } from "../lib/pull-request-actions";
import { sessionPullRequestFacts, sessionWaitingOn } from "../lib/session-pull-request";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";
import { SessionStatusLine } from "../components/session-status-line";
import { SessionWorktreeBindingChip } from "../components/session-worktree-binding-chip";
import { SessionTransportContext } from "../lib/session-transcript-thread-context-value";
import { useSessionTransportState } from "./use-session-transcript-thread-messages";
import { SessionUnrestrictedBadge } from "../components/session-unrestricted-badge";
import { SessionViewToggle } from "../components/session-board/session-view-toggle";
import { useSessionWindowViewState } from "./use-session-window-view-state";
import { useSessionWindowPresentation } from "./use-session-window-presentation";
import { sessionIsUnrestricted } from "../lib/session-unrestricted";

interface SessionTopbarWorktreeBinding {
  worktreeId: string;
  worktree: WorktreePayload | undefined;
  onOpenContext?: () => void;
  onResolve?: () => void;
}

/** The bound managed worktree's exit plan, offered first in the PR chip menu. */
interface SessionTopbarPullRequestExit {
  plan?: PullRequestExitPlanView;
  running: boolean;
  onAction: (action: string) => void;
}

/** The explicit PR refresh, owned by the window that holds the session's reads. */
interface SessionTopbarPullRequestRefresh {
  refreshing: boolean;
  refresh: () => void;
}

interface UseSessionTopbarSlotInput {
  /** Absent for an unbound session — absence is the signal, not a placeholder. */
  worktreeBinding?: SessionTopbarWorktreeBinding;
  session: SessionPayload;
  isStopping: boolean;
  isUnarchiving: boolean;
  inspectorOpen: boolean;
  sidebarOpen: boolean;
  /** Absent for workspace-root sessions: commit/push/create-PR never appear there. */
  pullRequestExit?: SessionTopbarPullRequestExit;
  pullRequestRefresh?: SessionTopbarPullRequestRefresh;
  /** Sends one normal user prompt to this session; it queues while a turn runs. */
  onSendPrompt?: (text: string) => void;
  /** Undefined until Goal presence is known; stopping still explains that any Goal will pause. */
  hasLiveGoal?: boolean;
  /** The connection chip (S4): mounts in the status slot's reserved trailing position, before the actions. */
  transportChip?: React.ReactNode;
  onInspectorToggle: () => void;
  onSidebarToggle: () => void;
  onStop: () => void;
  onUnarchive: () => void;
  /** Continued/forked origin shown as the status-line pill. */
  origin?: SessionOriginView | null;
  onOpenOriginSource?: (sessionId: string) => void;
}

function sessionTopbarActions({ session, isStopping, isUnarchiving }: UseSessionTopbarSlotInput) {
  const isActive = session.state === "active" || session.state === "starting";
  const isArchived = session.archived_at !== null;
  const lifecycleControllable = isUserControllableSession(session);
  const showUnarchiveAction = lifecycleControllable && isArchived;
  const showStopAction = lifecycleControllable && isActive;
  const controlsBusy = isStopping || isUnarchiving;

  return {
    lifecycleControllable,
    showUnarchiveAction,
    showStopAction,
    controlsBusy,
  };
}

/** Publishes lifecycle actions into the owning session window's topbar. */
export function useSessionTopbarSlot(input: UseSessionTopbarSlotInput): void {
  const { session, worktreeBinding, transportChip } = input;
  const actions = sessionTopbarActions(input);
  const view = useSessionWindowViewState();
  // Over the board the drawer owns its chrome: no rail to toggle, and the board is the view.
  const inWindow = useSessionWindowPresentation() === "window";
  // The slot consumer (the OS head) renders outside this window's session
  // runtime provider, so a node that reads the transport — the chip — would
  // see the default live snapshot there. The publisher runs inside the
  // provider: it carries the window's own snapshot with the node, per window,
  // no second stream (task_06 VC-01..05).
  const transport = useSessionTransportState();
  const { pullRequestExit, pullRequestRefresh, onSendPrompt } = input;
  const glyphState = sessionBadgeSignal(sessionBadgeOf(session), sessionWaitingOn(session)).state;

  useTopbarSlot({
    glyph: <StateGlyph size="sm" state={glyphState} />,
    glyphPresentation: "state",
    crumb: getSessionDisplayTitle(session),
    status: (
      <span className="flex min-w-0 items-center gap-2">
        <SessionPullRequestChip
          controllable={actions.lifecycleControllable}
          exitPlan={pullRequestExit?.plan}
          exitRunning={pullRequestExit?.running ?? false}
          facts={sessionPullRequestFacts(session)}
          onExitAction={pullRequestExit?.onAction}
          onPrompt={onSendPrompt ? (_kind, text) => onSendPrompt(text) : undefined}
          onRefresh={pullRequestRefresh?.refresh}
          refreshing={pullRequestRefresh?.refreshing ?? false}
        />
        <SessionStatusLine
          onOpenOriginSource={input.onOpenOriginSource}
          origin={input.origin}
          session={session}
          showState={false}
        />
        {sessionIsUnrestricted(session) ? (
          <SessionUnrestrictedBadge data-testid="session-topbar-unrestricted" />
        ) : null}
        {/* OQ7, revised: the PR chip menu carries the next exit action; the binding
            chip opens the full worktree context, and the split exit control stays there. */}
        {worktreeBinding ? (
          <SessionWorktreeBindingChip
            onOpenContext={worktreeBinding.onOpenContext}
            onResolve={worktreeBinding.onResolve}
            worktree={worktreeBinding.worktree}
            worktreeId={worktreeBinding.worktreeId}
          />
        ) : null}
        {transportChip ? (
          <SessionTransportContext.Provider value={transport}>
            {transportChip}
          </SessionTransportContext.Provider>
        ) : null}
      </span>
    ),
    // The run verbs lead; the two panel toggles trail as quiet icons.
    actions: (
      <>
        <SessionPrimaryAction key={session.id} {...input} {...actions} />
        {inWindow ? (
          <SessionPanelToggle
            panel="sidebar"
            open={input.sidebarOpen}
            onToggle={input.onSidebarToggle}
          />
        ) : null}
        <SessionPanelToggle
          panel="inspector"
          open={input.inspectorOpen}
          onToggle={input.onInspectorToggle}
        />
      </>
    ),
    // Per-session verbs live in the session list row menus; the head keeps only the view toggle.
    overflow: inWindow ? (
      <SessionViewToggle onValueChange={view.setView} value={view.view} />
    ) : undefined,
  });
}
