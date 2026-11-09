/** After the catalog: how it reaches the agents, where else it can come from, and how to publish. */
import { Pill, cn } from "@compozy/ui";
import { Eye, FolderGit2, GitPullRequest, CheckCircle2, Search } from "@compozy/ui/icons";

import marketplace from "./assets/app-marketplace.jpg";
import { Reveal } from "../landing/motion";
import { CONTAINER, HAIRLINE } from "../landing/layout";
import {
  InstallCommand,
  H2,
  H3,
  Lead,
  Muted,
  SecondaryCta,
  Shot,
  type Icon,
} from "../landing/primitives";

const STEPS: ReadonlyArray<readonly [Icon, string, string]> = [
  [Search, "Find it in the app", "The same catalog, by category, in the Marketplace window."],
  [
    Eye,
    "See what it brings",
    "Each entry shows what it contains, where it comes from, and how far to trust it.",
  ],
  [
    CheckCircle2,
    "Add it once",
    "Installed entries stay marked in place, with one view to manage them all.",
  ],
];

export function FromTheApp() {
  return (
    <section className={cn("border-t", HAIRLINE)}>
      <div className={cn(CONTAINER, "py-28")}>
        <Reveal className="text-center">
          <H2 size="md">Add it from the app</H2>
        </Reveal>
        <Reveal delay={0.08} className="mt-14">
          <Shot
            src={marketplace}
            alt="The Marketplace in the Compozy app, by category, with one-step Add"
          />
        </Reveal>
        <ul className="m-0 mt-14 grid list-none gap-10 p-0 md:grid-cols-3">
          {STEPS.map(([Glyph, title, body]) => (
            <li key={title}>
              <Glyph aria-hidden className="size-5 text-brand-strong" />
              <p className="m-0 mt-4 text-[18px] font-medium text-fg">{title}</p>
              <Muted className="mt-1.5">{body}</Muted>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

const SOURCES = [
  ["Compozy plugins", "Curated app plugins, on by default."],
  ["Claude plugins", "Anthropic's official plugins, on by default."],
  ["Codex plugins", "OpenAI's plugin marketplace, off until you turn it on."],
];

const INCLUDED = [
  ["Spec Cycle", "Spec authoring and agent-led delivery in isolated worktrees."],
  [
    "Open Design",
    "Design interfaces, sites, slides, and prototypes built with your own components.",
  ],
  ["Compozy skill", "Lets your agents operate sessions, Loops, tasks, and desktops."],
];

function Rows({ rows }: { rows: string[][] }) {
  return (
    <ul className="m-0 mt-8 list-none p-0">
      {rows.map(([name, body]) => (
        <li key={name} className={cn("border-t py-4", HAIRLINE)}>
          <p className="m-0 text-[16px] font-medium text-fg">{name}</p>
          <Muted className="mt-0.5">{body}</Muted>
        </li>
      ))}
    </ul>
  );
}

export function SourcesAndIncluded() {
  return (
    <section className={cn("border-t", HAIRLINE)}>
      <div className={cn(CONTAINER, "grid gap-16 py-28 lg:grid-cols-2 lg:gap-20")}>
        <Reveal>
          <H3>Where the catalog comes from</H3>
          <Rows rows={SOURCES} />
          <div className="mt-8">
            <p className="m-0 flex items-center gap-2.5 text-[16px] font-medium text-fg">
              <FolderGit2 aria-hidden className="size-4 text-subtle" /> Or add your own
              <Pill size="xs" form="hollow">
                Experimental
              </Pill>
            </p>
            <Muted className="mt-1.5 mb-4">
              Any GitHub repository, HTTPS Git repository, or local folder with a marketplace file.
            </Muted>
            <InstallCommand
              command="compozy marketplace sources add example/team-plugins --name team"
              className="flex w-full justify-between h-auto min-h-[52px] rounded-[16px] py-2 [&_code]:[overflow-wrap:anywhere] [&_code]:whitespace-normal"
            />
          </div>
        </Reveal>
        <Reveal delay={0.1}>
          <H3>Included with Compozy</H3>
          <Rows rows={INCLUDED} />
          <Muted className="mt-6">Nothing to install. Updating Compozy updates these.</Muted>
        </Reveal>
      </div>
    </section>
  );
}

export function Publish() {
  return (
    <section className={cn("border-t", HAIRLINE)}>
      <div
        className={cn(
          CONTAINER,
          "flex flex-col items-start gap-8 py-24 md:flex-row md:items-center md:justify-between"
        )}
      >
        <div className="max-w-[640px]">
          <H2 size="sm">Publish yours</H2>
          <Lead className="mt-4">
            Package it, validate it, and open a pull request against the catalog.
          </Lead>
        </div>
        <SecondaryCta icon={GitPullRequest}>Read the contribution guide</SecondaryCta>
      </div>
    </section>
  );
}
