import { useState, type ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import { ConfirmDialog } from "@agh/ui";

import { useDeleteAgent } from "./use-agents";
import type { AgentPayload, DeleteAgentResponse } from "../types";

export interface UseAgentDeleteFlowOptions {
  agent: AgentPayload | undefined;
  workspaceId: string | null;
}

export interface UseAgentDeleteFlowResult {
  open: boolean;
  openDialog: () => void;
  closeDialog: () => void;
  confirmDialog: ReactNode;
  isDeleting: boolean;
}

export function useAgentDeleteFlow({
  agent,
  workspaceId,
}: UseAgentDeleteFlowOptions): UseAgentDeleteFlowResult {
  const navigate = useNavigate();
  const deleteAgent = useDeleteAgent();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const openDialog = () => {
    setError(null);
    setOpen(true);
  };

  const closeDialog = () => {
    if (deleteAgent.isPending) return;
    setOpen(false);
    setError(null);
  };

  const onConfirm = () => {
    if (!agent) return;
    setError(null);
    deleteAgent.mutate(
      { name: agent.name, workspace: workspaceId },
      {
        onSuccess: (result: DeleteAgentResponse) => {
          setOpen(false);
          const unshadowed = result.unshadowed_origin;
          if (unshadowed) {
            toast.success("Agent deleted", {
              description: `The ${unshadowed} definition is now active.`,
            });
          } else {
            toast.success("Agent deleted");
          }
          void navigate({ to: "/agents" });
        },
        onError: err => {
          setError(err instanceof Error ? err.message : "Couldn't delete agent");
        },
      }
    );
  };

  const baseDescription = agent ? (
    <>
      This removes the agent definition for <code className="font-mono text-fg">{agent.name}</code>.
      Existing session records are kept.
    </>
  ) : null;
  const description = agent ? (
    agent.origin === "workspace" ? (
      <>
        {baseDescription} If a global definition with this name exists, it will become active after
        delete.
      </>
    ) : (
      baseDescription
    )
  ) : null;

  const confirmDialog = (
    <ConfirmDialog
      open={open && Boolean(agent)}
      tone="danger"
      title="Delete agent"
      description={description}
      error={error}
      isPending={deleteAgent.isPending}
      confirmLabel={deleteAgent.isPending ? "Deleting…" : "Delete agent"}
      cancelLabel="Cancel"
      confirmTyping={agent?.name}
      contentProps={{ "data-testid": "agent-delete-dialog" }}
      descriptionProps={{ "data-testid": "agent-delete-description" }}
      errorProps={{ "data-testid": "agent-delete-error" }}
      confirmButtonProps={{ "data-testid": "agent-delete-confirm" }}
      cancelButtonProps={{ "data-testid": "agent-delete-cancel" }}
      confirmInputProps={{ "data-testid": "agent-delete-confirm-typing" }}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) closeDialog();
      }}
    />
  );

  return {
    open,
    openDialog,
    closeDialog,
    confirmDialog,
    isDeleting: deleteAgent.isPending,
  };
}
