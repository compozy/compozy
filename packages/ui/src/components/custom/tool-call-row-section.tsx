import * as React from "react";

import { CodeBlock } from "./code-block";
import { Eyebrow } from "./eyebrow";
import { Markdown } from "./markdown";

export interface ToolCallRowSectionProps {
  children?: React.ReactNode;
  source?: string;
  format?: "markdown" | "code";
  language?: string;
}

function ToolCallSectionBody({
  children,
  source,
  format,
  language,
}: Pick<ToolCallRowSectionProps, "children" | "source" | "format" | "language">) {
  if (children !== undefined && children !== null && children !== false) {
    return <>{children}</>;
  }
  const content = source ?? "";
  if (format === "code") {
    return <CodeBlock code={content} language={language} copyable density="compact" />;
  }
  return (
    <Markdown compact className="max-w-none rounded-sm bg-canvas p-2 text-small-body text-muted">
      {content}
    </Markdown>
  );
}

export function ToolCallRowSection({
  slot,
  label,
  children,
  source,
  format,
  language,
}: {
  slot: "input" | "output";
  label: string;
} & ToolCallRowSectionProps) {
  return (
    // The row body bounds its sections (`max-h-64`): a section is a shrinkable
    // flex column so a child that scrolls internally (a bounded payload) can give
    // up height and keep what follows it — its counts and actions — in view.
    <div data-slot={`tool-call-row-${slot}`} className="flex min-h-0 min-w-0 flex-col gap-1.5">
      <Eyebrow className="shrink-0 text-subtle">{label}</Eyebrow>
      <div data-slot={`tool-call-row-${slot}-body`} className="flex min-h-0 min-w-0 flex-col">
        <ToolCallSectionBody source={source} format={format} language={language}>
          {children}
        </ToolCallSectionBody>
      </div>
    </div>
  );
}
