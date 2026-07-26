import { AlertCircle } from "lucide-react";

import { Spinner } from "@agh/ui";

import { useSessionWorkspaceGuard } from "@/hooks/routes/use-session-workspace-guard";
import { SessionChatRuntimeProvider, useSessionById } from "@/systems/session";

import { SessionWindowContent } from "./session-window-content";

export function SessionWindowView({
  name,
  id,
  workspaceId,
  onDeleteSuccess,
}: {
  name: string;
  id: string;
  workspaceId: string;
  onDeleteSuccess: () => void;
}) {
  const { data: session, isLoading, error } = useSessionById(id, workspaceId);
  const sessionWorkspaceId = session?.workspace_id?.trim();

  useSessionWorkspaceGuard({
    sessionWorkspaceId,
    agentName: name,
    sessionId: id,
    sessionName: session?.name,
  });

  if (isLoading) {
    return (
      <div
        className="flex min-h-full items-center justify-center"
        data-testid="session-route-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }
  if (!session || !sessionWorkspaceId) {
    return <SessionWindowNotice message={error?.message ?? "Session not found"} />;
  }

  const resolvedAgentName = session.agent_name || name;
  return (
    <SessionChatRuntimeProvider key={id} sessionId={id} workspaceId={sessionWorkspaceId}>
      <SessionWindowContent
        agentName={resolvedAgentName}
        sessionId={id}
        session={session}
        workspaceId={sessionWorkspaceId}
        onDeleteSuccess={onDeleteSuccess}
      />
    </SessionChatRuntimeProvider>
  );
}

export function SessionWindowNotice({ message }: { message: string }) {
  return (
    <div className="flex min-h-full items-center justify-center">
      <div className="flex flex-col items-center gap-2 text-center">
        <AlertCircle className="size-6 text-danger" />
        <p className="text-small-body text-subtle">{message}</p>
      </div>
    </div>
  );
}
