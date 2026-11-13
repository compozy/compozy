import {
  Bot,
  FolderTree,
  LayoutDashboard,
  MessagesSquare,
  SquareTerminal,
  Zap,
  type LucideIcon,
} from "@compozy/ui/icons";

import type { DraftAppId } from "./model";

export const APP_ICON: Record<DraftAppId, LucideIcon> = {
  session: MessagesSquare,
  terminal: SquareTerminal,
  files: FolderTree,
  dashboard: LayoutDashboard,
  agents: Bot,
  automations: Zap,
};

export const APP_NAME: Record<DraftAppId, string> = {
  session: "Session",
  terminal: "Terminal",
  files: "Files",
  dashboard: "Dashboard",
  agents: "Agents",
  automations: "Automations",
};
