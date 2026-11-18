/**
 * Today's menus, with their shipped labels and order, beside the proposed grouped tree each family
 * would share on every surface. Sources are cited per family; the proposals reuse those labels and
 * name a new submenu only where the family has none.
 */
import {
  AppWindow,
  Archive,
  ArrowDown,
  ArrowUp,
  Bot,
  Clipboard,
  Code2,
  Columns2,
  Copy,
  ExternalLinkIcon,
  FileDiff,
  Folder,
  FolderGit2,
  FolderInput,
  FolderOpen,
  FolderOutput,
  FolderPlus,
  Hash,
  Layers,
  LayoutGrid,
  Link2,
  Maximize2,
  MessageSquareTextIcon,
  Monitor,
  Pencil,
  Pin,
  Play,
  Plus,
  RefreshCwIcon,
  Search,
  Settings2,
  Trash2,
  Unlink,
  WrenchIcon,
  X,
  type LucideIcon,
} from "@compozy/ui/icons";

import type { SessionMenuNode } from "../session-menus/session-menu-model";

type Extra = Partial<Extract<SessionMenuNode, { kind: "item" }>>;

const i = (id: string, label: string, icon?: LucideIcon, extra: Extra = {}): SessionMenuNode => ({
  kind: "item",
  id,
  label,
  icon,
  ...extra,
});
const sub = (
  id: string,
  label: string,
  icon: LucideIcon,
  children: SessionMenuNode[],
  hint?: string
): SessionMenuNode => ({ kind: "sub", id, label, icon, children, hint });
const sep = (id: string): SessionMenuNode => ({ kind: "separator", id });
const group = (id: string, label: string, children: SessionMenuNode[]): SessionMenuNode => ({
  kind: "group",
  id,
  label,
  children,
});

const FOLDERS = [i("f-clients", "Clients", Folder), i("f-labs", "Labs", Folder)];

/** One menu as a surface shows it. */
export interface MenuSpecimen {
  surface: string;
  /** Where it lives in code, for the implementation. */
  source: string;
  nodes: SessionMenuNode[];
  openSubs?: string[];
}

export interface MenuFamily {
  title: string;
  /** What changes, in one or two sentences. */
  summary: string;
  today: MenuSpecimen[];
  proposed: MenuSpecimen;
}

// Projects — session-list-group-project-menu.tsx, sessions-board-swimlane.tsx,
// os-workspaces-overview.tsx, workspace-management.tsx, session-list-create-menu.tsx.
const PROJECT_ARCHIVE = [
  i("archive", "Archive project", Archive),
  i("remove", "Remove project", Trash2, { destructive: true }),
];

export const PROJECTS: MenuFamily = {
  title: "Project",
  summary:
    "Six surfaces act on a project with four different verb sets: only the session list moves it between folders, no right-click can start a session, and the overview and Settings offer only Archive and Remove. One tree everywhere; New ▸ brings the + button's verbs to right-click.",
  today: [
    {
      surface: "Session list · project header",
      source: "session-list-group-project-menu.tsx",
      nodes: [
        i("files", "Open files", FolderOpen),
        sub("move-folder", "Move to folder", FolderInput, FOLDERS),
        i("out", "Move out of folder", FolderOutput),
        i("up", "Move project up", ArrowUp),
        i("down", "Move project down", ArrowDown),
        sep("s1"),
        ...PROJECT_ARCHIVE,
      ],
    },
    {
      surface: "Kanban · swimlane header",
      source: "sessions-board-swimlane.tsx",
      nodes: [i("files", "Open files", FolderOpen), sep("s1"), ...PROJECT_ARCHIVE],
    },
    {
      surface: "Overview tile · Settings row",
      source: "workspace-project-actions.tsx",
      nodes: PROJECT_ARCHIVE,
    },
  ],
  proposed: {
    surface: "Every project surface",
    source: "one project menu model",
    openSubs: ["new"],
    nodes: [
      sub("new", "New", Plus, [
        i("new-session", "New session", Plus),
        i("new-folder", "New folder", FolderPlus),
        i("new-agent", "Add always-on agent", Bot),
      ]),
      i("files", "Open files", FolderOpen),
      sep("s1"),
      sub("move", "Move", FolderInput, [
        group("folder", "Folder", [...FOLDERS, i("out", "Move out of folder", FolderOutput)]),
        sep("order"),
        i("up", "Move project up", ArrowUp),
        i("down", "Move project down", ArrowDown),
      ]),
      sep("s2"),
      ...PROJECT_ARCHIVE,
    ],
  },
};

