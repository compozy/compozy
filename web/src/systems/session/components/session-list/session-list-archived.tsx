import { ChevronRight } from "lucide-react";

import { Button, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListRow } from "./session-list-row";

export interface SessionListArchivedProps {
  sessions: readonly SessionPayload[];
  total?: number;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

export function SessionListArchived({
  sessions,
  total,
  onSelectSession,
  sessionActions,
  testIdPrefix,
}: SessionListArchivedProps) {
  if (sessions.length === 0) return null;

  return (
    <section className="px-2.5 py-4" data-testid={`${testIdPrefix}-archived`}>
      <Collapsible>
        <CollapsibleTrigger
          className="group/session-list-archived"
          render={
            <Button
              type="button"
              variant="ghost"
              size="sm"
              aria-label={
                total === undefined ? "Archived sessions" : `Archived sessions (${total})`
              }
              className="w-full justify-start gap-2 pl-2 text-small-body text-subtle"
            />
          }
        >
          <ChevronRight className="size-3 transition-transform group-data-panel-open/session-list-archived:rotate-90" />
          Archived
          {total === undefined ? null : (
            <span className="font-mono text-micro text-faint">{total}</span>
          )}
        </CollapsibleTrigger>
        <CollapsibleContent className="pb-1">
          {sessions.map(session => (
            <SessionListRow
              key={session.id}
              session={session}
              onSelect={() => onSelectSession(session)}
              sessionActions={sessionActions}
              testIdPrefix={testIdPrefix}
            />
          ))}
        </CollapsibleContent>
      </Collapsible>
    </section>
  );
}
