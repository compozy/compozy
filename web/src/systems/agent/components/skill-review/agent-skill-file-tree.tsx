/** The proposed skill's folders and files. No shared tree exists yet, so it stays with the review. */
import { ChevronRight, FileCode, FileText, Folder, FolderOpen } from "@compozy/ui/icons";
import { useMemo, useState, type ReactNode } from "react";

import { cn } from "@compozy/ui";

import type { SkillReviewFile as SkillFile } from "../../lib/agent-skill-review";

interface TreeFolder {
  name: string;
  path: string;
  folders: TreeFolder[];
  files: SkillFile[];
}

function buildTree(files: readonly SkillFile[]): TreeFolder {
  const root: TreeFolder = { name: "", path: "", folders: [], files: [] };
  for (const file of files) {
    const parts = file.path.split("/");
    let folder = root;
    for (const part of parts.slice(0, -1)) {
      const path = folder.path ? `${folder.path}/${part}` : part;
      let next = folder.folders.find(item => item.name === part);
      if (!next) {
        next = { name: part, path, folders: [], files: [] };
        folder.folders.push(next);
      }
      folder = next;
    }
    folder.files.push(file);
  }
  return root;
}

const ROW =
  "flex h-8 w-full min-w-0 items-center gap-1.5 rounded-sm pr-2 text-left text-small-body outline-none hover:bg-hover focus-visible:shadow-focus-ring";

function indent(depth: number) {
  return { paddingLeft: `${depth * 14 + 8}px` };
}

function TreeRowButton({
  depth,
  className,
  onClick,
  children,
}: {
  depth: number;
  className: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button className={cn(ROW, className)} onClick={onClick} style={indent(depth)} type="button">
      {children}
    </button>
  );
}

function FolderRow({
  folder,
  depth,
  children,
}: {
  folder: TreeFolder;
  depth: number;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(true);
  const Glyph = open ? FolderOpen : Folder;
  return (
    <li aria-expanded={open} role="treeitem">
      <TreeRowButton
        className="text-muted-foreground"
        depth={depth}
        onClick={() => setOpen(value => !value)}
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3.5 shrink-0 text-faint transition-transform duration-base motion-reduce:transition-none",
            open && "rotate-90"
          )}
        />
        <Glyph aria-hidden="true" className="size-4 shrink-0 text-subtle" />
        <span className="truncate">{folder.name}</span>
      </TreeRowButton>
      {open ? <ul role="group">{children}</ul> : null}
    </li>
  );
}

function FileRow({
  file,
  depth,
  selected,
  onSelect,
}: {
  file: SkillFile;
  depth: number;
  selected: boolean;
  onSelect: () => void;
}) {
  const Glyph = file.language === "markdown" ? FileText : FileCode;
  const name = file.path.split("/").at(-1);
  return (
    <li aria-selected={selected} role="treeitem">
      <TreeRowButton
        className={selected ? "bg-selected text-fg" : "text-fg-2"}
        depth={depth + 1}
        onClick={onSelect}
      >
        <Glyph aria-hidden="true" className="size-4 shrink-0 text-subtle" />
        <span className="truncate">{name}</span>
        {file.automatic ? (
          <span
            aria-label="Runs automatically"
            className="ml-auto size-1.5 shrink-0 rounded-full bg-warning"
            role="img"
          />
        ) : null}
      </TreeRowButton>
    </li>
  );
}

function FolderItems({
  folder,
  depth,
  selected,
  onSelect,
}: {
  folder: TreeFolder;
  depth: number;
  selected: string;
  onSelect: (path: string) => void;
}) {
  return (
    <>
      {folder.folders.map(child => (
        <FolderRow depth={depth} folder={child} key={child.path}>
          <FolderItems depth={depth + 1} folder={child} onSelect={onSelect} selected={selected} />
        </FolderRow>
      ))}
      {folder.files.map(file => (
        <FileRow
          depth={depth}
          file={file}
          key={file.path}
          onSelect={() => onSelect(file.path)}
          selected={file.path === selected}
        />
      ))}
    </>
  );
}

export function AgentSkillFileTree({
  files,
  selected,
  onSelect,
}: {
  files: readonly SkillFile[];
  selected: string;
  onSelect: (path: string) => void;
}) {
  const tree = useMemo(() => buildTree(files), [files]);
  return (
    <ul aria-label="Skill files" className="flex flex-col gap-0.5" role="tree">
      <FolderItems depth={0} folder={tree} onSelect={onSelect} selected={selected} />
    </ul>
  );
}
