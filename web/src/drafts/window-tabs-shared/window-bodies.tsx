/**
 * What the draft windows show. The draft is about the chrome around them, so bodies are short
 * stand-ins built from the same primitives the apps use; they never reach a daemon.
 */
import { File, Folder } from "@compozy/ui/icons";

import { CodeBlock, Markdown, StateGlyph, cn } from "@compozy/ui";

import type { DraftWindow } from "./model";

const SESSION_REPLY: Record<string, string> = {
  tabs: "The tab bar now belongs to the desktop. I moved the split tree into the tab and removed the floating layer.\n\n- `os-tab-bar` renders one plate per tab\n- `os-split-view` lays windows out from their shares\n- Seams are the only resize handle",
  auth: "The redirect loop came from the callback route reading a stale cookie. I cleared it on sign-out and added a regression test.",
  review:
    "I found two things worth your call before I approve:\n\n1. The retry path swallows the timeout error\n2. `attachWithRetry` is exported but unused",
  docs: "Draft of the release notes is ready. The migration block explains what becomes a tab.",
  flaky: "The test fails once in forty runs. I am bisecting the attach timing now.",
};

function SessionBody({ window }: { window: DraftWindow }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-hidden px-6 py-5">
        <div className="max-w-[80%] self-end rounded-lg bg-surface-2 px-4 py-3 text-body text-fg">
          {window.title}
        </div>
        <div className="max-w-2xl text-body text-fg-2">
          <Markdown>{SESSION_REPLY[window.id] ?? SESSION_REPLY.auth!}</Markdown>
        </div>
        {window.signal === "running" ? (
          <span className="flex items-center gap-2 text-meta text-muted-foreground">
            <StateGlyph size="sm" state="running" />
            Working
          </span>
        ) : null}
      </div>
      <div className="px-4 pb-4">
        <div className="rounded-lg bg-card px-4 py-3 text-body text-subtle shadow-card">
          Message the agent
        </div>
      </div>
    </div>
  );
}

const TERMINAL_LINES: Record<string, string> = {
  term: "$ make dev\n▸ daemon   listening on unix:///tmp/compozy.sock\n▸ web      http://127.0.0.1:5173\n▸ watch    rebuilding internal/windowmanager\n\n  ready in 1.4s",
  "term-tests":
    "$ bun test --watch\n ✓ tab-projection.test.ts (14)\n ✓ drop-edge.test.ts (22)\n ✓ os-tab-bar.test.tsx (9)\n\n 45 pass, 0 fail",
};

function TerminalBody({ window }: { window: DraftWindow }) {
  return (
    <pre className="min-h-0 flex-1 overflow-hidden bg-canvas px-4 py-3 font-mono text-meta leading-relaxed text-fg-2">
      {TERMINAL_LINES[window.id] ?? TERMINAL_LINES.term}
    </pre>
  );
}

const FILE_ROWS = [
  { name: "components", folder: true },
  { name: "hooks", folder: true },
  { name: "lib", folder: true },
  { name: "os-tab-bar.tsx", folder: false, open: true },
  { name: "os-tab.tsx", folder: false },
  { name: "os-split-view.tsx", folder: false },
];

function FilesBody() {
  return (
    <div className="flex min-h-0 flex-1">
      <ul className="w-44 shrink-0 border-r border-line bg-sunken py-2 text-meta text-fg-2">
        {FILE_ROWS.map(row => (
          <li
            className={cn("flex items-center gap-2 px-3 py-1", row.open && "bg-selected text-fg")}
            key={row.name}
          >
            {row.folder ? (
              <Folder aria-hidden="true" className="size-3.5 text-subtle" />
            ) : (
              <File aria-hidden="true" className="size-3.5 text-subtle" />
            )}
            <span className="truncate">{row.name}</span>
          </li>
        ))}
      </ul>
      <div className="min-w-0 flex-1 overflow-hidden p-3">
        <CodeBlock
          code={
            'export function OsTabBar({ tabs, activeTabId }: OsTabBarProps) {\n  return (\n    <div role="tablist">\n      {tabs.map(tab => (\n        <OsTab key={tab.id} tab={tab} active={tab.id === activeTabId} />\n      ))}\n    </div>\n  );\n}'
          }
          language="tsx"
        />
      </div>
    </div>
  );
}

const TILES = [
  { label: "Running", value: "3" },
  { label: "Needs you", value: "2" },
  { label: "Done today", value: "14" },
  { label: "Automations", value: "5" },
];

function TilesBody({ heading }: { heading: string }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-6">
      <h2 className="text-card-title font-medium text-fg">{heading}</h2>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3">
        {TILES.map(tile => (
          <div className="rounded-lg bg-card p-4 shadow-card" key={tile.label}>
            <div className="text-meta text-muted-foreground">{tile.label}</div>
            <div className="mt-1 text-2xl font-medium text-fg tabular-nums">{tile.value}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function WindowBody({ window }: { window: DraftWindow }) {
  switch (window.app) {
    case "session":
      return <SessionBody window={window} />;
    case "terminal":
      return <TerminalBody window={window} />;
    case "files":
      return <FilesBody />;
    default:
      return <TilesBody heading={window.title} />;
  }
}
