import { AlertCircle, FolderLock, Search } from "lucide-react";
import type { ComponentProps } from "react";

import { Button, Empty, PAGE_CONTENT_GUTTER, cn } from "@compozy/ui";

/** Why the daemon will not hand this automation over; each reason has its own next step. */
export type AutomationDetailUnavailableReason = "missing" | "elsewhere" | "error";

const UNAVAILABLE_COPY = {
  missing: {
    icon: Search,
    title: "Automation unavailable",
    testId: "automation-detail-empty",
  },
  elsewhere: {
    icon: FolderLock,
    title: "Unable to load details",
    testId: "automation-detail-elsewhere",
  },
  error: {
    icon: AlertCircle,
    title: "Unable to load details",
    testId: "automation-detail-error",
  },
} as const satisfies Record<
  AutomationDetailUnavailableReason,
  { icon: typeof Search; title: string; testId: string }
>;

interface AutomationDetailUnavailableProps extends Omit<ComponentProps<"div">, "children"> {
  description: string;
  reason: AutomationDetailUnavailableReason;
  onBack: () => void;
}

/** A missing, other-project or unreadable automation — always with a way back. */
export function AutomationDetailUnavailable({
  className,
  description,
  reason,
  onBack,
  ...props
}: AutomationDetailUnavailableProps) {
  const copy = UNAVAILABLE_COPY[reason];
  return (
    <div
      className={cn(
        PAGE_CONTENT_GUTTER,
        "flex min-h-0 flex-1 items-center justify-center py-10",
        className
      )}
      data-testid={copy.testId}
      {...props}
    >
      <Empty
        action={
          <Button onClick={onBack} size="sm" type="button" variant="neutral">
            Back to Automations
          </Button>
        }
        className="max-w-md"
        description={description}
        framed
        icon={copy.icon}
        title={copy.title}
      />
    </div>
  );
}
