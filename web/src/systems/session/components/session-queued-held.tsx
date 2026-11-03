import { Pause } from "@compozy/ui/icons";

/** A queued row's state while it is held (S12): text plus glyph, never color alone. */
export function SessionQueuedHeld() {
  return (
    <span
      data-testid="composer-queued-held"
      className="inline-flex shrink-0 items-center gap-1.5 text-micro text-subtle tabular-nums"
    >
      <Pause aria-hidden="true" className="size-3" />
      Held until resume
    </span>
  );
}
