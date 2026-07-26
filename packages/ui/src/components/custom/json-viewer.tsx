"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface JsonViewerProps extends Omit<React.ComponentProps<"pre">, "children"> {
  /** Object/array/scalar to render. Pretty-printed with 2-space indentation. */
  value: unknown;
  /** Override pretty-printed indent. */
  indent?: number;
}

interface RenderEntry {
  id: string;
  kind: "key" | "string" | "number" | "boolean" | "null" | "punct";
  text: string;
}

function tokenize(text: string): RenderEntry[] {
  const out: RenderEntry[] = [];
  // Lightweight tokenizer for the structured pretty-print produced by JSON.stringify.
  const regex =
    /("[^"\\]*(?:\\.[^"\\]*)*"\s*:|"[^"\\]*(?:\\.[^"\\]*)*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null|[{}[\],])/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let counter = 0;
  while ((match = regex.exec(text)) !== null) {
    const before = text.slice(lastIndex, match.index);
    if (before.length > 0) {
      out.push({ id: `gap-${counter++}`, kind: "punct", text: before });
    }
    const token = match[0];
    if (/^"[^"\\]*(?:\\.[^"\\]*)*"\s*:$/.test(token)) {
      out.push({ id: `tok-${counter++}`, kind: "key", text: token });
    } else if (token.startsWith('"')) {
      out.push({ id: `tok-${counter++}`, kind: "string", text: token });
    } else if (/^(true|false)$/.test(token)) {
      out.push({ id: `tok-${counter++}`, kind: "boolean", text: token });
    } else if (token === "null") {
      out.push({ id: `tok-${counter++}`, kind: "null", text: token });
    } else if (/^-?\d/.test(token)) {
      out.push({ id: `tok-${counter++}`, kind: "number", text: token });
    } else {
      out.push({ id: `tok-${counter++}`, kind: "punct", text: token });
    }
    lastIndex = match.index + token.length;
  }
  const tail = text.slice(lastIndex);
  if (tail.length > 0) out.push({ id: `gap-${counter++}`, kind: "punct", text: tail });
  return out;
}

const KIND_CLASS: Record<RenderEntry["kind"], string> = {
  key: "text-accent",
  string: "text-success",
  number: "text-info",
  boolean: "text-warning",
  null: "text-muted",
  punct: "text-subtle",
};

function JsonViewer({ value, indent = 2, className, ...props }: JsonViewerProps) {
  let text: string;
  try {
    text = JSON.stringify(value, null, indent);
  } catch {
    text = String(value);
  }
  const tokens = tokenize(text);
  return (
    <pre
      data-slot="json-viewer"
      className={cn(
        "overflow-x-auto rounded border border-line bg-canvas px-3 py-2 font-mono text-form-hint leading-normal text-fg",
        className
      )}
      {...props}
    >
      <code data-slot="json-viewer-code">
        {tokens.map(token => (
          <span key={token.id} data-slot={`json-${token.kind}`} className={KIND_CLASS[token.kind]}>
            {token.text}
          </span>
        ))}
      </code>
    </pre>
  );
}

export { JsonViewer };
