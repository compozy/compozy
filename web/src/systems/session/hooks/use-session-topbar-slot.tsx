import { AgentAvatar, Pill, StateGlyph, useTopbarSlot } from "@compozy/ui";

import {
  agentAvatarAppearance,
  agentInstanceAvatarState,
  alwaysOnMainInstance,
  useAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";
import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";
import { PROFILE_SPRITE_URL } from "@/systems/profiles";
import { WorkspaceBadge, useActiveWorkspace, workspaceBadgeIdentity } from "@/systems/workspace";

import {
  SessionLinkedWorkNarrowEntry,
  SessionLinkedWorkStatus,
  type SessionLinkedWorkListProps,
  type SessionLinkedWorkStatusProps,
} from "../components/session-linked-work/session-linked-work-header";
import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { SessionMarkSeenAction } from "../components/session-mark-seen-action";
import { SessionPrimaryAction } from "../components/session-primary-action";
import { sessionProcessPorts, sessionRunningProcesses } from "../lib/session-processes";
import { SessionPanelToggle } from "../components/session-panel-toggle";
import { SessionReviewPanelToggle } from "../components/session-review-panel";
import { AlwaysOnAgentMenu } from "../components/always-on/always-on-agent-menu";

import type { WorktreePayload } from "@/systems/workspace";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionBadgeOf, sessionBadgeSignal } from "../lib/session-badge";
import type { SessionOriginView } from "../lib/session-origin";
import type { PullRequestExitPlanView } from "../lib/pull-request-actions";
import { isOwnBranch, type SessionLinkedWorkItem } from "../lib/session-linked-work";
import { sessionPullRequestFacts, sessionWaitingOn } from "../lib/session-pull-request";
import { isUserControllableSession } from "../lib/session-running";
import { requestSessionLinkDialog } from "./use-session-link-dialog";
import { requestSessionReviewChanges } from "./use-session-review-panel-state";
import { sessionHasFileRoot } from "../lib/session-file-root";
import type { SessionLinkedWorkModel } from "./use-session-linked-work";
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
  /** The Review panel (Files); its toggle is absent for a session without a file root. */
  reviewOpen: boolean;
  onReviewToggle: () => void;
  sidebarOpen: boolean;
  /** Absent for workspace-root sessions: commit/push/create-PR never appear there. */
  pullRequestExit?: SessionTopbarPullRequestExit;
  pullRequestRefresh?: SessionTopbarPullRequestRefresh;
  linkedWork: SessionLinkedWorkModel;
  /** Opens the worktree a linked item lives in, when this client can show it. */
  worktreeAction?: (item: SessionLinkedWorkItem) => (() => void) | undefined;
  onOpenSession?: (sessionId: string) => void;
  /** Sends one normal user prompt to this session; it queues while a turn runs. */
  onSendPrompt?: (text: string) => void;
  /** Undefined until Goal presence is known; stopping still explains that any Goal will pause. */
  hasLiveGoal?: boolean;
  /** The connection chip (S4): mounts in the status slot's reserved trailing position, before the actions. */
  transportChip?: React.ReactNode;
  onInspectorToggle: () => void;
  onSidebarToggle: () => void;
  onStop: (processes?: SessionProcessPolicy) => void;
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

/** The linked work list's props: what it shows, where the session runs, what a row can do. */
function useLinkedWorkList(
  input: UseSessionTopbarSlotInput,
  controllable: boolean,
  workspaceNames: ReadonlyMap<string, string>
): SessionLinkedWorkListProps {
  const { session, worktreeBinding, linkedWork, pullRequestRefresh } = input;
  const { label: agentLabel } = useAgentIdentityView(session.agent_name);
  return {
    items: linkedWork.items,
    runsIn: worktreeBinding
      ? { kind: "worktree", name: worktreeBinding.worktree?.name ?? worktreeBinding.worktreeId }
      : {
          kind: "project",
          name:
            workspaceNames.get(session.workspace_id ?? "") ??
            session.workspace_path ??
            session.workspace_id ??
            "",
        },
    agentLabel,
    actions: {
      controllable,
      isOwn: item => isOwnBranch(item, session),
      refreshing: linkedWork.refreshing,
      onLink: () => requestSessionLinkDialog(session.id),
      onUnlink: linkedWork.unlink,
      onOpenSession: id => input.onOpenSession?.(id),
      onRefresh: item => {
        if (pullRequestRefresh && isOwnBranch(item, session)) pullRequestRefresh.refresh();
        else linkedWork.refresh(item);
      },
      onPrompt: input.onSendPrompt,
    },
  };
}

/** The delivery actions a chip carries for the session's own branch. */
function ownChipActions({
  session,
  pullRequestExit,
  pullRequestRefresh,
}: UseSessionTopbarSlotInput): SessionLinkedWorkStatusProps["ownChip"] {
  return {
    // The session's own branch diff (US-007.AC-8): Branch scope, absent in user scope.
    onReviewChanges: sessionHasFileRoot(session.workspace_id)
      ? () => requestSessionReviewChanges(session.id, { scope: "branch" })
      : undefined,
    exitPlan: pullRequestExit?.plan,
    exitRunning: pullRequestExit?.running ?? false,
    onExitAction: pullRequestExit?.onAction,
    onRefresh: pullRequestRefresh?.refresh,
    refreshing: pullRequestRefresh?.refreshing ?? false,
  };
}

