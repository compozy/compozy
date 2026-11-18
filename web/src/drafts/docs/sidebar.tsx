/** The section tree: only the current section's folders, with a lit marker on the page being read. */
import { cn } from "@compozy/ui";
import { ChevronRight } from "@compozy/ui/icons";
import { useState } from "react";

import { DocsGlyph } from "./glyph";
import { GROUPS, containsSlug, keyOf, shortLabel, type DocPage, type NavNode } from "./nav";
import { BEAM, FOCUS } from "./text";

interface TreeProps {
  nodes: readonly NavNode[];
  depth: number;
  active: string;
  toggled: Record<string, boolean>;
  onToggle: (key: string, open: boolean) => void;
  onOpen: (slug: string) => void;
}

const ROW = cn(
  "flex min-h-8 w-full cursor-pointer items-center gap-2.5 rounded-[8px] bg-transparent px-2 py-1.5 text-left text-[14px]/[1.35] transition-colors",
  FOCUS
);

function LeafRow({
  node,
  depth,
  active,
  onOpen,
}: {
  node: NavNode;
  depth: number;
  active: boolean;
  onOpen: () => void;
}) {
  return (
    <button
      type="button"
      aria-current={active ? "page" : undefined}
      onClick={onOpen}
      className={cn(ROW, "relative", active ? "text-fg" : "text-muted-foreground hover:text-fg")}
    >
      {active ? (
        <span
          aria-hidden
          className={cn("absolute top-1.5 bottom-1.5 -left-[13px] w-0.5 rounded-full", BEAM)}
        />
      ) : null}
      {node.i && depth === 0 ? (
        <DocsGlyph name={node.i} className="size-4 shrink-0 text-subtle" />
      ) : null}
      <span className="min-w-0 break-words">{shortLabel(node)}</span>
    </button>
  );
}

function FolderRow(props: {
  node: NavNode;
  depth: number;
  open: boolean;
  active: boolean;
  onToggle: () => void;
  onOpen: (slug: string) => void;
}) {
  const { node } = props;
  const pick = () => {
    if (node.s === undefined || props.active) props.onToggle();
    if (node.s !== undefined) props.onOpen(node.s);
  };
  return (
    <button
      type="button"
      aria-expanded={props.open}
      aria-current={props.active ? "page" : undefined}
      onClick={pick}
      className={cn(
        ROW,
        props.active || (props.open && props.depth === 0) ? "text-fg" : "text-fg/75 hover:text-fg",
        props.active && "bg-[color:var(--light-060)]"
      )}
    >
      {props.depth === 0 ? (
        <DocsGlyph name={node.i} className="size-4 shrink-0 text-subtle" />
      ) : null}
      <span className="min-w-0 flex-1 break-words">{shortLabel(node)}</span>
      <ChevronRight
        aria-hidden
        className={cn(
          "size-3.5 shrink-0 text-subtle transition-transform",
          props.open && "rotate-90"
        )}
      />
    </button>
  );
}

function Tree({ nodes, depth, active, toggled, onToggle, onOpen }: TreeProps) {
  return (
    <ul className="m-0 flex list-none flex-col gap-px p-0">
      {nodes.map(node => {
        const key = keyOf(node);
        if (node.sep !== undefined) {
          return (
            <li key={key} className="mt-4 mb-1 px-2 font-mono text-[11.5px] text-subtle first:mt-1">
              {node.sep}
            </li>
          );
        }
        if (!node.c || node.c.length === 0) {
          return (
            <li key={key}>
              <LeafRow
                node={node}
                depth={depth}
                active={node.s === active}
                onOpen={() => onOpen(node.s ?? "")}
              />
            </li>
          );
        }
        const open = toggled[key] ?? containsSlug(node, active);
        return (
          <li key={key}>
            <FolderRow
              node={node}
              depth={depth}
              open={open}
              active={node.s === active}
              onToggle={() => onToggle(key, !open)}
              onOpen={onOpen}
            />
            {open ? (
              <div className="mt-px mb-1 ml-[15px] border-l border-[color:var(--light-090)] pl-3">
                <Tree
                  nodes={node.c}
                  depth={depth + 1}
                  active={active}
                  toggled={toggled}
                  onToggle={onToggle}
                  onOpen={onOpen}
                />
              </div>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}

export function DocsSidebar({ page, onOpen }: { page: DocPage; onOpen: (slug: string) => void }) {
  const [toggled, setToggled] = useState<Record<string, boolean>>({});
  const groups = page.group ? [page.group] : GROUPS;
  return (
    <nav aria-label="Docs navigation" className="flex flex-col gap-7">
      {groups.map(group => (
        <div key={group.id}>
          <p className="m-0 mb-2 flex items-baseline justify-between px-2 text-[13px] text-subtle">
            {group.label}
            <span className="font-mono text-[11.5px] tabular-nums">{group.count} pages</span>
          </p>
          <Tree
            nodes={group.items}
            depth={0}
            active={page.slug}
            toggled={toggled}
            onToggle={(key, open) => setToggled(current => ({ ...current, [key]: open }))}
            onOpen={onOpen}
          />
        </div>
      ))}
    </nav>
  );
}
