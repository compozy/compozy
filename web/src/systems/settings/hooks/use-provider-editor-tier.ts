import type { EntityMode } from "@compozy/ui";
import { useState } from "react";

/**
 * The provider detail host keeps its dialog mounted between openings, so the disclosure tier is
 * reset whenever it switches entity or mode — a create must open on the common
 * path even if the last edit ended in Advanced. Keyed by the entry, never by the
 * draft: a create draft's name changes on every keystroke and would reset the
 * tier mid-typing.
 */
export function useProviderEditorTier(surfaceKey: string) {
  const [tier, setTier] = useState<EntityMode>("simple");
  const [tierSurface, setTierSurface] = useState(surfaceKey);
  if (tierSurface !== surfaceKey) {
    setTierSurface(surfaceKey);
    setTier("simple");
  }
  return [tier, setTier] as const;
}
