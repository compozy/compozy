import { Button } from "@compozy/ui";
import { Check, Copy } from "@compozy/ui/icons";

import { useCopyFeedback, type CopyState } from "./use-copy-feedback";

export interface GatewayManualCodeProps {
  /** Exactly six digits, as issued by the daemon. */
  code: string;
}

const COPY_STATUS: Record<CopyState, string> = {
  idle: "",
  copied: "Pairing code copied",
  failed: "Couldn't copy the pairing code. Select it and copy it by hand.",
};

/**
 * The six-digit manual code: the primary artifact of the issuance view, because
 * it can be read aloud, typed on another device, and copied without a camera.
 *
 * The visible grouping is hidden from assistive technology, which hears the
 * digits one by one instead. Copy writes exactly the six digits, so pasting it
 * anywhere yields the same code the redeem gate accepts.
 */
export function GatewayManualCode({ code }: GatewayManualCodeProps) {
  const { state, copy } = useCopyFeedback();
  const copied = state === "copied";

  return (
    <div className="flex min-w-0 flex-col gap-2" data-testid="gateway-pairing-manual-code-block">
      <span className="eyebrow text-muted-foreground">Pairing code</span>
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-line-soft bg-surface-2 px-4 py-3">
        <p
          className="font-mono text-display-2xl font-semibold tracking-mono text-fg tabular-nums"
          data-testid="gateway-pairing-manual-code"
        >
          <span aria-hidden="true">{groupManualCode(code)}</span>
          <span className="sr-only">{code.split("").join(" ")}</span>
        </p>
        <Button
          data-testid="gateway-pairing-manual-code-copy"
          onClick={() => void copy(code)}
          size="cta-lg"
          type="button"
          variant="secondary"
        >
          {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
          {copied ? "Copied" : "Copy code"}
        </Button>
      </div>
      <span className="sr-only" role="status">
        {COPY_STATUS[state]}
      </span>
    </div>
  );
}

/** `123456` reads as `123 456`: the grouping is presentation, never the copied value. */
function groupManualCode(code: string): string {
  return `${code.slice(0, 3)} ${code.slice(3)}`;
}
