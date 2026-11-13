import { toast, tidyMenuTree, type MenuTreeNode } from "@compozy/ui";
import { Clipboard, Hash, Link2, Lock, Pencil, Play, Search, Trash2 } from "@compozy/ui/icons";

import type { AutomationView } from "./automation-view";

export interface AutomationMenuActions {
  /** Automations are unavailable: the daemon would refuse the run. */
  runDisabled: boolean;
  runPending: boolean;
  onRunNow: () => void;
  onEdit: () => void;
  onInspect: () => void;
  onDelete: () => void;
}

/** Per-surface test ids; a verb without one renders none. */
export type AutomationMenuTestIds = Partial<
  Record<"runNow" | "edit" | "editInConfig" | "inspect" | "copyId" | "delete", string>
>;

/** Copies a webhook automation's public link. */
function copyAutomationLink(view: AutomationView): void {
  if (!view.webhookPath) return;
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (!clipboard) {
    toast.error("Couldn't copy the link.");
    return;
  }
  void clipboard
    .writeText(`${window.location.origin}${view.webhookPath}`)
    .then(() => toast.success("Copied link."))
    .catch(() => toast.error("Couldn't copy the link."));
}

function copyAutomationId(view: AutomationView): void {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (!clipboard) {
    toast.error("Could not copy the automation id.");
    return;
  }
  void clipboard
    .writeText(view.id)
    .then(() => toast.success(`Copied ${view.id}.`))
    .catch(() => toast.error("Could not copy the automation id."));
}

/** The copies: behind Copy ▸ when there are several, else the single one in its place. */
function copyNodes(view: AutomationView, testIds: AutomationMenuTestIds): MenuTreeNode[] {
  const copies: MenuTreeNode[] = [
    ...(view.webhookPath
      ? [
          {
            kind: "item",
            id: "copy-link",
            label: "Copy link",
            icon: Link2,
            onSelect: () => copyAutomationLink(view),
          } as const,
        ]
      : []),
    {
      kind: "item",
      id: "copy-id",
      label: "Copy automation id",
      icon: Hash,
      testId: testIds.copyId,
      onSelect: () => copyAutomationId(view),
    },
  ];
  if (copies.length === 1) return copies;
  return [{ kind: "sub", id: "copy", label: "Copy", icon: Clipboard, children: copies }];
}

/**
 * The one automation menu, shared by the list row's `…` and the detail page's `⋯`. Only the
 * verbs the daemon supports for this kind and source (Business Rules 10–11): Run now for
 * schedules, Edit and Delete for automations created here, a disabled Edit in config.toml for a
 * config-owned one; Inspect and the copies for every automation, behind Copy ▸ only when there are
 * two (a webhook's link and its id).
 */
export function automationMenuNodes(
  view: AutomationView,
  actions: AutomationMenuActions,
  testIds: AutomationMenuTestIds = {}
): MenuTreeNode[] {
  return tidyMenuTree([
    ...(view.canRunNow
      ? [
          {
            kind: "item",
            id: "run",
            label: actions.runPending ? "Starting…" : "Run now",
            icon: Play,
            testId: testIds.runNow,
            disabled: actions.runDisabled || actions.runPending,
            onSelect: actions.onRunNow,
          } as const,
        ]
      : []),
    ...(view.canEdit
      ? [
          {
            kind: "item",
            id: "edit",
            label: "Edit",
            icon: Pencil,
            testId: testIds.edit,
            onSelect: actions.onEdit,
          } as const,
        ]
      : view.source === "config"
        ? [
            {
              kind: "item",
              id: "edit-in-config",
              label: "Edit in config.toml",
              icon: Lock,
              testId: testIds.editInConfig,
              disabled: true,
            } as const,
          ]
        : []),
    {
      kind: "item",
      id: "inspect",
      label: "Inspect",
      icon: Search,
      testId: testIds.inspect,
      onSelect: actions.onInspect,
    },
    ...copyNodes(view, testIds),
    { kind: "separator", id: "danger" },
    ...(view.canEdit
      ? [
          {
            kind: "item",
            id: "delete",
            label: "Delete automation…",
            icon: Trash2,
            testId: testIds.delete,
            destructive: true,
            onSelect: actions.onDelete,
          } as const,
        ]
      : []),
  ]);
}
