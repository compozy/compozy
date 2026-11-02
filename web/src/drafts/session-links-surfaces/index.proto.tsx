import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SESSION_LINK_SETS } from "@/systems/session/mocks/session-links-fixtures";

import { BoardSurface } from "./board-surface";
import { DialogSurface } from "./dialog-surface";
import { PaletteSurface } from "./palette-surface";

export const meta = definePrototype({
  title: "Session links · Shared by every direction",
  route: "/sessions",
  states: [
    { id: "board", label: "Kanban cards with +N PRs" },
    { id: "palette-find", label: "Palette finds a session by its PR" },
    { id: "palette-link", label: "Palette links to this session" },
    { id: "dialog-pick", label: "Link dialog, picking from many" },
    { id: "dialog-search", label: "Link dialog, search narrows it" },
    { id: "dialog-paste", label: "Link dialog, pasted link found" },
    { id: "dialog-unknown", label: "Link dialog, unknown number" },
    { id: "dialog-other-project", label: "Link dialog, another project's PR" },
    { id: "dialog-offline", label: "Link dialog, GitHub not connected" },
  ],
});

const DIALOGS: Record<string, Parameters<typeof DialogSurface>[0]["preset"]> = {
  "dialog-pick": { selected: ["pr-962", "pr-948"] },
  "dialog-search": { query: "login" },
  "dialog-paste": {
    query: "https://github.com/compozy/compozy/pull/963",
    selected: ["pr-963"],
  },
  "dialog-unknown": { query: "#99999" },
  "dialog-other-project": { query: "https://github.com/acme/billing/pull/212" },
  "dialog-offline": { offline: true },
};

export default function SessionLinksSurfacesDraft({ state }: PrototypeProps) {
  if (state === "board") return <BoardSurface />;
  if (state === "palette-find" || state === "palette-link") {
    return <PaletteSurface mode={state === "palette-find" ? "find" : "link"} />;
  }
  return <DialogSurface linked={SESSION_LINK_SETS.mixed} preset={DIALOGS[state] ?? {}} />;
}
