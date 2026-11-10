/** After the catalog: how it reaches the agents, where else it can come from, and how to publish. */
import { Pill, cn } from "@compozy/ui";
import { CheckCircle2, Eye, FolderGit2, GitPullRequest, Search } from "@compozy/ui/icons";
import Link from "next/link";

import marketplace from "@/components/marketplace/assets/app-marketplace.jpg";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Reveal } from "@/components/site/motion";
import {
  H2,
  H3,
  InstallCommand,
  Lead,
  Muted,
  SecondaryCta,
  Shot,
  type Icon,
} from "@/components/site/primitives";
import { bundledExtensions } from "@/lib/marketplace-bundled";
import { siteConfig } from "@/lib/site-config";

const CONTRIBUTION_GUIDE_URL = `${siteConfig.repoUrl}/tree/main/catalog`;

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
            src={marketplace.src}
            alt="The Marketplace in the CompozyOS app, by category, with one-step Add"
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

interface SourceRow {
  name: string;
  body: string;
  href?: string;
}

const SOURCES: SourceRow[] = [
  { name: "CompozyOS plugins", body: "Curated app plugins, on by default." },
  { name: "Claude plugins", body: "Anthropic's official plugins, on by default." },
  { name: "Codex plugins", body: "OpenAI's plugin marketplace, off until you turn it on." },
];

/** The draft's copy per bundled extension; names and links come from the bundled loader. */
const INCLUDED_COPY: Record<string, string> = {
  "spec-cycle": "Spec authoring and agent-led delivery in isolated worktrees.",
  "open-design": "Design interfaces, sites, slides, and prototypes built with your own components.",
};

const INCLUDED: SourceRow[] = [
  ...bundledExtensions.map(extension => ({
    name: extension.displayName,
    body: INCLUDED_COPY[extension.name] ?? extension.description,
    href: extension.path,
  })),
  {
    name: "CompozyOS skill",
    body: "Lets your agents operate sessions, Loops, tasks, and desktops.",
  },
];

function Rows({ rows }: { rows: readonly SourceRow[] }) {
  return (
    <ul className="m-0 mt-8 list-none p-0">
      {rows.map(row => (
        <li key={row.name} className={cn("border-t py-4", HAIRLINE)}>
          <p className="m-0 text-[16px] font-medium text-fg">
            {row.href ? (
              <Link
                href={row.href}
                className="underline-offset-4 outline-none hover:underline focus-visible:underline"
              >
                {row.name}
              </Link>
            ) : (
              row.name
            )}
          </p>
          <Muted className="mt-0.5">{row.body}</Muted>
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
              className="flex h-auto min-h-[52px] w-full justify-between rounded-[16px] py-2 [&_code]:flex-1 [&_code]:whitespace-normal [&_code]:[overflow-wrap:anywhere]"
            />
          </div>
        </Reveal>
        <Reveal delay={0.1}>
          <H3>Included with CompozyOS</H3>
          <Rows rows={INCLUDED} />
          <Muted className="mt-6">Nothing to install. Updating CompozyOS updates these.</Muted>
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
        <SecondaryCta icon={GitPullRequest} href={CONTRIBUTION_GUIDE_URL}>
          Read the contribution guide
        </SecondaryCta>
      </div>
    </section>
  );
}
