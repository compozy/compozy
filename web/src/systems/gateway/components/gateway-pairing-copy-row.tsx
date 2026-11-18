import { Button, MonoId } from "@compozy/ui";
import { Check, Copy } from "@compozy/ui/icons";

import { useCopyFeedback, type CopyState } from "./use-copy-feedback";

export interface GatewayPairingCopyRowProps {
  /** Structural label above the value, for example "Pairing link". */
  label: string;
  /** Full opaque value. It is displayed truncated but copied whole. */
  value: string;
  /** Visible button label before a copy, for example "Copy link". */
  copyLabel: string;
  testId: string;
}

const COPY_STATUS: Record<CopyState, string> = {
  idle: "",
  copied: "Copied to clipboard",
  failed: "Couldn't copy to clipboard. Select the text and copy it by hand.",
};

/**
 * A secondary copyable value: the long link or artifact. The value truncates
 * inside its own column so it can never widen the dialog, and the copy target
 * keeps the full string.
 */
export function GatewayPairingCopyRow({
  label,
  value,
  copyLabel,
  testId,
}: GatewayPairingCopyRowProps) {
  const { state, copy } = useCopyFeedback();
  const copied = state === "copied";

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="eyebrow text-muted-foreground">{label}</span>
      <div className="flex min-w-0 items-center gap-2">
        <MonoId className="min-w-0 flex-1" data-testid={testId} preserveCase value={value} />
        <Button
          data-testid={`${testId}-copy`}
          onClick={() => void copy(value)}
          size="cta-lg"
          type="button"
          variant="secondary"
        >
          {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
          {copied ? "Copied" : copyLabel}
        </Button>
      </div>
      <span className="sr-only" role="status">
        {COPY_STATUS[state]}
      </span>
      {state === "failed" ? (
        <p className="text-form-label text-destructive" data-testid={`${testId}-error`}>
          Couldn&apos;t copy. Select the value and copy it by hand.
        </p>
      ) : null}
    </div>
  );
}
