import { cn } from "@compozy/ui/utils";

import { lab, rules } from "../cls";
import { GenImage } from "../parts/gen-image";
import { Section } from "../parts/section";
import { photo, types } from "./data";

export function Types() {
  return (
    <Section
      id="types"
      n="02"
      file="types.md"
      bg="ink2"
      title={
        <>
          Five kinds of <em>picture</em>
        </>
      }
      intro={
        <p>
          Every photo Compozy publishes is one of five types. Each has one job, a place it is used
          and three rules for the shoot. A picture that fits none of them does not run.
        </p>
      }
    >
      <div className="grid gap-14">
        {types.map((t, i) => {
          const p = photo(t.id);
          return (
            <div
              key={t.id}
              className={cn(
                "grid items-center gap-6 lg:grid-cols-2 lg:gap-12",
                i % 2 === 1 && "lg:[&>figure]:order-2"
              )}
            >
              <GenImage
                src={p.src}
                alt={p.alt}
                caption={p.caption}
                ratio={p.ratio}
                className={p.ratio === "4/5" ? "mx-auto w-full max-w-[460px]" : undefined}
              />
              <div>
                <p className={lab}>
                  Type 0{i + 1}
                  {t.id === "laptop-closed" ? " · the hero" : ""}
                </p>
                <h3 className="m-0 mt-3 font-display text-[40px] leading-[1.05] tracking-[-0.03em]">
                  {t.name}
                </h3>
                <p className="m-0 mt-3 text-[16px] text-(--b-muted)">
                  <span className="text-(--b-fg)">Used in:</span> {t.use}
                </p>
                <ul className={cn(rules, "mt-5 text-[15.5px]")}>
                  {t.rules.map(r => (
                    <li key={r}>{r}</li>
                  ))}
                </ul>
              </div>
            </div>
          );
        })}
      </div>
      <p className="mt-14 max-w-[64ch] text-[15px] text-(--b-muted)">
        The promise image carries the brand line “agents that keep running”. Use it once per
        surface, at the largest size on the page, with the headline beside it rather than on it.
      </p>
    </Section>
  );
}
