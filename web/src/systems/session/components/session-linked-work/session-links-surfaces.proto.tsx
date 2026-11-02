import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  SessionLinksBoardStage,
  SessionLinksDialogStage,
  SessionLinksPaletteStage,
  type LinkDialogPreset,
} from "@/drafts/stays-on-agents-shared/session-links-stage";

export const meta = definePrototype({
  title: "Session links · Board, palette and link dialog",
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

const DIALOGS: Record<string, LinkDialogPreset> = {
  "dialog-pick": { selected: [962, 948] },
  "dialog-search": { query: "login" },
  "dialog-paste": { query: "https://github.com/compozy/compozy/pull/963" },
  "dialog-unknown": { query: "#99999" },
  "dialog-other-project": { query: "https://github.com/acme/billing/pull/212" },
  "dialog-offline": { offline: true },
};

export default function SessionLinksSurfacesPrototype({ state }: PrototypeProps) {
  if (state === "board") return <SessionLinksBoardStage />;
  if (state === "palette-find" || state === "palette-link") {
    return <SessionLinksPaletteStage mode={state === "palette-find" ? "find" : "link"} />;
  }
  return <SessionLinksDialogStage preset={DIALOGS[state] ?? {}} />;
}
