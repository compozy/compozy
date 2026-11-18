import { useEffect, useState } from "react";

import { loadSdk, type SdkBundle } from "../sdk";

/** The SDK reference data: `null` while it loads. */
export function useSdk() {
  const [bundle, setBundle] = useState<SdkBundle | null>(null);
  useEffect(() => {
    let live = true;
    void loadSdk().then(loaded => {
      if (live) setBundle(loaded);
    });
    return () => {
      live = false;
    };
  }, []);
  return bundle;
}
