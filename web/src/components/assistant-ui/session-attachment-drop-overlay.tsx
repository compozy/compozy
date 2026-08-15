import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { useSessionComposerDrop } from "./hooks/use-session-composer-drop";

export function SessionComposerDropRoot({
  children,
  disabled = false,
}: {
  children: (state: { isDragging: boolean }) => ReactNode;
  disabled?: boolean;
}) {
  const { isDragging, dropProps } = useSessionComposerDrop();
  const active = !disabled && isDragging;
  return (
    <div className="relative" {...(disabled ? {} : dropProps)}>
      {children({ isDragging: active })}
      <SessionAttachmentDropOverlay visible={active} />
    </div>
  );
}

export function SessionAttachmentDropOverlay({ visible }: { visible: boolean }) {
  return (
    <div
      data-testid="composer-drop-overlay"
      aria-hidden={!visible}
      className={cn(
        "pointer-events-none absolute inset-0 z-2 grid place-items-center rounded-[inherit] p-4 text-center",
        "bg-[color-mix(in_oklch,var(--elevated)_88%,transparent)] shadow-[inset_0_0_0_1px_var(--accent-dim)]",
        visible ? "grid" : "hidden"
      )}
    >
      <div>
        <b className="block text-[13px] font-medium text-fg-strong">Drop files</b>
        <span className="mt-[3px] block text-[11px] text-subtle">
          Images, PDF, Markdown, or text
        </span>
      </div>
    </div>
  );
}
