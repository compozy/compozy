import { MessageCircle } from "lucide-react";

import { Button, Empty } from "@agh/ui";

export interface DirectsEmptyProps {
  onNewDirect?: () => void;
  className?: string;
  filtered?: boolean;
}

/**
 * Empty state for the Directs tab (`_design.md` §7.2).
 */
export function DirectsEmpty({ onNewDirect, className, filtered = false }: DirectsEmptyProps) {
  return (
    <Empty
      action={
        onNewDirect ? (
          <Button
            data-testid="network-directs-empty-new"
            onClick={onNewDirect}
            size="sm"
            type="button"
            variant="outline"
          >
            New direct
          </Button>
        ) : null
      }
      className={className}
      data-testid="network-directs-empty"
      description={
        filtered
          ? "Try another search or remove a filter."
          : "Open one to talk privately with a peer in this channel."
      }
      icon={MessageCircle}
      title={filtered ? "No matching direct rooms." : "No direct rooms yet."}
    />
  );
}
