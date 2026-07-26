import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  Eyebrow,
  Icon,
  PillDot,
  SearchInput,
  StatusDot,
  Time,
  type PillTone,
} from "@agh/ui";

import { cn } from "@/lib/utils";
import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";

import { useOsSessionsModal } from "../hooks/use-os-sessions-modal";

type ModalView = "recent" | "all";

interface SessionGroup {
  agentName: string;
  sessions: SessionPayload[];
}

export interface OsSessionsModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessions: readonly SessionPayload[];
  disconnected: boolean;
}

function statusTone(badge: string): PillTone {
  if (badge === "running") return "accent";
  if (badge === "idle") return "success";
  if (badge === "waiting-for-auth" || badge === "hung") return "warning";
  if (badge === "failed" || badge === "unhealthy") return "danger";
  return "neutral";
}

function SessionStatusMark({ badge }: { badge: string }) {
  if (badge === "stopped") {
    return <StatusDot tone="faint" variant="ring" label="stopped" />;
  }
  return <PillDot tone={statusTone(badge)} pulse={badge === "running"} size="sm" />;
}

function SessionRow({ session, onSelect }: { session: SessionPayload; onSelect: () => void }) {
  return (
    <button
      type="button"
      className="grid w-full grid-cols-[8px_minmax(0,1fr)_auto] items-start gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
      data-status={session.badge}
      data-testid={`os-sessions-modal-session-${session.id}`}
      onClick={onSelect}
    >
      <span className="mt-1.5 grid place-items-center">
        <SessionStatusMark badge={session.badge} />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-small-body text-fg-strong">
          {getSessionDisplayTitle(session)}
        </span>
        <span className="block truncate text-micro text-subtle">
          <span className="font-medium text-muted">{session.agent_name}</span>
          <span aria-hidden="true"> · </span>
          {session.badge}
        </span>
      </span>
      <Time iso={session.updated_at} className="mt-0.5 font-mono text-micro text-subtle" />
    </button>
  );
}