// Pull request — session-pull-request-menu.tsx with the chip footer from
// session-linked-work-header.tsx and the row footer from session-link-row.tsx.
export const PULL_REQUEST: MenuFamily = {
  title: "Pull request",
  summary:
    "The topbar chip and a Linked work row show the same pull request with different menus: the row has no Review changes, Worktree or Link another…, and the chip spends a heading on where the session runs. Short enough to stay flat: the fix is one shared tree, with Runs here as a hint and Copy pull request link added.",
  today: [
    {
      surface: "Session topbar · PR chip",
      source: "session-pull-request-chip.tsx",
      nodes: [
        i("fix", "Fix failing checks", WrenchIcon),
        sep("s1"),
        i("open", "Open PR", ExternalLinkIcon),
        i("refresh", "Refresh", RefreshCwIcon),
        i("review", "Review changes", FileDiff),
        group("ask", "Ask the agent", [
          i("address", "Address review comments", MessageSquareTextIcon),
        ]),
        sep("s2"),
        group("runs", "This session runs in session-pr-chip", [
          i("worktree", "Worktree session-pr-chip", FolderGit2),
          i("link", "Link another…", Link2),
          i("unlink", "Unlink from this session", Unlink),
        ]),
      ],
    },
    {
      surface: "Linked work · row …",
      source: "session-link-row.tsx",
      nodes: [
        i("fix", "Fix failing checks", WrenchIcon),
        sep("s1"),
        i("open", "Open PR", ExternalLinkIcon),
        i("refresh", "Refresh", RefreshCwIcon),
        group("ask", "Ask the agent", [
          i("address", "Address review comments", MessageSquareTextIcon),
        ]),
        sep("s2"),
        i("unlink", "Unlink from this session", Unlink),
      ],
    },
  ],
  proposed: {
    surface: "PR chip, Linked work row, session menu",
    source: "pull-request-actions.ts",
    nodes: [
      i("fix", "Fix failing checks", WrenchIcon),
      i("open", "Open PR", ExternalLinkIcon),
      i("review", "Review changes", FileDiff),
      i("refresh", "Refresh", RefreshCwIcon),
      group("ask", "Ask the agent", [
        i("address", "Address review comments", MessageSquareTextIcon),
      ]),
      sep("s1"),
      i("worktree", "Worktree session-pr-chip", FolderGit2, { hint: "Runs here" }),
      i("copy", "Copy pull request link", Link2),
      sep("s2"),
      i("link", "Link another…", Link2),
      i("unlink", "Unlink from this session", Unlink),
    ],
  },
};

// Files — files-tree-row-menu.tsx + open-in-menu-items.tsx; open-in-button.tsx;
// session-change-file-header.tsx has buttons only.
const EDITORS = [
  i("vscode", "Visual Studio Code", Code2),
  i("zed", "Zed", Code2),
  i("sublime", "Sublime Text", Code2),
  i("xcode", "Xcode", Code2),
];

export const FILES: MenuFamily = {
  title: "File · Open in",
  summary:
    "A file row lists every detected editor, the reveal target and two maintenance verbs at the top level. The configured editor and Reveal stay one click away; the other editors and the list's upkeep move behind Open in ▸. A changed file in Changes gets the same menu, which today has no way to open it in an editor.",
  today: [
    {
      surface: "Files · tree row",
      source: "files-tree-row-menu.tsx",
      nodes: [
        i("copy", "Copy path", Copy),
        sep("s1"),
        i("cursor", "Cursor", Code2),
        sep("s2"),
        ...EDITORS,
        sep("s3"),
        i("reveal", "Finder", FolderOpen),
        sep("s4"),
        i("refresh", "Refresh list", RefreshCwIcon),
        i("settings", "Editor settings…", Settings2),
      ],
    },
    {
      surface: "Changes · file header",
      source: "session-change-file-header.tsx",
      nodes: [i("files", "Open in Files", FolderOpen), i("copy", "Copy path", Copy)],
    },
  ],
  proposed: {
    surface: "Tree row, viewer, folder header, Changes",
    source: "open-in-menu-items.tsx",
    openSubs: ["open-in"],
    nodes: [
      i("cursor", "Open in Cursor", Code2),
      sub("open-in", "Open in", AppWindow, [
        ...EDITORS,
        sep("s1"),
        i("refresh", "Refresh list", RefreshCwIcon),
        i("settings", "Editor settings…", Settings2),
      ]),
      i("reveal", "Reveal in Finder", FolderOpen),
      sep("s1"),
      i("copy", "Copy path", Copy),
    ],
  },
};

