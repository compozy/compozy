"use client";

import * as React from "react";

import { Button } from "../button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../collapsible";
import { CodeBlock, CopyIconButton } from "./code-block";
import { useResolvedCodeTheme } from "./hooks/use-code-block";

export function MarkdownMermaid({ code, streaming }: { code: string; streaming: boolean }) {
  const theme = useResolvedCodeTheme("auto");
  const [rendered, setRendered] = React.useState<{
    code: string;
    theme: string;
    src: string;
  } | null>(null);
  const src =
    !streaming && rendered?.code === code && rendered.theme === theme ? rendered.src : null;

  React.useEffect(() => {
    if (streaming) return;
    let cancelled = false;
    void import("../../lib/mermaid-renderer")
      .then(({ renderMermaidImage }) => renderMermaidImage(code, theme))
      .then(src => {
        if (!cancelled) setRendered({ code, theme, src });
      })
      .catch(() => {
        // Incomplete/invalid diagrams and unavailable chunks retain the readable source.
        if (!cancelled) setRendered(null);
      });
    return () => {
      cancelled = true;
    };
  }, [code, theme, streaming]);

  const source = (
    <CodeBlock code={code} language="mermaid" caption="mermaid" copyable density="compact" />
  );
  if (!src) return <div className="my-5">{source}</div>;
  return (
    <div data-slot="markdown-mermaid" className="my-5">
      <img
        src={src}
        alt="Mermaid diagram"
        className="max-w-full"
        onError={() => setRendered(null)}
      />
      <Collapsible>
        <div className="flex items-center gap-2">
          <CollapsibleTrigger render={<Button variant="ghost" size="sm" />}>
            View source
          </CollapsibleTrigger>
          <CopyIconButton value={code} />
        </div>
        <CollapsibleContent>{source}</CollapsibleContent>
      </Collapsible>
    </div>
  );
}
