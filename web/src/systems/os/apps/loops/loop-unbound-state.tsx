import { Repeat2 } from "@compozy/ui/icons";

import { Empty } from "@compozy/ui";

import type { DataScope } from "@/systems/workspace";

interface LoopUnboundStateProps {
  scope: DataScope | null;
  /** What to open the item from, e.g. "Open this run from Runs." */
  description: string;
  testId: string;
}

/**
 * An item route with no project to read: an unbound window on the All projects
 * desktop (items open bound from their lists). Unsettled scope stays blank so
 * the first load never flashes an empty state.
 */
export function LoopUnboundState({ scope, description, testId }: LoopUnboundStateProps) {
  return (
    <div
      aria-busy={scope === null || undefined}
      className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
      data-testid={testId}
    >
      {scope === null ? null : (
        <Empty
          className="max-w-md"
          description={description}
          icon={Repeat2}
          title="Opened without a project"
        />
      )}
    </div>
  );
}
