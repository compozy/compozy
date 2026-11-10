import { cn } from "@compozy/ui/utils";

import { lab, rules } from "../cls";
import { Frame } from "../kit-frame";
import { kitSpecById } from "../kit-specs";
import { InContext } from "../parts/in-context";
import { Section } from "../parts/section";
import { models, type Model } from "./data";

const ratio = (m: Model) => {
  const t = kitSpecById(m.id);
  return t.w / t.h;
};
const wide = models.filter(m => ratio(m) > 1.5);
const tall = models.filter(m => ratio(m) <= 1.5);

function ModelCard({ m }: { m: Model }) {
  return (
    <div className="flex min-w-0 flex-col">
      <Frame id={m.id} guides />
      <div className="mt-4 flex items-center gap-2">
        <span
          className={cn(
            "rounded-md px-2 py-0.5 mono-label text-[11px]",
            m.mode === "brand"
              ? "border border-(--b-line) text-(--b-fg)"
              : "bg-[color-mix(in_oklab,var(--color-brand-cyan)_14%,transparent)] text-(--b-accent)"
          )}
        >
          {m.mode}
        </span>
        <span className="text-[13px] text-(--b-muted)">{m.where}</span>
      </div>
      <p className="m-0 mt-3 text-[14.5px]">{kitSpecById(m.id).use}.</p>
      <ul className={cn(rules, "mt-3 text-[14px] text-(--b-muted)")}>
        {m.guide.map(g => (
          <li key={g}>{g}</li>
        ))}
      </ul>
    </div>
  );
}

export function Models() {
  return (
    <Section
      id="models"
      n="03"
      file="templates/ads.ts"
      title={
        <>
          Eight <em>models</em>, live
        </>
      }
      status="proposal"
      intro={
        <p>
          Every ad starts from one of these. They are live templates: the dashed box is the content
          margin, hatched bands are covered by platform UI, and each opens at native size for
          export.
        </p>
      }
    >
      <div className="grid gap-x-6 gap-y-14 md:grid-cols-2 lg:grid-cols-4">
        {tall.map(m => (
          <ModelCard key={m.id} m={m} />
        ))}
      </div>
      <div className="mt-16 grid gap-x-6 gap-y-14 md:grid-cols-2">
        {wide.map(m => (
          <ModelCard key={m.id} m={m} />
        ))}
      </div>
      <p className={cn(lab, "mt-12")}>
        Fields are editable: {"/kit/<id>?hook=…&cta=…"} overrides any data-field before export.
      </p>
      <InContext ids={["billboard", "bus-shelter"]} title="In context · the same ads out of home" />
    </Section>
  );
}
