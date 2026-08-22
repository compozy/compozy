import { lazy, Suspense } from "react";

import { Spinner } from "@compozy/ui";

import {
  ProfileOwnerBanner,
  useProfileLens,
  useSwitchProfile,
  type ProfileOwner,
} from "@/systems/profiles";
import { SessionChatRuntimeProvider, type SessionPayload } from "@/systems/session";

import { loadSessionThread } from "./session-window-module-loader";

const SessionThread = lazy(() =>
  loadSessionThread().then(module => ({ default: module.SessionThread }))
);

export interface SessionProfileOwnerNoticeProps {
  owner: ProfileOwner;
  session: SessionPayload;
  agentName: string;
  windowId: string;
  liveTailEnabled: boolean;
}

/**
 * A deep link into another profile's session.
 *
 * Nothing is blocked and nothing is summarised away: the session came from the
 * labeled aggregate read, so its real transcript renders here. What is absent is
 * every way to act on it — the composer, the decision dock, the sidebar's row
 * actions, rename, delete, fork, and the runtime and environment controls are
 * not rendered at all rather than rendered inert, because a disabled control
 * still claims this surface is where that action belongs.
 *
 * The banner names the owner and offers the one move that changes the situation.
 * That is an operator gesture, so it goes through the same switch the menubar
 * uses and updates the remembered choice with it.
 */
export function SessionProfileOwnerNotice({
  owner,
  session,
  agentName,
  windowId,
  liveTailEnabled,
}: SessionProfileOwnerNoticeProps) {
  const lens = useProfileLens();
  const switchProfile = useSwitchProfile(lens);
  const workspaceId = session.workspace_id?.trim() ?? "";
  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      data-testid="session-profile-owner-view"
      data-window={windowId}
    >
      <div className="shrink-0 p-3 pb-0">
        <ProfileOwnerBanner
          noun="session"
          owner={owner}
          switchPending={switchProfile.isPending}
          onSwitch={() => switchProfile.mutate({ kind: "profile", profile: owner.name })}
        />
      </div>
      <SessionChatRuntimeProvider
        sessionId={session.id}
        workspaceId={workspaceId}
        liveTailEnabled={liveTailEnabled}
      >
        <Suspense
          fallback={
            <div className="flex min-h-0 flex-1 items-center justify-center">
              <Spinner className="size-5 text-subtle" />
            </div>
          }
        >
          <SessionThread
            readOnly
            canPrompt={false}
            liveDataEnabled={liveTailEnabled}
            sessionId={session.id}
            workspaceId={workspaceId}
            agentName={session.agent_name || agentName}
            acpSessionId={session.runtime.acp_session_id}
            sessionState={session.state}
            failure={session.failure}
          />
        </Suspense>
      </SessionChatRuntimeProvider>
    </div>
  );
}
