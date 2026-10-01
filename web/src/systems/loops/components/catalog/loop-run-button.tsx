import { Play } from "lucide-react";
import type { ComponentProps } from "react";

import { Button, cn } from "@compozy/ui";

interface LoopRunButtonProps extends ComponentProps<typeof Button> {
  loopName: string;
  onRun: () => void;
}

export function LoopRunButton({ loopName, onRun, className, ...props }: LoopRunButtonProps) {
  return (
    <Button
      className={cn("shrink-0", className)}
      data-testid={`loop-catalog-run-${loopName}`}
      onClick={onRun}
      size="sm"
      type="button"
      variant="secondary"
      {...props}
    >
      <Play aria-hidden="true" />
      Run
    </Button>
  );
}
