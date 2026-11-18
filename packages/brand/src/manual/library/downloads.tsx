import { cn } from "@compozy/ui/utils";

import { lab } from "../cls";
import { Section } from "../parts/section";
import { BRAND_BASE, groups, kb, type FileGroup } from "./files";

const accentLink = "text-(--b-accent)";

function GroupFiles({ group }: { group: FileGroup }) {
  if (group.files.length === 0) {
    return (
      <p
        data-download-pending={group.dir}
        className="m-0 border-t border-(--b-line) p-5 text-[14px] text-(--b-muted)"
      >
        Not published in this package yet:{" "}
        <code>brand/{group.dir === "." ? "" : `${group.dir}/`}</code>
      </p>
    );
  }
  return (
    <ul className="m-0 grid list-none gap-1.5 border-t border-(--b-line) p-4 sm:grid-cols-2 lg:grid-cols-3">
      {group.files.map(f => (
        <li key={f.path} className="min-w-0">
          <a
            href={f.path}
            download
            className="flex min-w-0 items-center justify-between gap-3 rounded-[10px] px-3 py-2 no-underline hover:bg-(--b-card-2)"
          >
            <span className="min-w-0 truncate font-mono text-[12.5px]">
              {f.path.replace(`${BRAND_BASE}/`, "")}
            </span>
            <span className="flex-none font-mono text-[11px] text-(--b-muted)">{kb(f.size)} ↓</span>
          </a>
        </li>
      ))}
    </ul>
  );
}

export function Downloads() {
  return (
    <Section
      id="downloads"
      n="01"
      file="brand/"
      title={
        <>
          Every official <em>file</em>
        </>
      }
      status="approved"
      intro={
        <p>
          The brand package lives in <code>brand/</code>. File names are stable public paths: link
          to them, don’t copy them into other repos by hand.
        </p>
      }
    >
      <div className="grid gap-3">
        {groups.map(g => (
          <details
            key={g.dir}
            className="group rounded-card border border-(--b-line) bg-(--b-card)"
            open={g.dir === "svg" || g.dir === "digital/web"}
          >
            <summary className="flex cursor-pointer list-none items-center justify-between gap-4 p-5 sm:p-6">
              <span>
                <span className="block font-display text-[22px] tracking-[-0.025em]">
                  {g.title}
                </span>
                <span className="mt-1 block text-[14px] text-(--b-muted)">{g.note}</span>
              </span>
              <span className="flex-none font-mono text-[12px] text-(--b-muted)">
                {g.files.length} files{" "}
                <span className="ml-2 inline-block transition-transform group-open:rotate-90">
                  ›
                </span>
              </span>
            </summary>
            <GroupFiles group={g} />
          </details>
        ))}
      </div>
      <p className={cn(lab, "mt-6 normal-case tracking-normal")}>
        Fonts:{" "}
        <a className={accentLink} href="https://fonts.google.com/specimen/Instrument+Sans">
          Instrument Sans
        </a>{" "}
        <a className={accentLink} href="https://fonts.google.com/specimen/Figtree">
          Figtree
        </a>{" "}
        and{" "}
        <a className={accentLink} href="https://fonts.google.com/specimen/Geist+Mono">
          Geist Mono
        </a>{" "}
        (OFL). In code, install{" "}
        <a className={accentLink} href="https://fontsource.org/fonts/instrument-sans">
          <code>@fontsource-variable/instrument-sans</code>
        </a>
        , <code>@fontsource-variable/figtree</code> and <code>@fontsource-variable/geist-mono</code>
        .
      </p>
    </Section>
  );
}
