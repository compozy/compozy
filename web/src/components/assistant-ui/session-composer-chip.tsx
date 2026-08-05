import type { DirectiveChipProps } from "@assistant-ui/react-lexical";
import { Blocks } from "lucide-react";

/**
 * Inline skill chip inside the composer: no fill, info-toned, sharing the
 * line box with typed text — the same visual language as the transcript's
 * skill pills and the reference composer.
 */
export function SessionCommandChip({ directiveId, directiveType, label }: DirectiveChipProps) {
  return (
    <span
      data-testid="composer-command-chip"
      data-command-token={directiveId}
      data-directive-type={directiveType}
      className="mx-0.5 inline max-w-full align-baseline font-medium text-info select-none"
    >
      <Blocks
        className="mr-1 inline-block size-[1em] -translate-y-px align-middle"
        aria-hidden="true"
      />
      {label}
    </span>
  );
}