function SessionsModalBody({
  sessions,
  disconnected,
  collapsedAgentIds,
  onToggleGroup,
  onSelectSession,
  onClose,
}: {
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedAgentIds: readonly string[];
  onToggleGroup: (agentName: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onClose: () => void;
}) {
  const [view, setView] = useState<ModalView>("recent");
  const [filter, setFilter] = useState("");
  const normalizedFilter = filter.trim().toLocaleLowerCase();
  const filtered = sessions.filter(session => {
    if (normalizedFilter === "") return true;
    return (
      getSessionDisplayTitle(session).toLocaleLowerCase().includes(normalizedFilter) ||
      session.agent_name.toLocaleLowerCase().includes(normalizedFilter)
    );
  });
  const byAgent = new Map<string, SessionPayload[]>();
  for (const session of filtered) {
    const current = byAgent.get(session.agent_name) ?? [];
    current.push(session);
    byAgent.set(session.agent_name, current);
  }
  const groups: SessionGroup[] = [...byAgent.entries()].map(([agentName, groupedSessions]) => ({
    agentName,
    sessions: groupedSessions,
  }));
  const collapsedAgents = new Set(collapsedAgentIds);

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="os-sessions-modal-content">
      <div className="flex items-center gap-2 px-3 pt-3 pb-1.5">
        {view === "all" ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Back to recent sessions"
            onClick={() => setView("recent")}
          >
            <Icon as={ChevronLeft} size="sm" />
          </Button>
        ) : null}
        <Eyebrow className="min-w-0 flex-1 text-subtle">
          Sessions <span className="ml-1 text-faint">{filtered.length}</span>
        </Eyebrow>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Close sessions"
          onClick={onClose}
        >
          <Icon as={X} size="sm" />
        </Button>
      </div>
      <div className="px-3 py-1.5">
        <SearchInput
          value={filter}
          onChange={setFilter}
          placeholder="Filter sessions…"
          aria-label="Filter sessions"
          containerClassName="min-w-0"
        />
      </div>
      {disconnected ? (
        <p
          className="mx-3 my-1 rounded-md border border-warning/30 bg-warning-tint px-2.5 py-2 text-small-body text-warning"
          role="status"
        >
          Session updates are unavailable. Cached sessions remain visible.
        </p>
      ) : null}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div
          className={cn(
            "flex h-full min-h-0 w-[200%] transition-transform duration-shell-slow ease-spring",
            view === "all" && "-translate-x-1/2"
          )}
          data-view={view}
        >
          <div className="flex h-full w-1/2 shrink-0 flex-col">
            <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5">
              {filtered.slice(0, 6).map(session => (
                <SessionRow
                  key={session.id}
                  session={session}
                  onSelect={() => onSelectSession(session)}
                />
              ))}
              {filtered.length === 0 ? (
                <p className="px-3 py-8 text-center text-small-body text-muted">
                  No sessions match.
                </p>
              ) : null}
            </div>
            <button
              type="button"
              className="flex w-full shrink-0 items-center justify-center gap-1.5 border-t border-line-soft px-2 py-2.5 text-small-body font-medium text-subtle transition-colors hover:bg-row-hover hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={() => setView("all")}
            >
              Show all sessions
              <Icon as={ChevronRight} size="sm" />
            </button>
          </div>
          <div className="h-full w-1/2 shrink-0 overflow-y-auto px-2.5 pb-3">
            {groups.map(group => {
              const collapsed = collapsedAgents.has(group.agentName);
              return (
                <section
                  key={group.agentName}
                  className="border-b border-line-soft py-1 last:border-b-0"
                >
                  <button
                    type="button"
                    className="flex min-h-7 w-full items-center gap-2 rounded-sm px-2 py-1 text-left hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
                    aria-expanded={!collapsed}
                    onClick={() => onToggleGroup(group.agentName)}
                  >
                    <span className="grid size-5 place-items-center rounded-sm bg-elevated font-mono text-micro font-semibold text-muted">
                      {group.agentName.slice(0, 2).toUpperCase()}
                    </span>
                    <span className="min-w-0 flex-1 truncate text-small-body font-medium text-fg-strong">
                      {group.agentName}
                    </span>
                    <span className="font-mono text-micro text-faint">{group.sessions.length}</span>
                    <Icon
                      as={ChevronRight}
                      size="sm"
                      className={cn("text-faint transition-transform", !collapsed && "rotate-90")}
                    />
                  </button>
                  <div
                    className={cn(
                      "grid transition-[grid-template-rows] duration-base",
                      collapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]"
                    )}
                  >
                    <div className="min-h-0 overflow-hidden pl-2">
                      {group.sessions.map(session => (
                        <SessionRow
                          key={session.id}
                          session={session}
                          onSelect={() => onSelectSession(session)}
                        />
                      ))}
                    </div>
                  </div>
                </section>
              );
            })}
            {groups.length === 0 ? (
              <p className="px-3 py-8 text-center text-small-body text-muted">No sessions match.</p>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Global sessions catalog (shell-level Dialog). Same filter / recent↔all body
 * as the former rail; chrome matches ⌘K (portal, scrim, top offset).
 */
export function OsSessionsModal({
  open,
  onOpenChange,
  sessions,
  disconnected,
}: OsSessionsModalProps) {
  const { coordinator, store, collapsedAgentIds } = useOsSessionsModal();

  const selectSession = (session: SessionPayload) => {
    onOpenChange(false);
    void coordinator.userOpen({
      app: "session",
      instanceKey: session.id,
      route: {
        pathname: `/agents/${encodeURIComponent(session.agent_name)}/sessions/${encodeURIComponent(session.id)}`,
        search: {},
      },
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        unframed
        showCloseButton={false}
        data-testid="os-sessions-modal"
        className="top-[9vh] flex h-[min(var(--height-modal-md),70vh)] max-h-[70vh] w-full translate-y-0 flex-col overflow-hidden min-[960px]:top-[16vh] sm:w-detail-inspector-inline sm:max-w-none"
      >
        <DialogTitle className="sr-only">Sessions</DialogTitle>
        <DialogDescription className="sr-only">
          Filter sessions and open one in the current workspace.
        </DialogDescription>
        <SessionsModalBody
          sessions={sessions}
          disconnected={disconnected}
          collapsedAgentIds={collapsedAgentIds}
          onToggleGroup={agentName => store.getState().toggleRailGroup(agentName)}
          onSelectSession={selectSession}
          onClose={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}
