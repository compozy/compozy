"use client";

import { EditorContent } from "@tiptap/react";

import { cn } from "../../lib/utils";
import { markdownEditorAttributes, useMarkdownEditor } from "./hooks/use-markdown-editor";
import { MarkdownEditorToolbar } from "./markdown-editor-toolbar";

export interface MarkdownEditorProps {
  /** Markdown source; the editor re-reads it only when it differs from what it holds. */
  value: string;
  onValueChange: (markdown: string) => void;
  /** Cmd/Ctrl+Enter inside the editor. */
  onSubmit?: () => void;
  placeholder?: string;
  disabled?: boolean;
  autoFocus?: boolean;
  id?: string;
  className?: string;
  /** Classes for the scrolling content area (height bounds). */
  contentClassName?: string;
  toolbarLabel?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
  "aria-labelledby"?: string;
  "aria-label"?: string;
  "data-testid"?: string;
}

const CONTENT_CLASS = cn(
  "min-h-24 px-3 py-2 text-small-body text-fg outline-none [overflow-wrap:anywhere]",
  "[&_p]:my-0 [&_p+p]:mt-2 [&_ul]:my-1 [&_ol]:my-1 [&_ul]:list-disc [&_ol]:list-decimal [&_ul]:ps-5 [&_ol]:ps-5 [&_li>p]:my-0",
  "[&_strong]:font-semibold [&_strong]:text-fg-strong [&_s]:text-muted",
  "[&_code]:rounded-xxs [&_code]:bg-surface-2 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[0.92em]",
  "[&_a]:text-fg [&_a]:underline [&_a]:decoration-line-strong [&_a]:underline-offset-2",
  "[&_.is-editor-empty:first-child]:before:pointer-events-none [&_.is-editor-empty:first-child]:before:float-left [&_.is-editor-empty:first-child]:before:h-0 [&_.is-editor-empty:first-child]:before:text-subtle [&_.is-editor-empty:first-child]:before:content-[attr(data-placeholder)]"
);

/**
 * Rich-text field that reads and writes Markdown: bold, italic, strike,
 * inline code, bullet and numbered lists, and links. Headings, quotes, code
 * blocks, and rules stay out so the note reads as prose. Built on Tiptap
 * with its official Markdown extension; `value` is the Markdown source.
 */
export function MarkdownEditor({
  value,
  onValueChange,
  onSubmit,
  placeholder,
  disabled = false,
  autoFocus = false,
  id,
  className,
  contentClassName,
  toolbarLabel = "Formatting",
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
  "aria-labelledby": ariaLabelledBy,
  "aria-label": ariaLabel,
  "data-testid": testId,
}: MarkdownEditorProps) {
  const editor = useMarkdownEditor({
    value,
    onValueChange,
    onSubmit,
    placeholder,
    disabled,
    autoFocus,
    attributes: markdownEditorAttributes({
      id,
      className: cn(CONTENT_CLASS, contentClassName),
      ariaInvalid,
      ariaDescribedBy,
      ariaLabelledBy,
      ariaLabel,
      testId,
    }),
  });

  return (
    <div
      className={cn(
        "flex w-full min-w-0 flex-col rounded-md border border-line bg-canvas transition-colors hover:border-line-strong focus-within:border-line-strong focus-within:shadow-focus-ring",
        ariaInvalid && "border-danger hover:border-danger focus-within:border-danger",
        disabled && "pointer-events-none border-line-soft bg-sunken text-disabled",
        className
      )}
      data-disabled={disabled || undefined}
      data-invalid={ariaInvalid || undefined}
      data-slot="markdown-editor"
      data-testid={testId}
    >
      <MarkdownEditorToolbar disabled={disabled} editor={editor} label={toolbarLabel} />
      <EditorContent className="max-h-60 min-h-0 overflow-y-auto" editor={editor} />
    </div>
  );
}
