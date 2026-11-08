// The Compozy product icon set: 24 px grid, 20 px live area, 1.5 px stroke, round caps and joins.
// Paths are inner SVG markup for a 24×24 viewBox; render with stroke="currentColor" fill="none".
// Shared by the Graphic system (chapter 04) and the Product chapter (05).

export type IconName =
  | "session"
  | "loop"
  | "task"
  | "automation"
  | "agent"
  | "skill"
  | "extension"
  | "terminal"
  | "vault"
  | "approval"
  | "worktree"
  | "memory"
  | "trigger"
  | "desktop"
  | "gateway"
  | "network";

export const icons: Record<IconName, { label: string; meaning: string; path: string }> = {
  session: {
    label: "Session",
    meaning: "A durable agent run with its history",
    path: '<rect x="3.5" y="4" width="17" height="16" rx="3"/><path d="M7.5 9h9M7.5 12.5h5.5"/><circle cx="16.25" cy="16" r="1.25"/>',
  },
  loop: {
    label: "Loop",
    meaning: "Goal, act, verify, stop",
    path: '<ellipse cx="12" cy="12.4" rx="8.6" ry="5.6" transform="rotate(-50 12 12.4)"/><circle cx="17.6" cy="5.8" r="1.1" fill="currentColor" stroke="none"/>',
  },
  task: {
    label: "Task",
    meaning: "A unit of work with a done state",
    path: '<rect x="4" y="4" width="16" height="16" rx="3.5"/><path d="M8.25 12.2l2.6 2.6 5-5.3"/>',
  },
  automation: {
    label: "Automation",
    meaning: "Work started on a schedule or event",
    path: '<path d="M20 12a8 8 0 1 1-2.34-5.66"/><path d="M20 4.25V8.5h-4.25"/><path d="M12 8.25V12l2.5 1.6"/>',
  },
  agent: {
    label: "Agent",
    meaning: "Claude Code, Codex, Cursor and others",
    path: '<rect x="4.5" y="7.5" width="15" height="11.5" rx="3.5"/><path d="M12 7.5V4.5M9.5 12.5v1.5M14.5 12.5v1.5"/><path d="M2.5 12v2.5M21.5 12v2.5"/>',
  },
  skill: {
    label: "Skill",
    meaning: "Instructions an agent loads on demand",
    path: '<path d="M12 6.6C10 5.1 7 4.6 4 5.1v13c3-.5 6 0 8 1.5 2-1.5 5-2 8-1.5v-13c-3-.5-6 0-8 1.5z"/><path d="M12 6.6v13"/>',
  },
  extension: {
    label: "Extension",
    meaning: "Code that adds tools, hooks or apps",
    path: '<rect x="4" y="4" width="7" height="7" rx="2"/><rect x="13" y="4" width="7" height="7" rx="2"/><rect x="4" y="13" width="7" height="7" rx="2"/><path d="M16.5 13.5v6M13.5 16.5h6"/>',
  },
  terminal: {
    label: "Terminal",
    meaning: "A shell the agent and you share",
    path: '<rect x="3" y="4.5" width="18" height="15" rx="3"/><path d="M7 9.5l3 2.5-3 2.5M12.5 15H17"/>',
  },
  vault: {
    label: "Vault",
    meaning: "Secrets the agent can use, never read",
    path: '<rect x="4.5" y="10.5" width="15" height="9.5" rx="2.5"/><path d="M8 10.5V8a4 4 0 0 1 8 0v2.5M12 14.25v2"/>',
  },
  approval: {
    label: "Tool approval",
    meaning: "A person decides before the tool runs",
    path: '<path d="M12 3.5l7 2.6v5.4c0 4.2-2.9 7.4-7 9-4.1-1.6-7-4.8-7-9V6.1z"/><path d="M8.75 12.1l2.25 2.25 4.25-4.5"/>',
  },
  worktree: {
    label: "Worktree",
    meaning: "An isolated checkout per session",
    path: '<circle cx="7" cy="5.5" r="2"/><circle cx="7" cy="18.5" r="2"/><circle cx="17" cy="8" r="2"/><path d="M7 7.5v9M17 10v.5a4 4 0 0 1-4 4h-3.5A2.5 2.5 0 0 0 7 17"/>',
  },
  memory: {
    label: "Memory",
    meaning: "What carries over between sessions",
    path: '<path d="M12 4l8 4-8 4-8-4z"/><path d="M4 12l8 4 8-4M4 16l8 4 8-4"/>',
  },
  trigger: {
    label: "Trigger",
    meaning: "The event that starts an automation",
    path: '<path d="M13.2 3L5.5 13.5h6.2L10.8 21l7.7-10.5h-6.2z"/>',
  },
  desktop: {
    label: "Desktop",
    meaning: "The desktop: menubar, app rail, windows",
    path: '<rect x="3" y="4" width="18" height="12" rx="2.5"/><path d="M9 20h6M12 16v4"/>',
  },
  gateway: {
    label: "Gateway",
    meaning: "Where outside channels reach a session",
    path: '<path d="M12 3.5v17"/><path d="M3.5 8.5h5.5M6.75 6.25L9 8.5l-2.25 2.25M20.5 15.5H15M17.25 13.25L15 15.5l2.25 2.25"/>',
  },
  network: {
    label: "Network",
    meaning: "Sessions handing work to each other",
    path: '<circle cx="12" cy="5" r="2"/><circle cx="5" cy="18" r="2"/><circle cx="19" cy="18" r="2"/><path d="M11 6.8l-5 9.4M13 6.8l5 9.4M7 18h10"/>',
  },
};

export const iconNames = Object.keys(icons) as IconName[];

/** terminal glyphs: one meaning each, the same in CLI, TUI, product rows and marketing */
export const glyphs: [glyph: string, name: string, meaning: string, colour: string][] = [
  ["●", "filled dot", "Working: the session is running now", "cyan"],
  ["◐", "half dot", "Needs you: waiting for an answer or approval", "warning"],
  ["✓", "check", "Done: finished and verified", "success"],
  ["■", "square", "Stuck or stopped: no progress, needs a look", "danger"],
  ["▸", "triangle", "Step or child: a line that belongs to the one above", "muted"],
  ["→", "arrow", "Handoff or result: this leads to that", "muted"],
];
