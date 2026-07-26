import { Play } from "lucide-react";
import type { ComponentProps } from "react";

import { Button, cn } from "@agh/ui";

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
      variant="outline"
      {...props}
    >
      <Play aria-hidden="true" className="size-3" />
      Run
    </Button>
  );
}
