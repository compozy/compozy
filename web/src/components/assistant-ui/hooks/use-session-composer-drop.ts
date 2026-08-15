import { useAui, useAuiState } from "@assistant-ui/react";
import { useState, type DragEvent } from "react";

import { ATTACHMENT_MAX_COUNT } from "@/systems/session/lib/attachment-kinds";

function transferHasFiles(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types ?? []).includes("Files");
}

function onDragOver(event: DragEvent<HTMLElement>) {
  if (!transferHasFiles(event)) return;
  event.preventDefault();
  event.dataTransfer.dropEffect = "copy";
}

export function useSessionComposerDrop() {
  const aui = useAui();
  const attachmentCount = useAuiState(state => state.composer.attachments.length);
  const [dragDepth, setDragDepth] = useState(0);
  const isDragging = dragDepth > 0;

  const addFiles = (files: File[]) => {
    const remaining = Math.max(0, ATTACHMENT_MAX_COUNT - attachmentCount);
    for (const file of files.slice(0, remaining)) {
      void aui.composer.addAttachment(file);
    }
  };

  const onDragEnter = (event: DragEvent<HTMLElement>) => {
    if (!transferHasFiles(event)) return;
    event.preventDefault();
    setDragDepth(depth => depth + 1);
  };

  const onDragLeave = (event: DragEvent<HTMLElement>) => {
    if (!transferHasFiles(event)) return;
    event.preventDefault();
    setDragDepth(depth => Math.max(0, depth - 1));
  };

  const onDrop = (event: DragEvent<HTMLElement>) => {
    if (!transferHasFiles(event)) return;
    event.preventDefault();
    setDragDepth(0);
    addFiles(Array.from(event.dataTransfer.files));
  };

  return {
    isDragging,
    dropProps: { onDragEnter, onDragOver, onDragLeave, onDrop },
  };
}
