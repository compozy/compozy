import { useEffect, useState } from "react";

import { loadDoc } from "../content";
import type { Doc } from "../mdx";

interface Loaded {
  slug: string;
  doc: Doc | null;
}

/** The page body for a slug: `null` while the MDX is on its way. */
export function useDoc(slug: string) {
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  useEffect(() => {
    let live = true;
    void loadDoc(slug).then(doc => {
      if (live) setLoaded({ slug, doc });
    });
    return () => {
      live = false;
    };
  }, [slug]);
  return loaded?.slug === slug ? loaded : null;
}
