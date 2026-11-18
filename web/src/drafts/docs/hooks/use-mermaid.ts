import { useEffect, useState } from "react";

/** Renders a Mermaid chart to an image with the shipped renderer; `null` until it is ready or if it fails. */
export function useMermaid(chart: string) {
  const [image, setImage] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    void import("@compozy/ui/lib/mermaid-renderer")
      .then(({ renderMermaidImage }) => renderMermaidImage(chart, "dark"))
      .then(src => {
        if (live) setImage(src);
      })
      .catch(() => {
        if (live) setImage(null);
      });
    return () => {
      live = false;
    };
  }, [chart]);
  return image;
}
