import { AlertTriangle } from "lucide-react";

import { Empty } from "@agh/ui";

export interface ConversationErrorProps {
  title: string;
  description: string;
  className?: string;
  testId?: string;
}

export function ConversationError({
  title,
  description,
  className,
  testId = "network-conversation-error",
}: ConversationErrorProps) {
  return (
    <Empty
      className={className}
      data-testid={testId}
      description={description}
      fill={false}
      icon={AlertTriangle}
      title={title}
    />
  );
}
