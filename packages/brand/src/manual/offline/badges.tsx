// 11 · 02 Badges: one family, four roles. Surface and art carry the role; the chip names it.
import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { badgeRules, roles } from "./data";

const th = "px-5 py-3 font-medium";

function Lanyard() {
  return (
    <div className={card}>
      <p className={lab}>lanyard · 20 mm · black polyester</p>
      <h3 className={cn(h3, "mt-3")}>Symbol repeated, one colour</h3>
      <div className="mt-4 grid gap-2.5" role="img" aria-label="Lanyard variants">
        <div className="flex h-9 items-center justify-between overflow-hidden rounded-lg bg-black px-3">
          {Array.from({ length: 10 }, (_, i) => (
            <img key={i} src="/brand/svg/compozy-symbol-mono-white.svg" alt="" className="size-4" />
          ))}
        </div>
        <div className="flex h-9 items-center justify-between overflow-hidden rounded-lg bg-black px-3 mono-label text-[11px] tracking-[0.2em] text-(--color-brand-cyan)">
          {Array.from({ length: 4 }, (_, i) => (
            <span key={i}>always-on</span>
          ))}
        </div>
      </div>
      <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
        Dye-sublimated, symbol every 60 mm. Team lanyards may use the cyan line; everyone else gets
        the symbol. Safety breakaway clasp at the neck.
      </p>
    </div>
  );
}

export function Badges() {
  return (
    <Section
      id="badges"
      n="02"
      file="templates/offline/EventBadge.astro"
      title={
        <>
          One badge, four <em>roles</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          At an event the badge is the brand most people hold. The role is read from the art and the
          chip that spells it out; the name is read from two metres. A6, portrait, slot for a 20 mm
          lanyard.
        </p>
      }
    >
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {roles.map(r => (
          <div key={r.id} className="min-w-0">
            <Frame id={r.id} guides />
            <p className="m-0 mt-1 text-sm">
              <b className="font-medium">{r.role}</b> ·{" "}
              <span className="text-(--b-muted)">{r.who}</span>
            </p>
          </div>
        ))}
      </div>
      <p className="m-0 mt-3 text-sm text-(--b-muted)">
        Hatched band: lanyard slot, keep clear. Fields <code>?name=</code>, <code>?company=</code>,{" "}
        <code>?event=</code>, <code>?date=</code> fill the kit route for batch export.
      </p>

      <div className="mt-8 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
        <table className="w-full min-w-[640px] border-collapse text-[14px]">
          <thead>
            <tr className="border-b border-(--b-line) text-left mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
              <th className={th}>Role</th>
              <th className={th}>Surface and art</th>
              <th className={th}>Text</th>
              <th className={th}>Kit</th>
            </tr>
          </thead>
          <tbody>
            {roles.map(r => (
              <tr key={r.id} className="border-b border-(--b-line) last:border-0">
                <td className="px-5 py-3 font-medium text-(--b-accent)">{r.role}</td>
                <td className="px-5 py-3">{r.surface}</td>
                <td className="px-5 py-3 text-(--b-muted)">{r.text}</td>
                <td className="px-5 py-3 font-mono text-[12.5px] text-(--b-accent)">
                  <a href={`/kit/${r.id}`} className="no-underline hover:underline">
                    kit/{r.id} ↗
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <h3 className={h3}>What never changes</h3>
          <ul className={cn(rules, "mt-3 text-[15px]")}>
            {badgeRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
        <Lanyard />
      </div>
    </Section>
  );
}
