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
    return (
      <CodeBlock code={content} language={language} showPrompt={false} copyable density="compact" />
    );
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
    <div data-slot={`tool-call-row-${slot}`} className="flex min-w-0 flex-col gap-1.5">
      <Eyebrow className="text-subtle">{label}</Eyebrow>
      <div data-slot={`tool-call-row-${slot}-body`} className="min-w-0">
        <ToolCallSectionBody source={source} format={format} language={language}>
          {children}
        </ToolCallSectionBody>
      </div>
    </div>
  );
}
