import { type Dispatch, type SetStateAction, useState } from "react";

import type { KnowledgeMemoryItem } from "../types";

type KnowledgeDialogKey = "confirmDeleteOpen" | "editOpen";

interface KnowledgeDialogState {
  memoryIdentity: string;
  confirmDeleteOpen: boolean;
  editOpen: boolean;
}

function setKnowledgeDialogOpen(
  setDialogState: Dispatch<SetStateAction<KnowledgeDialogState>>,
  memoryIdentity: string,
  key: KnowledgeDialogKey,
  open: boolean
) {
  setDialogState(previous => ({ ...previous, memoryIdentity, [key]: open }));
}

function knowledgeDialogMemoryIdentity(memory: KnowledgeMemoryItem | undefined): string {
  if (!memory) return "";
  if (memory.key) return memory.key;

  return [
    memory.scope,
    memory.workspace_id ?? "",
    memory.agent_name ?? "",
    memory.agent_tier ?? "",
    memory.filename,
  ].join(":");
}

/**
 * Delete/edit dialog visibility, scoped to the memory it opened for: switching
 * memories closes both without an Effect.
 */
function useKnowledgeDetailDialogs(memory: KnowledgeMemoryItem | undefined) {
  const memoryIdentity = knowledgeDialogMemoryIdentity(memory);
  const [dialogState, setDialogState] = useState<KnowledgeDialogState>({
    memoryIdentity,
    confirmDeleteOpen: false,
    editOpen: false,
  });

  if (dialogState.memoryIdentity !== memoryIdentity) {
    setDialogState({ memoryIdentity, confirmDeleteOpen: false, editOpen: false });
  }

  const isCurrentDialogState = dialogState.memoryIdentity === memoryIdentity;
  const setOpen = (key: KnowledgeDialogKey) => (open: boolean) =>
    setKnowledgeDialogOpen(setDialogState, memoryIdentity, key, open);

  return {
    confirmDeleteOpen: isCurrentDialogState && dialogState.confirmDeleteOpen,
    editOpen: isCurrentDialogState && dialogState.editOpen,
    setDeleteOpen: setOpen("confirmDeleteOpen"),
    setEditOpen: setOpen("editOpen"),
  };
}

export { useKnowledgeDetailDialogs };