// Windows — os-window-deck.tsx (tab right-click) and menubar/window-menu.tsx.
export const WINDOWS: MenuFamily = {
  title: "Window tab",
  summary:
    "A tab's right-click cannot send its window to another desktop, float it or tile it, which the Window menu can. Close variants and moves group; the tab menu reaches the Window menu's verbs. Overlaps the window tabs and splits spec, so it follows that spec's decisions.",
  today: [
    {
      surface: "Window deck · tab right-click",
      source: "os-window-deck.tsx",
      nodes: [
        i("close", "Close tab", X),
        i("close-others", "Close other tabs", X),
        i("close-right", "Close tabs to the right", X),
        sep("s1"),
        i("pin", "Pin tab", Pin),
        i("new-window", "Move tab to new window", AppWindow),
        i("merge", "Merge all windows", Layers),
      ],
    },
  ],
  proposed: {
    surface: "Tab right-click",
    source: "os-window-deck.tsx",
    openSubs: ["move"],
    nodes: [
      i("close", "Close tab", X),
      sub("close-more", "Close", X, [
        i("close-others", "Close other tabs", X),
        i("close-right", "Close tabs to the right", X),
      ]),
      i("pin", "Pin tab", Pin),
      sep("s1"),
      sub("move", "Move", AppWindow, [
        i("new-window", "Move tab to new window", AppWindow),
        sub("desktop", "Move window to", Monitor, [
          i("d1", "Desktop 1", Monitor),
          i("d2", "Desktop 2", Monitor),
        ]),
        i("merge", "Merge all windows", Layers),
      ]),
      sub("arrange", "Move & resize", LayoutGrid, [
        i("left", "Left half", Columns2),
        i("right", "Right half", Columns2),
        i("fill", "Fill", Maximize2),
        sep("s1"),
        i("float", "Make window floating", AppWindow),
      ]),
    ],
  },
};

// Automations — automation-row-parts.tsx and automation-detail-actions.tsx.
export const AUTOMATIONS: MenuFamily = {
  title: "Automation",
  summary:
    "The list row and the detail page offer different verbs for one automation: only the row copies the webhook link, only the detail inspects or copies the id. One tree for both, with the copies behind Copy ▸.",
  today: [
    {
      surface: "Automations · row …",
      source: "automation-row-parts.tsx",
      nodes: [
        i("run", "Run now", Play),
        i("link", "Copy link", Link2),
        i("edit", "Edit", Pencil),
        sep("s1"),
        i("delete", "Delete automation…", Trash2, { destructive: true }),
      ],
    },
    {
      surface: "Automation detail · ⋯",
      source: "automation-detail-actions.tsx",
      nodes: [
        i("id", "Copy automation id", Hash),
        i("inspect", "Inspect", Search),
        sep("s1"),
        i("delete", "Delete automation…", Trash2, { destructive: true }),
      ],
    },
  ],
  proposed: {
    surface: "Row … and detail ⋯",
    source: "one automation menu model",
    openSubs: ["copy"],
    nodes: [
      i("run", "Run now", Play),
      i("edit", "Edit", Pencil),
      i("inspect", "Inspect", Search),
      sub("copy", "Copy", Clipboard, [
        i("link", "Copy link", Link2),
        i("id", "Copy automation id", Hash),
      ]),
      sep("s1"),
      i("delete", "Delete automation…", Trash2, { destructive: true }),
    ],
  },
};

export const FAMILIES: Record<string, MenuFamily> = {
  projects: PROJECTS,
  "pull-request": PULL_REQUEST,
  files: FILES,
  windows: WINDOWS,
  automations: AUTOMATIONS,
};
