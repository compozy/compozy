import { useEffect, useState } from "react";

import { frameStatusType, postFrameStatus } from "#gallery/shell/frame-status";
import { loaderFor } from "#gallery/shell/registry";
import type { PrototypeModule } from "#gallery/shell/registry-model";

type Loaded = { slug: string; module?: PrototypeModule; error?: string };

/** Loads a prototype module lazily; a failed import is reported to the embedding gallery. */
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
        postFrameStatus({ type: frameStatusType, status: "error", slug, message });
      }
    );
    return () => {
      live = false;
    };
  }, [slug]);
  return result?.slug === slug ? result : null;
}
