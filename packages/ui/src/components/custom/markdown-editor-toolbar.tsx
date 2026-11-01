"use client";

import { useEditorState, type Editor } from "@tiptap/react";
import * as React from "react";

import { Bold, Code, Italic, Link2, List, ListOrdered, Strikethrough } from "../../icons";
import { Button } from "../button";
import { Input } from "../input";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";
import { Toggle } from "../toggle";

type MarkName = "bold" | "italic" | "strike" | "code" | "bulletList" | "orderedList" | "link";

interface ToolbarAction {
  name: Exclude<MarkName, "link">;
  label: string;
  icon: typeof Bold;
  run: (editor: Editor) => void;
}

const ACTIONS: readonly ToolbarAction[] = [
  {
    name: "bold",
    label: "Bold",
    icon: Bold,
    run: e => e.chain().focus().toggleBold().run(),
  },
  {
    name: "italic",
    label: "Italic",
    icon: Italic,
    run: e => e.chain().focus().toggleItalic().run(),
  },
  {
    name: "strike",
    label: "Strikethrough",
    icon: Strikethrough,
    run: e => e.chain().focus().toggleStrike().run(),
  },
  {
    name: "code",
    label: "Inline code",
    icon: Code,
    run: e => e.chain().focus().toggleCode().run(),
  },
  {
    name: "bulletList",
    label: "Bulleted list",
    icon: List,
    run: e => e.chain().focus().toggleBulletList().run(),
  },
  {
    name: "orderedList",
    label: "Numbered list",
    icon: ListOrdered,
    run: e => e.chain().focus().toggleOrderedList().run(),
  },
];

const NO_ACTIVE: Record<MarkName, boolean> = {
  bold: false,
  italic: false,
  strike: false,
  code: false,
  bulletList: false,
  orderedList: false,
  link: false,
};

/** A typed address as a link target: bare hosts get `https://`; other schemes and empty targets are refused. */
function normalizeHref(raw: string): string | null {
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  const href = /^[a-z][a-z\d+.-]*:/i.test(trimmed) ? trimmed : `https://${trimmed}`;
  let url: URL;
  try {
    url = new URL(href);
  } catch {
    return null;
  }
  // A target needs somewhere to go: a host for web links, a recipient for mail.
  if (url.protocol === "http:" || url.protocol === "https:") return url.hostname ? href : null;
  if (url.protocol === "mailto:") return url.pathname ? href : null;
  return null;
}

function LinkControl({ editor, active }: { editor: Editor | null; active: boolean }) {
  const [open, setOpen] = React.useState(false);
  const [href, setHref] = React.useState("");
  // A typed address the editor refuses keeps the current link; only a blank one removes it.
  const invalid = href.trim() !== "" && normalizeHref(href) === null;

  const openChange = (next: boolean) => {
    if (next) setHref(editor?.getAttributes("link").href ?? "");
    setOpen(next);
  };

  const apply = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    event.stopPropagation();
    if (!editor || invalid) return;
    const target = normalizeHref(href);
    const chain = editor.chain().focus().extendMarkRange("link");
    if (target) chain.setLink({ href: target }).run();
    else chain.unsetLink().run();
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={openChange}>
      <PopoverTrigger
        render={
          <Toggle aria-label="Link" disabled={!editor} pressed={active} size="sm" title="Link" />
        }
      >
        <Link2 aria-hidden="true" />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72">
        <form className="flex items-center gap-1.5" onSubmit={apply}>
          <Input
            aria-invalid={invalid}
            aria-label="Link address"
            autoFocus
            className="h-button-sm"
            onChange={event => setHref(event.target.value)}
            placeholder="https://"
            value={href}
          />
          <Button disabled={invalid} size="sm" type="submit">
            {href.trim() === "" && active ? "Remove" : "Apply"}
          </Button>
        </form>
      </PopoverContent>
    </Popover>
  );
}

/** Formatting row of `MarkdownEditor`; pressed states follow the selection. */
export function MarkdownEditorToolbar({
  editor,
  disabled,
  label,
}: {
  editor: Editor | null;
  disabled: boolean;
  label: string;
}) {
  const active =
    useEditorState({
      editor,
      selector: ({ editor: current }) => {
        if (!current) return NO_ACTIVE;
        return {
          bold: current.isActive("bold"),
          italic: current.isActive("italic"),
          strike: current.isActive("strike"),
          code: current.isActive("code"),
          bulletList: current.isActive("bulletList"),
          orderedList: current.isActive("orderedList"),
          link: current.isActive("link"),
        };
      },
    }) ?? NO_ACTIVE;

  return (
    <div
      aria-label={label}
      className="flex items-center gap-0.5 border-b border-line-soft px-1.5 py-1"
      data-slot="markdown-editor-toolbar"
      role="toolbar"
    >
      {ACTIONS.map(action => (
        <Toggle
          aria-label={action.label}
          disabled={disabled || !editor}
          key={action.name}
          onPressedChange={() => editor && action.run(editor)}
          pressed={active[action.name]}
          size="sm"
          title={action.label}
        >
          <action.icon aria-hidden="true" />
        </Toggle>
      ))}
      {disabled ? null : <LinkControl active={active.link} editor={editor} />}
    </div>
  );
}
