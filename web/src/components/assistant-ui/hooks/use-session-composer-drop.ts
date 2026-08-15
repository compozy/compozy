import { useAui } from "@assistant-ui/react";
import { useEffect, useState, type DragEvent } from "react";

import { ATTACHMENT_MAX_COUNT } from "@/systems/session/lib/attachment-kinds";

function transferHasFiles(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types ?? []).includes("Files");
}

export function useSessionComposerDrop({ disabled = false }: { disabled?: boolean } = {}) {
  const aui = useAui();
  const [dragDepth, setDragDepth] = useState(0);
  const isDragging = dragDepth > 0;

  useEffect(() => {
    if (disabled) setDragDepth(0);
  }, [disabled]);

  const addFiles = (files: File[]) => {
    const currentCount = aui.composer.getState().attachments.length;
    const remaining = Math.max(0, ATTACHMENT_MAX_COUNT - currentCount);
    for (const file of files.slice(0, remaining)) {
      void aui.composer.addAttachment(file);
    }
  };

  const onDragEnter = (event: DragEvent<HTMLElement>) => {
    if (!transferHasFiles(event)) return;
    event.preventDefault();
    if (disabled) return;
    setDragDepth(depth => depth + 1);
  };

  const onDragOver = (event: DragEvent<HTMLElement>) => {
    if (!transferHasFiles(event)) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = disabled ? "none" : "copy";
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
    if (disabled) return;
    addFiles(Array.from(event.dataTransfer.files));
  };

  return {
    isDragging: !disabled && isDragging,
    dropProps: { onDragEnter, onDragOver, onDragLeave, onDrop },
  };
}
