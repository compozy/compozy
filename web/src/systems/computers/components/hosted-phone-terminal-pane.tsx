import { AlertCircle, FolderOpen, TerminalSquare } from "@compozy/ui/icons";

import { Button, Empty } from "@compozy/ui";

import {
  TerminalStoreProvider,
  TerminalWindowApp,
  TerminalWindowSkeleton,
} from "@/systems/terminal";

import { useHostedPhoneTerminal } from "../hooks/use-hosted-phone-terminal";

export interface HostedPhoneTerminalPaneProps {
  workspaceId: string;
  projectLabel?: string;
}

function HostedPhoneTerminalBody({ workspaceId, projectLabel }: HostedPhoneTerminalPaneProps) {
  const terminal = useHostedPhoneTerminal(workspaceId, true);

  if (workspaceId.trim() === "") {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <Empty
          description="Open a session or create a project on this computer to use Terminal."
          icon={FolderOpen}
          title="No project available"
        />
      </div>
    );
  }

  if (terminal.isPending) {
    return <TerminalWindowSkeleton label="Loading terminals" />;
  }

  if (terminal.error) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <Empty
          action={
            <Button
              onClick={() => void terminal.refetch()}
              size="sm"
              type="button"
              variant="secondary"
            >
              Retry
            </Button>
          }
          description={terminal.error instanceof Error ? terminal.error.message : undefined}
          icon={AlertCircle}
          title="Couldn't load Terminal"
        />
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="phone-terminal-pane">
      {terminal.terminals.length > 1 ? (
        <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-line-soft bg-sunken px-3 py-1.5">
          {terminal.terminals.map(item => {
            const active = item.id === terminal.selectedTerminalId;
            return (
              <Button
                key={item.id}
                onClick={() => terminal.setSelectedTerminalId(item.id)}
                size="xs"
                type="button"
                variant={active ? "secondary" : "ghost"}
              >
                <TerminalSquare aria-hidden="true" className="size-3" />
                <span className="max-w-32 truncate">{item.title || item.id}</span>
              </Button>
            );
          })}
        </div>
      ) : null}
      <TerminalWindowApp
        actions={terminal.actions}
        hostChrome
        inputRequestTitles={new Map(terminal.terminals.map(item => [item.id, item.title]))}
        inputRequests={terminal.inputRequests}
        journal={null}
        profile={terminal.profile}
        projectLabel={projectLabel}
        requestedTerminalId={terminal.selectedTerminalId}
        resolveReady={terminal.isFetchedAfterMount}
        resolvedInputRequests={terminal.resolvedInputRequests}
        terminals={terminal.terminals}
        viewerId={null}
        workspaceId={workspaceId}
      />
    </div>
  );
}

/**
 * Composes the real `TerminalWindowApp` inside a dedicated `TerminalStoreProvider`
 * for the hosted phone Terminal tab (never forks the terminal emulator).
 */
export function HostedPhoneTerminalPane(props: HostedPhoneTerminalPaneProps) {
  return (
    <TerminalStoreProvider>
      <HostedPhoneTerminalBody {...props} />
    </TerminalStoreProvider>
  );
}
