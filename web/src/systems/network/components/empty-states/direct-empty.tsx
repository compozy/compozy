import { MessageCircle } from "lucide-react";

import { Empty } from "@agh/ui";

export interface DirectEmptyProps {
  className?: string;
}

/**
 * Empty state for a direct room with no messages yet (`_design.md` §7.2).
 */
export function DirectEmpty({ className }: DirectEmptyProps) {
  return (
    <Empty
      className={className}
      data-testid="network-direct-empty"
      description="Send the first message; they'll be notified."
      fill={false}
      icon={MessageCircle}
      title="Quiet so far."
    />
  );
}
