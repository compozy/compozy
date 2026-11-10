// Governance tag. approved = decided and binding; proposal = direction until the founder approves it.
import { cn } from "@compozy/ui/utils";

import type { Status } from "../chapters";

const LABEL: Record<Status, string> = { approved: "Approved", proposal: "Proposal" };
const GLYPH: Record<Status, string> = { approved: "✓", proposal: "◌" };
const TITLE: Record<Status, string> = {
  approved: "Approved: binding rule",
  proposal: "Proposal: direction until approved",
};

export function StatusTag({ status, className }: { status: Status; className?: string }) {
  return (
    <span
      data-status={status}
      title={TITLE[status]}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 mono-label text-[11px] font-medium",
        status === "approved"
          ? "bg-[color-mix(in_oklab,var(--b-accent)_14%,transparent)] text-(--b-accent)"
          : "border border-dashed border-(--b-muted) text-(--b-muted)",
        className
      )}
    >
      <span aria-hidden="true">{GLYPH[status]}</span>
      {LABEL[status]}
    </span>
  );
}
