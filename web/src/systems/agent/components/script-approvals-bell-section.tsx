/**
 * The bell's "Scripts to approve" group: one line per waiting script with its agent and Approve,
 * then a way to review them on Heartbeat. Rendered above Needs you by the shell's bell.
 */
import { SquareTerminal } from "@compozy/ui/icons";

import { Button, Eyebrow, Icon } from "@compozy/ui";

import type { ScriptApprovalItem } from "../lib/agent-script-approvals";

export interface ScriptApprovalsBellSectionProps {
  items: readonly ScriptApprovalItem[];
  approving: string | null;
  onApprove: (item: ScriptApprovalItem) => void;
  onReview: (item: ScriptApprovalItem) => void;
}

/** Mark, two lines and a trailing control: the frame every bell row shares. */
const ROW =
  "grid w-full grid-cols-[--spacing(4.5)_minmax(0,1fr)] items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left transition-colors hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none";

function ScriptRow({
  item,
  busy,
  onApprove,
  onReview,
}: {
  item: ScriptApprovalItem;
  busy: boolean;
  onApprove: (item: ScriptApprovalItem) => void;
  onReview: (item: ScriptApprovalItem) => void;
}) {
  return (
    <div className="flex min-w-0 items-center gap-1 pr-1" data-testid="os-bell-script-approval">
      <button
        aria-label={`Review ${item.script} on ${item.agentLabel}'s Heartbeat`}
        className={ROW}
        onClick={() => onReview(item)}
        type="button"
      >
        <span
          aria-hidden="true"
          className="grid size-4.5 shrink-0 place-items-center rounded-full bg-brand-tint text-brand"
        >
          <Icon as={SquareTerminal} size="xs" />
        </span>
        <span className="flex min-w-0 flex-col gap-px">
          <span className="truncate text-small-body font-medium text-fg-strong">{item.script}</span>
          <span className="truncate text-eyebrow text-subtle">{item.agentLabel}</span>
        </span>
      </button>
      <Button
        aria-busy={busy || undefined}
        aria-label={`Approve ${item.script} for ${item.agentLabel}`}
        disabled={busy}
        onClick={() => onApprove(item)}
        size="xs"
        type="button"
        variant="secondary"
      >
        Approve
      </Button>
    </div>
  );
}

export function ScriptApprovalsBellSection({
  items,
  approving,
  onApprove,
  onReview,
}: ScriptApprovalsBellSectionProps) {
  if (items.length === 0) return null;
  return (
    <section
      className="flex flex-col border-line pt-1.5 first:pt-0 [&+&]:mt-1 [&+&]:border-t"
      data-testid="os-bell-scripts"
    >
      <Eyebrow className="flex items-center gap-1.5 px-2.5 pt-1.5 pb-1 text-subtle">
        Scripts to approve
        <span className="font-mono text-micro text-faint">{items.length}</span>
      </Eyebrow>
      {items.map(item => (
        <ScriptRow
          busy={approving === item.key}
          item={item}
          key={item.key}
          onApprove={onApprove}
          onReview={onReview}
        />
      ))}
      <Button
        className="mx-1 mt-1 self-start"
        onClick={() => onReview(items[0]!)}
        size="xs"
        type="button"
        variant="link"
      >
        Review in Heartbeat
      </Button>
    </section>
  );
}