/** The sessions sidebar toggle; the board drawer has no rail to toggle. */
function sidebarToggle(input: UseSessionTopbarSlotInput, inWindow: boolean) {
  return inWindow ? (
    <SessionPanelToggle panel="sidebar" open={input.sidebarOpen} onToggle={input.onSidebarToggle} />
  ) : null;
}

/** An ordinary session's head: linked work entry, run verbs, then the panel toggles. */
function sessionActions(
  input: UseSessionTopbarSlotInput,
  actions: ReturnType<typeof sessionTopbarActions>,
  linkedList: SessionLinkedWorkListProps,
  inWindow: boolean
) {
  const { session, linkedWork } = input;
  return (
    <>
      {linkedWork.clipped ? <SessionLinkedWorkNarrowEntry {...linkedList} /> : null}
      <SessionMarkSeenAction session={session} />
      <SessionPrimaryAction
        key={session.id}
        {...input}
        {...actions}
        runningProcesses={sessionRunningProcesses(session)}
        processPorts={sessionProcessPorts(session)}
      />
      {sidebarToggle(input, inWindow)}
      <SessionReviewPanelToggle
        workspaceId={session.workspace_id}
        open={input.reviewOpen}
        onToggle={input.onReviewToggle}
      />
      <SessionPanelToggle
        panel="inspector"
        open={input.inspectorOpen}
        onToggle={input.onInspectorToggle}
      />
    </>
  );
}

/**
 * An always-on agent's main (#1303): the agent's `…` menu owns its lifecycle, so the head keeps it
 * without linked work, Stop or the layout toggles. A panel that is still open (a kept preference,
 * or the inspector a subagent link opened) keeps its toggle as the way to close it.
 */
function alwaysOnMainActions(
  input: UseSessionTopbarSlotInput,
  instance: AgentInstancePayload,
  inWindow: boolean
) {
  return (
    <>
      <AlwaysOnAgentMenu instance={instance} />
      <SessionMarkSeenAction session={input.session} />
      {input.sidebarOpen ? sidebarToggle(input, inWindow) : null}
      {input.reviewOpen ? (
        <SessionPanelToggle panel="review" open onToggle={input.onReviewToggle} />
      ) : null}
      {input.inspectorOpen ? (
        <SessionPanelToggle panel="inspector" open onToggle={input.onInspectorToggle} />
      ) : null}
    </>
  );
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
  const { linkedWork } = input;
  const glyphState = sessionBadgeSignal(sessionBadgeOf(session), sessionWaitingOn(session)).state;

  // A main session is the agent's own conversation, so the agent names it. It accumulates linked
  // work indefinitely, so its head carries no linked chip (#1303).
  const instance = alwaysOnMainInstance(useAgentInstances(), session.id);
  const { registeredWorkspaces } = useActiveWorkspace();
  const workspaceNames = new Map(registeredWorkspaces.map(row => [row.id, row.name]));
  const linkedList = useLinkedWorkList(input, actions.lifecycleControllable, workspaceNames);

  useTopbarSlot({
    leading: inWindow ? (
      <WorkspaceBadge
        identity={workspaceBadgeIdentity(session.workspace_id ?? "", workspaceNames)}
        variant="chrome"
      />
    ) : undefined,
    glyph: instance ? (
      <AgentAvatar
        appearance={agentAvatarAppearance(instance.resolved_appearance, instance.agent_name)}
        label={instance.label}
        quietIdle
        size={20}
        spriteUrl={PROFILE_SPRITE_URL}
        state={agentInstanceAvatarState(instance)}
      />
    ) : (
      <StateGlyph size="sm" state={glyphState} />
    ),
    glyphPresentation: "state",
    crumb: instance?.label ?? getSessionDisplayTitle(session),
    status: (
      <span className="flex min-w-0 items-center gap-2">
        {instance ? <Pill size="sm">Always-on</Pill> : null}
        {/* Keyed by session: the head outlives a session switch, and an open list must not. */}
        {instance ? null : (
          <SessionLinkedWorkStatus
            key={session.id}
            {...linkedList}
            lead={linkedWork.lead}
            count={linkedWork.count}
            onClippedChange={linkedWork.setClipped}
            ownChip={ownChipActions(input)}
            ownFacts={sessionPullRequestFacts(session)}
            worktreeAction={input.worktreeAction}
          />
        )}
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
    // The run verbs lead; the panel toggles trail as quiet icons.
    actions: instance
      ? alwaysOnMainActions(input, instance, inWindow)
      : sessionActions(input, actions, linkedList, inWindow),
    // Per-session verbs live in the session list row menus; the head keeps only the view toggle.
    overflow: inWindow ? (
      <SessionViewToggle onValueChange={view.setView} value={view.view} />
    ) : undefined,
  });
}
