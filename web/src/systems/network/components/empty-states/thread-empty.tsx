import { MessageSquareReply } from "lucide-react";

import { Empty } from "@agh/ui";

export interface ThreadEmptyProps {
  className?: string;
}

/**
 * Empty state for an existing thread that has no replies (`_design.md` §7.2).
 */
export function ThreadEmpty({ className }: ThreadEmptyProps) {
  return (
    <Empty
      className={className}
      data-testid="network-thread-empty"
      description="Reply below to keep the context alive."
      fill={false}
      icon={MessageSquareReply}
      title="Thread has no replies."
    />
  );
}
