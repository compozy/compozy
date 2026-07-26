import { ServerCrash } from "lucide-react";

import { Button, ConnectionIndicator, Empty } from "@agh/ui";

export interface DaemonDownProps {
  onRetry?: () => void;
  className?: string;
}

/**
 * Full-page error state when the daemon is unreachable (`_design.md` §7.3).
 */
export function DaemonDown({ onRetry, className }: DaemonDownProps) {
  return (
    <Empty
      action={
        onRetry ? (
          <Button
            data-testid="network-daemon-down-retry"
            onClick={onRetry}
            size="sm"
            type="button"
            variant="outline"
          >
            Retry connection
          </Button>
        ) : null
      }
      className={className}
      data-testid="network-daemon-down"
      description="Make sure the AGH daemon is running."
      icon={ServerCrash}
      title={
        <span className="inline-flex flex-col items-center gap-2">
          <ConnectionIndicator status="error" />
          <span>Network is unreachable.</span>
        </span>
      }
    />
  );
}
