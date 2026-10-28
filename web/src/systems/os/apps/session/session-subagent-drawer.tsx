import { AuiConfig, AuiProvider } from "@assistant-ui/react";
import { SessionNativeSubagentActivity } from "@/components/assistant-ui/session-native-subagent-activity";
import { useSelector, useStore } from "@xstate/store-react";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpRight } from "lucide-react";
import { Button, ResizableDrawer, Tooltip, TooltipTrigger, TooltipContent } from "@compozy/ui";
import type { ReactNode } from "react";

import {
  SessionChatRuntimeProvider,
  sessionDetailOptions,
  subagentNavigationTarget,
} from "@/systems/session";
import {
  SubagentDrawerContext,
  subagentDrawerLogic,
  type SubagentNavigation,
  type SubagentView,
  SubagentRuntimeMeta,
  SubagentRowTrail,
  useSessionSubagents,
} from "@/systems/session";
import { SessionThread } from "./session-thread-lazy";

function SubagentTranscript({
  subagent,
  workspaceId,
  navigation,
}: {
  subagent: SubagentView;
  workspaceId: string;
  navigation: SubagentNavigation;
}) {
  const childId =
    subagent.origin === "provider_native"
      ? subagent.parent_session_id
      : (subagent.child_session_id ?? "");
  const query = useQuery(sessionDetailOptions(workspaceId, childId));
  if (!childId)
    return (
      <p role="status" className="p-4 text-small-body text-muted">
        {subagent.error ||
          (subagent.status === "queued" || subagent.status === "running"
            ? "Waiting for the subagent session to start."
            : "No session was created for this subagent.")}
      </p>
    );
  if (query.isError)
    return (
      <div role="alert" className="p-4">
        <p>Couldn't load the subagent session.</p>
        <Button variant="ghost" onClick={() => void query.refetch()}>
          Try again
        </Button>
      </div>
    );
  if (!query.data)
    return (
      <p role="status" className="p-4 text-small-body text-muted">
        Loading session…
      </p>
    );
  const session = query.data;
  if (subagent.origin === "provider_native")
    return (
      <SessionChatRuntimeProvider
        key={childId}
        sessionId={childId}
        workspaceId={workspaceId}
        liveTailEnabled
        readOnly
      >
        <SessionNativeSubagentActivity key={subagent.id} subagent={subagent} />
      </SessionChatRuntimeProvider>
    );
  return (
    <SessionChatRuntimeProvider
      key={childId}
      sessionId={childId}
      workspaceId={workspaceId}
      liveTailEnabled
      readOnly
    >
      <SessionThread
        readOnly
        canPrompt={false}
        liveDataEnabled
        sessionId={childId}
        workspaceId={workspaceId}
        agentName={session.agent_name}
        acpSessionId={session.runtime.acp_session_id}
        sessionState={session.state}
        failure={session.failure}
        subagentNavigation={navigation}
      />
    </SessionChatRuntimeProvider>
  );
}

/** The selected parent roster owns live status and admission, including pending child IDs. */
function SubagentDrawerSelection({
  selected,
  navigation,
  children,
  width,
  onWidthChange,
  onClose,
  returnFocus,
}: {
  selected: { subagent: SubagentView; workspaceId: string } | null;
  navigation: SubagentNavigation;
  children: ReactNode;
  width: number;
  onWidthChange: (width: number) => void;
  onClose: () => void;
  returnFocus: HTMLElement | null;
}) {
  const roster = useSessionSubagents(
    selected?.workspaceId ?? "",
    selected?.subagent.parent_session_id ?? "",
    { enabled: selected !== null }
  );
  const subagent = selected
    ? (roster.subagents.find(row => row.id === selected.subagent.id) ?? selected.subagent)
    : null;
  const target =
    selected && subagent ? subagentNavigationTarget(subagent, selected.workspaceId) : null;
  return (
    <ResizableDrawer
      open={selected !== null}
      onOpenChange={open => {
        if (!open) onClose();
      }}
      width={width}
      onWidthChange={onWidthChange}
      returnFocus={returnFocus}
      title={
        subagent ? (
          <span className="flex min-w-0 flex-col gap-1">
            <span className="truncate">{subagent.title}</span>
            <SubagentRuntimeMeta subagent={subagent} />
            <SubagentRowTrail subagent={subagent} glyph />
          </span>
        ) : (
          "Subagent"
        )
      }
      actions={
        target && subagent?.origin !== "provider_native" ? (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant="quiet"
                  size="icon-sm"
                  aria-label="Open as session"
                  onClick={event => {
                    onClose();
                    navigation.openSession(target, { newWindow: event.metaKey || event.ctrlKey });
                  }}
                >
                  <ArrowUpRight />
                </Button>
              }
            />
            <TooltipContent>Open as session</TooltipContent>
          </Tooltip>
        ) : null
      }
      content={
        subagent && selected ? (
          <>
            <AuiProvider extends={null} config={AuiConfig({})}>
              <SubagentTranscript
                subagent={subagent}
                workspaceId={selected.workspaceId}
                navigation={navigation}
              />
            </AuiProvider>
          </>
        ) : null
      }
    >
      {children}
    </ResizableDrawer>
  );
}

export function SessionSubagentDrawer({
  children,
  openSession,
}: {
  children: ReactNode;
  openSession: SubagentNavigation["openSession"];
}) {
  const store = useStore(subagentDrawerLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  return (
    <SubagentDrawerContext
      value={{
        store,
        open: (subagent, options, workspaceId) => {
          const target = subagentNavigationTarget(subagent, workspaceId);
          if (options.newWindow && subagent.origin !== "provider_native") {
            if (target) {
              store.trigger.closed();
              openSession(target, options);
            }
            return;
          }
          store.trigger.opened({
            subagent,
            workspaceId,
            trigger: document.activeElement instanceof HTMLElement ? document.activeElement : null,
          });
        },
      }}
    >
      <SubagentDrawerSelection
        selected={state.selected}
        width={state.width}
        returnFocus={state.returnFocus}
        onWidthChange={width => store.trigger.resized({ width })}
        onClose={() => store.trigger.closed()}
        navigation={{ openSession }}
      >
        {children}
      </SubagentDrawerSelection>
    </SubagentDrawerContext>
  );
}
