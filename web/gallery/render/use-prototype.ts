import { useEffect, useState } from "react";

import { frameStatusType, postFrameStatus } from "#gallery/shell/frame-status";
import { loaderFor } from "#gallery/shell/registry";
import type { PrototypeModule } from "#gallery/shell/registry-model";

type Loaded = { slug: string; module?: PrototypeModule; error?: string };

/** Loads a prototype module lazily; a failed import marks the document and tells the gallery. */
export function usePrototype(slug: string) {
  const [result, setResult] = useState<Loaded | null>(null);
  useEffect(() => {
    const loader = loaderFor(slug);
    if (!loader) return;
    let live = true;
    loader().then(
      module => {
        if (live) setResult({ slug, module });
      },
      (error: unknown) => {
        const message = String(error);
        if (live) setResult({ slug, error: message });
        // Standalone pages (screenshots) have no parent frame: the document carries the failure.
        document.documentElement.dataset.prototypeStatus = "error";
        postFrameStatus({ type: frameStatusType, status: "error", slug, message });
      }
    );
    return () => {
      live = false;
    };
  }, [slug]);
  return result?.slug === slug ? result : null;
}
