import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { useSessionComposerDrop } from "./hooks/use-session-composer-drop";

export function SessionComposerDropRoot({
  children,
  disabled = false,
}: {
  children: ReactNode;
  disabled?: boolean;
}) {
  const { isDragging, dropProps } = useSessionComposerDrop({ disabled });
  return (
    <div className="group/drop relative" data-dragging={isDragging || undefined} {...dropProps}>
      {children}
      <SessionAttachmentDropOverlay visible={isDragging} />
    </div>
  );
}

function SessionAttachmentDropOverlay({ visible }: { visible: boolean }) {
  return (
    <div
      data-testid="composer-drop-overlay"
      aria-hidden={!visible}
      className={cn(
        "pointer-events-none absolute inset-0 z-2 hidden place-items-center rounded-lg",
        "border border-accent-dim bg-elevated/90 p-4 text-center",
        "group-data-[dragging=true]/drop:grid"
      )}
    >
      <div>
        <b className="block text-small-body font-medium text-fg-strong">Drop files</b>
        <span className="mt-1 block text-micro text-subtle">Images, PDF, Markdown, or text</span>
      </div>
    </div>
  );
}
