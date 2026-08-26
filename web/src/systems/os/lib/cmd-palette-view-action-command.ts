import type {
  CmdPaletteViewAction,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "./cmd-palette-types";
import { parseExtensionName } from "./palette-view-registry";

export function extensionName(viewId: string): string | null {
  return parseExtensionName(viewId);
}

export function commandForViewAction(
  viewId: string,
  action: CmdPaletteViewAction,
  registry?: PaletteRegistry
): ResolvedPaletteCommand {
  const target = action.action;
  if (!target) throw new Error("This view action requires a live view program.");
  if (target.kind === "client_op") {
    throw new Error("View actions cannot run client operations.");
  }
  const shortcuts = action.shortcut ? [action.shortcut] : [];
  const id = viewActionCommandID(viewId, action);
  const catalog = registry?.byId.get(id);
  if (catalog) {
    return {
      ...catalog,
      confirmation: action.confirmation ?? catalog.confirmation,
      destructive: action.destructive ?? catalog.destructive,
    };
  }
  const available = target.kind !== "tool";
  const extension = extensionName(viewId);
  return {
    id,
    title: action.title,
    section: action.section ?? "View",
    icon: action.icon ?? "command",
    source: extension ? `ext.${extension}` : "core",
    available,
    reason: available ? "" : "this command is not in the catalog",
    bindings: shortcuts,
    alias: null,
    destructive: action.destructive ?? false,
    confirmation: action.confirmation ?? null,
    arguments: [],
    action: target,
    execution: { retry_safe: false, single_flight: false },
    availability_exempt: false,
    visible: true,
    chords: shortcuts,
  };
}

/** Maps an extension-local action tool to its daemon-canonical command id. */
export function viewActionCommandID(viewId: string, action: CmdPaletteViewAction): string {
  const target = action.action;
  if (target?.kind !== "tool" || !target.tool) return `view-action.${viewId}`;
  const tool = target.tool.trim();
  if (tool.startsWith("ext__")) return tool;
  if (tool.startsWith("ext.")) return tool.replaceAll(".", "__");
  const extension = extensionName(viewId);
  return extension ? `ext__${extension}__${tool}` : tool;
}
