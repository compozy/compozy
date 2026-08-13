import { ExternalLinkIcon, GitPullRequestDraftIcon, GitPullRequestIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Kbd, Spinner, cn } from "@compozy/ui";

export type WorktreePrActionKey = "draft" | "create" | "view" | "browser";

export interface WorktreePrActionRow {
  key: WorktreePrActionKey;
  label: string;
  primary?: boolean;
  blocked?: boolean;
  /** Verbatim daemon reason; also mirrored on `title`. */
  reason?: string;
  url?: string;
}

interface WorktreePrActionRowsProps {
  rows: readonly WorktreePrActionRow[];
  submittingKey?: WorktreePrActionKey;
  onSelect: (row: WorktreePrActionRow) => void;
}

const ICON: Record<WorktreePrActionKey, ReactNode> = {
  draft: <GitPullRequestDraftIcon aria-hidden="true" />,
  create: <GitPullRequestIcon aria-hidden="true" />,
  view: <ExternalLinkIcon aria-hidden="true" />,
  browser: <ExternalLinkIcon aria-hidden="true" />,
};

/**
 * The PR step's actions, as rows rather than footer buttons.
 *
 * Draft is a peer of create, not a checkbox — it is a different request, and
 * burying it in a toggle hides that. Every row here exists because the payload
 * offers it; nothing is rendered speculatively.
 */
export function WorktreePrActionRows({ rows, submittingKey, onSelect }: WorktreePrActionRowsProps) {
  return (
    <div className="flex flex-col gap-1.5" data-slot="worktree-pr-action-rows">
      {rows.map(row => (
        <button
          aria-disabled={row.blocked || undefined}
          className={cn(
            "flex min-h-[34px] w-full items-center gap-[9px] rounded-md border border-line bg-canvas-tint px-3 text-small-body font-medium text-fg",
            "transition-colors duration-base ease-out hover:border-line-strong hover:bg-row-hover",
            "[&_svg]:size-3.5 [&_svg]:shrink-0 [&_svg]:text-muted",
            row.primary &&
              "border-transparent bg-accent text-accent-ink shadow-highlight hover:bg-accent-hover [&_svg]:text-accent-ink",
            row.blocked && "cursor-not-allowed opacity-50"
          )}
          data-action={row.key}
          data-blocked={row.blocked ? "" : undefined}
          data-primary={row.primary ? "" : undefined}
          data-slot="worktree-pr-action"
          disabled={row.blocked || submittingKey !== undefined}
          key={row.key}
          onClick={() => {
            if (!row.blocked) onSelect(row);
          }}
          title={row.reason}
          type="button"
        >
          {submittingKey === row.key ? <Spinner className="size-3" /> : ICON[row.key]}
          <span className="min-w-0 flex-1 truncate text-start">{row.label}</span>
          {row.primary ? <Kbd className="ml-auto">⌘↵</Kbd> : null}
        </button>
      ))}
    </div>
  );
}
