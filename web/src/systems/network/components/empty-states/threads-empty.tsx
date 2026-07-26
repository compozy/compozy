import { MessageSquare } from "lucide-react";

import { Button, Empty } from "@agh/ui";

export interface ThreadsEmptyProps {
  onStartThread?: () => void;
  className?: string;
  filtered?: boolean;
}

/**
 * Empty state for the Threads tab (`_design.md` §7.2).
 */
export function ThreadsEmpty({ onStartThread, className, filtered = false }: ThreadsEmptyProps) {
  return (
    <Empty
      action={
        onStartThread ? (
          <Button
            data-testid="network-threads-empty-start"
            onClick={onStartThread}
            size="sm"
            type="button"
            variant="outline"
          >
            Start a thread
          </Button>
        ) : null
      }
      className={className}
      data-testid="network-threads-empty"
      description={
        filtered
          ? "Try another search or remove a filter."
          : "Start the first one; agents and humans both join."
      }
      icon={MessageSquare}
      title={filtered ? "No matching threads." : "No threads yet."}
    />
  );
}
