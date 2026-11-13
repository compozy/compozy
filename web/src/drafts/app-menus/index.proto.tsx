/**
 * The session menu's pattern applied to the rest of the app: each state puts one object's menus
 * as they ship today beside the one grouped tree they would all share. Every menu is the shipped
 * `@compozy/ui` dropdown pinned open; right-click and hover work as in the app.
 */
import { Button, Eyebrow, TopbarOverflowIcon } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SessionDropdownMenu } from "../session-menus/session-menu";
import { FAMILIES, type MenuSpecimen } from "./app-menu-trees";

export const meta = definePrototype({
  title: "App menus: grouped and at parity",
  route: "/",
  states: [
    { id: "projects", label: "Project · six surfaces, one menu" },
    { id: "pull-request", label: "Pull request · chip and Linked work row" },
    { id: "files", label: "File · Open in" },
    { id: "windows", label: "Window tab · reaches the Window menu" },
    { id: "automations", label: "Automation · row and detail" },
  ],
});

function Specimen({ specimen, proposed }: { specimen: MenuSpecimen; proposed: boolean }) {
  return (
    <div className="flex w-64 shrink-0 flex-col gap-2">
      <div className="flex flex-col gap-0.5">
        <span className="text-small-body font-medium text-fg">{specimen.surface}</span>
        <span className="font-mono text-eyebrow text-subtle">{specimen.source}</span>
      </div>
      <SessionDropdownMenu
        align="start"
        nodes={specimen.nodes}
        open
        openSubs={proposed ? specimen.openSubs : undefined}
        trigger={
          <Button aria-label={`${specimen.surface} menu`} size="icon-sm" variant="ghost">
            <TopbarOverflowIcon aria-hidden="true" />
          </Button>
        }
      />
    </div>
  );
}

export default function AppMenusDraft({ state }: PrototypeProps) {
  const family = FAMILIES[state] ?? FAMILIES.projects!;
  return (
    <main className="flex min-h-dvh flex-col gap-6 bg-canvas p-8">
      <header className="flex max-w-3xl flex-col gap-1">
        <Eyebrow>{family.title}</Eyebrow>
        <p className="text-body text-muted">{family.summary}</p>
      </header>
      <div className="flex min-h-[640px] items-start gap-10">
        <section aria-label="Today" className="flex flex-col gap-3">
          <Eyebrow>Today</Eyebrow>
          <div className="flex items-start gap-6">
            {family.today.map(specimen => (
              <Specimen key={specimen.surface} proposed={false} specimen={specimen} />
            ))}
          </div>
        </section>
        <section
          aria-label="Proposed"
          className="flex flex-col gap-3 border-l border-border-subtle pl-10"
        >
          <Eyebrow>Proposed</Eyebrow>
          <Specimen proposed specimen={family.proposed} />
        </section>
      </div>
    </main>
  );
}
