"use client";

/** After the hero: the first few minutes, the terminal route, and every package in the release. */
import { Button, Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { ArrowUpRight, ChevronDown, Download } from "@compozy/ui/icons";
import { useState } from "react";

import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Entrance, Reveal } from "@/components/site/motion";
import { H2, H3, InstallCommand, Lead, Muted, Small } from "@/components/site/primitives";
import type { DesktopRelease } from "@/lib/release/desktop-downloads";
import {
  commandGroups,
  desktopGroups,
  megabytes,
  type DownloadItem,
  type Platform,
} from "@/lib/release/download-offer";
import { STEPS, installMethods } from "./content";

export function FirstRun({ platform }: { platform: Platform }) {
  return (
    <section className={cn("border-t", HAIRLINE)}>
      <div className={cn(CONTAINER, "py-20 md:py-24")}>
        <H3>After you download</H3>
        <ol className="m-0 mt-8 grid list-none gap-8 p-0 md:grid-cols-3">
          {STEPS[platform].map((step, index) => (
            <li key={step.title} className="flex gap-4">
              <span className="font-mono text-[14px] text-subtle tabular-nums">{index + 1}</span>
              <div>
                <p className="m-0 text-[16px] font-medium text-fg">{step.title}</p>
                <Muted className="mt-1.5">{step.body}</Muted>
              </div>
            </li>
          ))}
        </ol>
        <Muted className="mt-10">
          You'll need one agent CLI, like Claude Code, Codex, or Cursor, signed in the way it
          normally is.
        </Muted>
      </div>
    </section>
  );
}

const TAB = "h-9 rounded-[10px] px-3.5 text-[14px]";

export function FromTheTerminal({ tag }: { tag: string | null }) {
  const methods = installMethods(tag);
  return (
    <section className={cn("border-t", HAIRLINE)}>
      <div
        className={cn(
          CONTAINER,
          "grid grid-cols-[minmax(0,1fr)] items-center gap-10 py-20 md:py-24 lg:grid-cols-[1fr_1.2fr] lg:gap-20"
        )}
      >
        <Reveal>
          <H2 size="sm">Prefer the terminal?</H2>
          <Lead className="mt-4 max-w-[480px] md:text-[18px]">
            Install the compozy command on its own. It uses the same local runtime, sessions, and
            settings as the app.
          </Lead>
        </Reveal>
        <Reveal delay={0.1}>
          <div className="min-w-0 rounded-[22px] border border-site-light-120 bg-[#101010] p-2">
            <Tabs defaultValue="Installer" className="gap-0">
              <TabsList className="h-auto flex-wrap gap-1 border-b-0">
                {methods.map(item => (
                  <TabsTrigger key={item.id} value={item.id} className={TAB}>
                    {item.id}
                  </TabsTrigger>
                ))}
              </TabsList>
              {methods.map(item => (
                <TabsContent key={item.id} value={item.id} className="min-w-0 p-5 pt-6">
                  {/* Long commands scroll inside the box instead of breaking mid-URL. */}
                  <InstallCommand
                    command={item.command}
                    className="flex h-auto min-h-[52px] w-full justify-between rounded-[16px] py-2 [&_code]:flex-1 [&_code]:overflow-x-auto [&_code]:text-clip [&_code]:whitespace-nowrap [&_code]:[scrollbar-width:none]"
                  />
                  <Small className="mt-4">{item.note}</Small>
                </TabsContent>
              ))}
            </Tabs>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function Row({ item }: { item: DownloadItem }) {
  return (
    <li className={cn("border-t", HAIRLINE)}>
      <a
        href={item.href}
        className="group/row flex items-center gap-4 py-4 transition-colors hover:text-fg"
      >
        <span className="w-[150px] shrink-0 text-[15.5px] text-fg">
          {item.name} <span className="text-muted-foreground">· {item.detail}</span>
        </span>
        <span className="hidden min-w-0 flex-1 truncate font-mono text-[13px] text-subtle md:block">
          {item.fileName}
        </span>
        <span className="ml-auto shrink-0 font-mono text-[13px] text-muted-foreground">
          {item.size === null ? "" : megabytes(item.size)}
        </span>
        <Download
          aria-hidden
          className="size-4 shrink-0 text-subtle transition-colors group-hover/row:text-fg"
        />
      </a>
    </li>
  );
}

function Group({ label, note, items }: { label: string; note: string; items: DownloadItem[] }) {
  return (
    <div className="grid gap-3 py-7 md:grid-cols-[220px_minmax(0,1fr)] md:gap-10">
      <div>
        <p className="m-0 text-[17px] font-medium text-fg">{label}</p>
        <Small className="mt-1">{note}</Small>
      </div>
      <ul className="m-0 list-none p-0 [&>li:first-child]:border-t-0 [&>li:first-child>a]:pt-0.5">
        {items.map(item => (
          <Row key={`${item.name}-${item.detail}`} item={item} />
        ))}
      </ul>
    </div>
  );
}

function releaseLinks(release: DesktopRelease) {
  const ready = release.status === "ready" ? release : null;
  return [
    ["Release notes", release.releaseUrl],
    ["Checksums", ready?.checksums?.url ?? release.releaseUrl],
    ["Sigstore signature", ready?.signature?.url ?? release.releaseUrl],
    ["All releases", "https://github.com/compozy/compozy/releases"],
  ] as const;
}

/** `releaseDate` is formatted on the server so the changelog parser stays out of the client bundle. */
export function AllDownloads({
  release,
  releaseDate,
}: {
  release: DesktopRelease;
  releaseDate: string | null;
}) {
  const [more, setMore] = useState(false);
  const groups = [...desktopGroups(release), ...(more ? commandGroups(release) : [])];
  return (
    <section id="all-downloads">
      <div className={cn(CONTAINER, "pb-20 md:pb-24")}>
        <Entrance>
          <div className="flex flex-wrap items-baseline justify-between gap-4">
            <H2 size="sm">All downloads</H2>
            <p className="m-0 font-mono text-[14px] text-muted-foreground">
              {release.status === "ready"
                ? [release.tag, releaseDate].filter(Boolean).join(" · ")
                : "Latest release"}
            </p>
          </div>
        </Entrance>
        <div className="mt-10 divide-y divide-site-light-080 border-y border-site-light-080">
          {groups.map(group =>
            group.items.length > 0 ? (
              <Group
                key={`${group.label}-${group.note}`}
                label={group.label}
                note={group.note}
                items={group.items}
              />
            ) : null
          )}
        </div>
        <div className="mt-8 flex flex-wrap items-center justify-between gap-5">
          <Button variant="outline" onClick={() => setMore(value => !value)} aria-expanded={more}>
            {more ? "Fewer packages" : "Command-line packages"}
            <ChevronDown className={cn("transition-transform", more && "rotate-180")} />
          </Button>
          <nav aria-label="Release files" className="flex flex-wrap gap-x-6 gap-y-2">
            {releaseLinks(release).map(([label, href]) => (
              <a
                key={label}
                href={href}
                className="inline-flex items-center gap-1 text-[14px] text-muted-foreground transition-colors hover:text-fg"
              >
                {label} <ArrowUpRight aria-hidden className="size-3.5" />
              </a>
            ))}
          </nav>
        </div>
      </div>
    </section>
  );
}
