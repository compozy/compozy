"use client";

/** The section tree: only the current section's folders, with a lit marker on the page being read. */
import { cn } from "@compozy/ui";
import { ChevronRight } from "@compozy/ui/icons";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";

import { BEAM, FOCUS } from "@/components/docs/docs-classes";
import { useRevealCurrent } from "@/components/docs/hooks/use-reveal-current";
import type { DocsNavNode } from "@/lib/docs-shell";

export interface DocsSidebarGroup {
  id: string;
  label: string;
  count: number;
  nodes: DocsNavNode[];
}

interface TreeProps {
  nodes: readonly DocsNavNode[];
  depth: number;
  active: string;
  toggled: Record<string, boolean>;
  onToggle: (key: string, open: boolean) => void;
}

const ROW = cn(
  "flex min-h-8 w-full cursor-pointer items-center gap-2.5 rounded-[8px] bg-transparent px-2 py-1.5 text-left text-[14px]/[1.35] transition-colors",
  FOCUS
);

/** A CLI page's last word is the command; the rest is its path. */
function shortLabel(name: string) {
  if (!name.startsWith("compozy ")) return name;
  return name.split(" ").at(-1) ?? name;
}

function containsUrl(node: DocsNavNode, url: string): boolean {
  if (node.url === url) return true;
  return (node.children ?? []).some(child => !child.separator && containsUrl(child, url));
}

function Glyph({ icon }: { icon: ReactNode }) {
  if (!icon) return null;
  return (
    <span aria-hidden className="flex size-4 shrink-0 text-subtle [&_svg]:size-4">
      {icon}
    </span>
  );
}

function LeafRow({ node, depth, active }: { node: DocsNavNode; depth: number; active: boolean }) {
  return (
    <Link
      href={node.url ?? "/docs"}
      aria-current={active ? "page" : undefined}
      className={cn(ROW, "relative", active ? "text-fg" : "text-muted-foreground hover:text-fg")}
    >
      {active ? (
        <span
          aria-hidden
          className={cn("absolute top-1.5 bottom-1.5 -left-[13px] w-0.5 rounded-full", BEAM)}
        />
      ) : null}
      {depth === 0 ? <Glyph icon={node.icon} /> : null}
      <span className="min-w-0 break-words">{shortLabel(node.name)}</span>
    </Link>
  );
}

function FolderRow(props: {
  node: DocsNavNode;
  depth: number;
  open: boolean;
  active: boolean;
  onToggle: () => void;
}) {
  const router = useRouter();
  const { node } = props;
  const pick = () => {
    if (node.url === undefined || props.active) props.onToggle();
    if (node.url !== undefined && !props.active) router.push(node.url);
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
        props.active && "bg-site-light-060"
      )}
    >
      {props.depth === 0 ? <Glyph icon={node.icon} /> : null}
      <span className="min-w-0 flex-1 break-words">{shortLabel(node.name)}</span>
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

function Tree({ nodes, depth, active, toggled, onToggle }: TreeProps) {
  return (
    <ul className="m-0 flex list-none flex-col gap-px p-0">
      {nodes.map(node => {
        if (node.separator) {
          return (
            <li
              key={node.key}
              className="mt-4 mb-1 px-2 font-mono text-[11.5px] text-subtle first:mt-1"
            >
              {node.name}
            </li>
          );
        }
        if (!node.children || node.children.length === 0) {
          return (
            <li key={node.key}>
              <LeafRow node={node} depth={depth} active={node.url === active} />
            </li>
          );
        }
        const open = toggled[node.key] ?? containsUrl(node, active);
        return (
          <li key={node.key}>
            <FolderRow
              node={node}
              depth={depth}
              open={open}
              active={node.url === active}
              onToggle={() => onToggle(node.key, !open)}
            />
            {open ? (
              <div className="mt-px mb-1 ml-[15px] border-l border-site-light-090 pl-3">
                <Tree
                  nodes={node.children}
                  depth={depth + 1}
                  active={active}
                  toggled={toggled}
                  onToggle={onToggle}
                />
              </div>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}

/** The current section's tree; pages outside every section get all of them. */
export function DocsSidebar({
  groups,
  activeUrl,
}: {
  groups: readonly DocsSidebarGroup[];
  activeUrl: string;
}) {
  const [toggled, setToggled] = useState<Record<string, boolean>>({});
  const nav = useRevealCurrent<HTMLElement>(activeUrl);
  return (
    <nav ref={nav} aria-label="Docs navigation" className="flex flex-col gap-7">
      {groups.map(group => (
        <div key={group.id}>
          <p className="m-0 mb-2 flex items-baseline justify-between px-2 text-[13px] text-subtle">
            {group.label}
            <span className="font-mono text-[11.5px] tabular-nums">{group.count} pages</span>
          </p>
          <Tree
            nodes={group.nodes}
            depth={0}
            active={activeUrl}
            toggled={toggled}
            onToggle={(key, open) => setToggled(current => ({ ...current, [key]: open }))}
          />
        </div>
      ))}
    </nav>
  );
}
