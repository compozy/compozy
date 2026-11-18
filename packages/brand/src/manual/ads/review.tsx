import { cn } from "@compozy/ui/utils";

import { card, lab } from "../cls";
import { Section } from "../parts/section";
import { checklist } from "./data";

export function Review() {
  return (
    <Section
      id="review"
      n="06"
      file="review.md"
      title={
        <>
          Before it <em>runs</em>
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          Every ad passes this list before money goes behind it. One unchecked box in Claims stops
          the launch; anything else gets fixed or written down with a reason.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {checklist.map(([g, items], i) => (
          <div
            key={g}
            className={cn(
              card,
              i === 0 && "border-[color-mix(in_oklab,var(--color-destructive)_45%,transparent)]"
            )}
          >
            <div className="flex items-center justify-between">
              <p className={lab}>
                {String(i + 1).padStart(2, "0")} · {g}
              </p>
              {i === 0 && <span className="mono-label text-[11px] text-destructive">blocking</span>}
            </div>
            <ul className="m-0 mt-4 grid list-none gap-2.5 p-0 text-[14.5px]">
              {items.map(it => (
                <li key={it} className="grid grid-cols-[22px_minmax(0,1fr)] gap-2">
                  <span className="mt-[3px] grid h-4 w-4 place-items-center rounded-[4px] border border-(--b-muted)" />
                  <span>{it}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
        <div className={cn(card, "flex flex-col justify-between")}>
          <p className={lab}>Sign-off</p>
          <p className="m-0 mt-4 text-[15px]">
            The founder approves every new hook and every proof claim. Variants of an approved hook
            only need the checklist, filled in the ad’s ticket with the export link.
          </p>
          <p className="m-0 mt-6 font-mono text-[12px] text-(--b-muted)">owner · Pedro Nauck</p>
        </div>
      </div>
    </Section>
  );
}
