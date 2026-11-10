import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { visuals } from "../art/visuals";
import { lab } from "../cls";
import { Section } from "../parts/section";
import { atmosphereUse } from "./data";

const atmosphere = visuals.filter(v => v.family === "atmosphere");

export function Atmosphere() {
  return (
    <Section
      id="atmosphere"
      n="03"
      file="atmosphere/"
      title={
        <>
          Atmosphere: seven approved <em>images</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          The Atmosphere set is closed: seven images, generated and approved one by one. Pieces crop
          and position them; they never recolour, mirror, stack or generate new ones. Each one ships
          at three sizes: 1280 px for thumbnails, 2560 px for the web, and a faithful 4× upscale for
          print and large exports in <code>brand/visuals/hi/</code> (6688 × 3764 landscape, 4488 ×
          5608 portrait; Magnific Precision, no invented detail).
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2">
        {atmosphere.map((v, i) => (
          <figure key={v.name} className={cn("m-0 min-w-0", i === 0 && "md:col-span-2")}>
            <div className="relative overflow-hidden rounded-card border border-(--b-line)">
              <Visual name={v.name} inline ratio={i === 0 ? "21/9" : "16/9"} />
            </div>
            <figcaption className="mt-3 grid gap-1">
              <span className="flex flex-wrap items-baseline gap-x-3">
                <span className="font-display text-[22px] tracking-[-0.015em] text-(--b-fg)">
                  {v.title}
                </span>
                <span className={lab}>
                  {v.name}
                  {v.portrait ? " · landscape + portrait" : " · landscape"}
                </span>
              </span>
              <span className="text-[15px] text-(--b-muted)">{v.idea}</span>
              <span className="font-mono text-[12px] text-(--b-muted)">
                Use · {atmosphereUse[v.name]} · focus {v.focus.join(", ")}
              </span>
            </figcaption>
          </figure>
        ))}
      </div>
    </Section>
  );
}
