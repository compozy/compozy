"use client";

/** The section tree: only the current section's folders, with a lit marker on the page being read. */
import { cn } from "@compozy/ui";
import { ChevronRight } from "@compozy/ui/icons";
import Link from "next/link";
import type { MouseEvent, ReactNode } from "react";

import { BEAM, FOCUS } from "@/components/docs/docs-classes";
import { useRevealCurrent } from "@/components/docs/hooks/use-reveal-current";
import { containsUrl } from "@/lib/docs-navigation";
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

/** A plain left click; modified clicks keep the browser's own handling (new tab, new window). */
function isPlainClick(event: MouseEvent<HTMLAnchorElement>) {
  return event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;
}

function Chevron({ open }: { open: boolean }) {
  return (
    <ChevronRight
      aria-hidden
      className={cn("size-3.5 shrink-0 text-subtle transition-transform", open && "rotate-90")}
    />
  );
}

/**
 * A folder row. With a landing page the name is a real link (middle click, new tab, crawlers) and
 * the chevron is its own toggle; clicking the name of the folder being read toggles it, as in the
 * draft. A folder without a landing page is one toggle button.
 */
function FolderRow(props: {
  node: DocsNavNode;
  depth: number;
  open: boolean;
  active: boolean;
  onToggle: () => void;
}) {
  const { node, open, active, depth } = props;
  const tone = active || (open && depth === 0) ? "text-fg" : "text-fg/75 hover:text-fg";
  const label = (
    <>
      {depth === 0 ? <Glyph icon={node.icon} /> : null}
      <span className="min-w-0 flex-1 break-words">{shortLabel(node.name)}</span>
    </>
  );
  if (node.url === undefined) {
    return (
      <button type="button" aria-expanded={open} onClick={props.onToggle} className={cn(ROW, tone)}>
        {label}
        <Chevron open={open} />
      </button>
    );
  }
  return (
    <div className={cn("flex items-center rounded-[8px]", active && "bg-site-light-060")}>
      <Link
        href={node.url}
        aria-current={active ? "page" : undefined}
        onClick={event => {
          if (!active || !isPlainClick(event)) return;
          event.preventDefault();
          props.onToggle();
        }}
        className={cn(ROW, "min-w-0 flex-1 pr-1", tone)}
      >
        {label}
      </Link>
      <button
        type="button"
        aria-expanded={open}
        aria-label={`${open ? "Collapse" : "Expand"} ${shortLabel(node.name)}`}
        onClick={props.onToggle}
        className={cn(
          "flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-[8px] bg-transparent",
          FOCUS
        )}
      >
        <Chevron open={open} />
      </button>
    </div>
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

/**
 * The current section's tree. The folder state is owned by the docs frame, so the rail and the
 * drawer share it and it survives client navigations.
 */
export function DocsSidebar({
  groups,
  activeUrl,
  toggled,
  onToggle,
}: {
  groups: readonly DocsSidebarGroup[];
  activeUrl: string;
  toggled: Record<string, boolean>;
  onToggle: (key: string, open: boolean) => void;
}) {
  const nav = useRevealCurrent<HTMLElement>(activeUrl, groups.map(group => group.id).join(" "));
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
            onToggle={onToggle}
          />
        </div>
      ))}
    </nav>
  );
}
