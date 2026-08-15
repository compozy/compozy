import { Route } from "lucide-react";

import { LoopRunQuietNote } from "./loop-run-quiet-note";

interface LoopRunNextNoteProps {
  /** The graph-derived sentence from `buildNextNote`; null renders nothing. */
  note: string | null;
}

/**
 * The "What happens next" quiet note (§2/§5.3): one graph-derived sentence,
 * rendered only while the run is live and the pinned graph names what's ahead.
 */
export function LoopRunNextNote({ note }: LoopRunNextNoteProps) {
  if (!note) return null;
  return (
    <LoopRunQuietNote data-testid="loop-run-next" icon={Route}>
      {note}
    </LoopRunQuietNote>
  );
}
