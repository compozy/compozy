"use client";

import { cn } from "@compozy/ui";
import type { ReactNode } from "react";

import { useModalDialog } from "@/components/docs/hooks/use-modal-dialog";

/** A modal overlay on the native dialog: focus stays inside, Escape and a backdrop click close it. */
export function DocsModal(props: {
  label: string;
  onClose: () => void;
  className?: string;
  children: ReactNode;
}) {
  const ref = useModalDialog();
  return (
    <dialog
      ref={ref}
      aria-label={props.label}
      onClose={props.onClose}
      onCancel={event => {
        event.preventDefault();
        props.onClose();
      }}
      onClick={event => {
        // The dialog's own box is covered by its content, so a click that lands on it is the backdrop.
        if (event.target === event.currentTarget) props.onClose();
      }}
      className={cn(
        "p-0 text-fg backdrop:bg-site-dark-700 backdrop:backdrop-blur-sm",
        props.className
      )}
    >
      {props.children}
    </dialog>
  );
}
