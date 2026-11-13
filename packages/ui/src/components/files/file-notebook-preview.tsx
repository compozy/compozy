"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { CodeBlock } from "../custom/code-block";
import { StreamMarkdown } from "../custom/stream-markdown";
import { FileHtmlPreview } from "./file-html-preview";
import { imageDataUrl } from "./image-data-url";
import { parseNotebook, type NotebookCell, type NotebookOutput } from "./notebook-model";

export interface FileNotebookPreviewProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** The `.ipynb` JSON text. */
  text: string;
  /** Rendered when the text is not a readable nbformat 4 notebook. */
  fallback?: React.ReactNode;
}

const OUTPUT_TEXT = "overflow-x-auto px-3 py-2 font-mono text-xs leading-5 whitespace-pre-wrap";

function CellOutput({ output }: { output: NotebookOutput }) {
  switch (output.kind) {
    case "text":
      return (
        <pre
          data-slot="file-notebook-output"
          data-stream={output.stream}
          className={cn(OUTPUT_TEXT, output.stream === "stderr" && "bg-destructive-tint text-fg")}
        >
          {output.text}
        </pre>
      );
    case "image": {
      const src = imageDataUrl(output.mimeType, output.data);
      return src ? (
        <img
          data-slot="file-notebook-output"
          src={src}
          alt="Cell output"
          className="max-w-full px-3 py-2"
        />
      ) : null;
    }
    case "markdown":
      return (
        <StreamMarkdown data-slot="file-notebook-output" className="px-3 py-2">
          {output.text}
        </StreamMarkdown>
      );
    case "html":
      return (
        <FileHtmlPreview
          data-slot="file-notebook-output"
          html={output.html}
          title="Cell output"
          className="h-80 flex-none"
        />
      );
    case "error":
      return (
        <pre
          data-slot="file-notebook-output"
          data-error="true"
          className={cn(OUTPUT_TEXT, "bg-destructive-tint text-fg")}
        >
          <strong className="font-semibold text-destructive-ink">{output.name}</strong>
          {output.text ? `\n${output.text}` : null}
        </pre>
      );
  }
}

function Cell({ cell, language }: { cell: NotebookCell; language: string }) {
  if (cell.kind === "markdown") {
    return (
      <div data-slot="file-notebook-cell" data-cell="markdown" className="px-[4.5rem]">
        <StreamMarkdown>{cell.source}</StreamMarkdown>
      </div>
    );
  }
  if (cell.kind === "raw") {
    return (
      <pre
        data-slot="file-notebook-cell"
        data-cell="raw"
        className={cn(OUTPUT_TEXT, "ml-[4.5rem] text-muted-foreground")}
      >
        {cell.source}
      </pre>
    );
  }
  const prompt = `[${cell.executionCount ?? " "}]:`;
  return (
    <div data-slot="file-notebook-cell" data-cell="code" className="flex gap-2">
      <span
        aria-hidden
        className="w-16 shrink-0 pt-2.5 text-right font-mono text-xs text-muted-foreground tabular-nums"
      >
        {prompt}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <CodeBlock code={cell.source} language={language} density="compact" />
        {cell.outputs.length > 0 ? (
          <div className="flex flex-col overflow-hidden rounded-sm border border-line-soft">
            {cell.outputs.map(output => (
              <CellOutput key={output.id} output={output} />
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

/**
 * A Jupyter notebook read-only: markdown cells as prose, code cells highlighted with their
 * execution count, and their saved outputs (text, images, markdown, errors; HTML in a sandbox).
 */
function FileNotebookPreview({
  text,
  fallback = null,
  className,
  ...props
}: FileNotebookPreviewProps) {
  const notebook = React.useMemo(() => parseNotebook(text), [text]);
  if (!notebook) return fallback;
  return (
    <div
      data-slot="file-notebook-preview"
      data-cell-count={notebook.cells.length}
      className={cn("min-h-0 flex-1 overflow-auto bg-canvas", className)}
      {...props}
    >
      <div className="mx-auto flex max-w-5xl flex-col gap-4 py-6 pr-6 pl-2">
        {notebook.cells.map(cell => (
          <Cell key={cell.id} cell={cell} language={notebook.language} />
        ))}
      </div>
    </div>
  );
}

export { FileNotebookPreview };
