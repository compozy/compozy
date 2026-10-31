import { Extension, useEditor, type Editor } from "@tiptap/react";
import { Placeholder } from "@tiptap/extensions";
import { Markdown } from "@tiptap/markdown";
import { StarterKit } from "@tiptap/starter-kit";
import * as React from "react";

/** The Markdown an editor holds; an empty document is the empty string. */
export function readMarkdown(editor: Pick<Editor, "getMarkdown" | "isEmpty">): string {
  return editor.isEmpty ? "" : editor.getMarkdown().trim();
}

/** DOM attributes of the editable element: its role, name, and state. */
export function markdownEditorAttributes(options: {
  id?: string;
  className: string;
  ariaInvalid?: boolean;
  ariaDescribedBy?: string;
  ariaLabelledBy?: string;
  ariaLabel?: string;
  testId?: string;
}): Record<string, string> {
  const attributes: Record<string, string> = {
    role: "textbox",
    "aria-multiline": "true",
    class: options.className,
  };
  if (options.id) attributes.id = options.id;
  if (options.ariaLabel) attributes["aria-label"] = options.ariaLabel;
  if (options.ariaLabelledBy) attributes["aria-labelledby"] = options.ariaLabelledBy;
  if (options.ariaDescribedBy) attributes["aria-describedby"] = options.ariaDescribedBy;
  if (options.ariaInvalid) attributes["aria-invalid"] = "true";
  if (options.testId) attributes["data-testid"] = `${options.testId}-input`;
  return attributes;
}

interface UseMarkdownEditorOptions {
  value: string;
  onValueChange: (markdown: string) => void;
  onSubmit?: () => void;
  placeholder?: string;
  disabled: boolean;
  autoFocus: boolean;
  attributes: Record<string, string>;
}

/**
 * The Tiptap editor behind `MarkdownEditor`: prose marks, lists, and links,
 * read and written as Markdown, with Cmd/Ctrl+Enter routed to `onSubmit`.
 */
export function useMarkdownEditor({
  value,
  onValueChange,
  onSubmit,
  placeholder,
  disabled,
  autoFocus,
  attributes,
}: UseMarkdownEditorOptions): Editor | null {
  const latest = React.useRef({ onValueChange, onSubmit });
  React.useEffect(() => {
    latest.current = { onValueChange, onSubmit };
  });

  const editor = useEditor(
    {
      extensions: [
        StarterKit.configure({
          blockquote: false,
          codeBlock: false,
          heading: false,
          horizontalRule: false,
          underline: false,
          trailingNode: false,
          link: { openOnClick: false, autolink: true, defaultProtocol: "https" },
        }),
        Markdown,
        Placeholder.configure({ placeholder: placeholder ?? "" }),
        Extension.create({
          name: "markdownEditorSubmit",
          addKeyboardShortcuts: () => ({
            "Mod-Enter": () => {
              const submit = latest.current.onSubmit;
              if (!submit) return false;
              submit();
              return true;
            },
          }),
        }),
      ],
      content: value,
      contentType: "markdown",
      editable: !disabled,
      autofocus: autoFocus ? "end" : false,
      editorProps: { attributes },
      onUpdate: ({ editor: current }) => latest.current.onValueChange(readMarkdown(current)),
    },
    [placeholder]
  );

  React.useEffect(() => {
    if (!editor) return;
    editor.setEditable(!disabled, false);
    editor.setOptions({ editorProps: { attributes } });
    // An outside change (a reset, a loaded draft) replaces the content; the
    // editor's own echo is already equal and leaves the selection alone.
    if (readMarkdown(editor) !== value.trim()) {
      editor.commands.setContent(value, { contentType: "markdown", emitUpdate: false });
    }
  });

  return editor;
}
