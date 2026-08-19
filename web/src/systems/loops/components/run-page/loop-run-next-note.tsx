import { Route } from "lucide-react";

import { LoopRunQuietNote } from "./loop-run-quiet-note";

interface LoopRunNextNoteProps {
  /** The graph-derived sentence from `buildNextNote`; null renders nothing. */
  note: string | null;
}

export function LoopRunNextNote({ note }: LoopRunNextNoteProps) {
  if (!note) return null;
  return (
    <LoopRunQuietNote data-testid="loop-run-next" icon={Route}>
      {note}
    </LoopRunQuietNote>
  );
}
