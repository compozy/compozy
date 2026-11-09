import { cn } from "@compozy/ui/utils";
import { card, h3, lab } from "../cls";
import { grade, ratio } from "../contrast";
import { Section } from "../parts/section";
import { colors, contrastPairs, proportion } from "./data";

const GRADE_TONE: Record<string, string> = {
  fail: "text-destructive",
  "AA large": "text-warning",
};

function Swatches() {
  return (
    <>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {colors.core.map(c => (
          <button
            key={c.name}
            type="button"
            data-copy={c.hex}
            className="group grid min-h-[280px] cursor-pointer content-between rounded-card border border-(--b-line) p-6 text-left"
            style={{ background: c.fill, color: c.ink }}
          >
            <span className="font-mono text-[12px]">{c.token}</span>
            <span>
              <span className="block font-display text-[30px] tracking-[-0.02em]">{c.name}</span>
              <span className="block font-mono text-[13px]">{c.hex}</span>
              <span className="mt-3 block text-[14px]">{c.role}</span>
            </span>
          </button>
        ))}
      </div>
      <div className="mt-4 overflow-hidden rounded-card border border-(--b-line)">
        <div className="h-28 [background:var(--gradient-brand)]" />
        <div className="grid gap-px bg-(--b-line) sm:grid-cols-3">
          {colors.gradient.map(c => (
            <button
              key={c.name}
              type="button"
              data-copy={c.hex}
              className="cursor-pointer bg-(--b-card) p-5 text-left"
            >
              <span className="mb-3 block h-3 w-10 rounded-full" style={{ background: c.fill }} />
              <span className="block font-display text-[19px]">
                {c.name} <span className="font-mono text-[12px] text-(--b-muted)">{c.hex}</span>
              </span>
              <span className="mt-1 block text-[13.5px] text-(--b-muted)">{c.role}</span>
            </button>
          ))}
        </div>
        <p className="m-0 border-t border-(--b-line) bg-(--b-card) px-5 py-3 font-mono text-[12px] text-(--b-muted)">
          --gradient-brand: linear-gradient(45deg, #1F87FC 0%, #1DF0D7 60%, #B5FAF6 100%)
        </p>
      </div>

      <h3 className={`${h3} mt-16`}>Neutrals</h3>
      <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8">
        {colors.neutrals.map(c => (
          <button
            key={c.name}
            type="button"
            data-copy={c.hex}
            className="cursor-pointer border-0 bg-transparent p-0 text-left text-(--b-fg)"
          >
            <span
              className="block aspect-[4/5] rounded-[14px] border border-(--b-line)"
              style={{ background: c.fill }}
            />
            <span className="mt-2 block text-[14px] font-medium">{c.name}</span>
            <span className="block font-mono text-[11.5px] text-(--b-muted)">{c.hex}</span>
            <span className="block text-[12.5px] text-(--b-muted)">{c.role}</span>
          </button>
        ))}
      </div>
    </>
  );
}

function ContrastTable() {
  return (
    <div className={card}>
      <p className={lab}>Contrast · WCAG 2.2</p>
      <table className="mt-4 w-full border-collapse text-[14px]">
        <tbody>
          {contrastPairs.map(p => {
            const r = ratio(p.fg, p.bg);
            const g = grade(r);
            return (
              <tr key={p.use} className="border-b border-(--b-line)">
                <td className="py-2.5 pr-3">
                  <span
                    className={cn(
                      "inline-grid h-9 w-14 place-items-center rounded-md font-display shadow-[inset_0_0_0_1px_var(--b-line)]",
                      g === "AA large" ? "text-[24px]" : "text-[16px]"
                    )}
                    style={{ background: p.bgFill, color: p.fgFill }}
                  >
                    Aa
                  </span>
                </td>
                <td className="py-2.5 pr-3">{p.use}</td>
                <td className="py-2.5 pr-3 font-mono text-[12.5px] text-(--b-muted)">
                  {p.fg} / {p.bg}
                </td>
                <td className="py-2.5 text-right font-mono text-[12.5px]">
                  {r.toFixed(2)}:1 <span className={GRADE_TONE[g] ?? "text-(--b-accent)"}>{g}</span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function Proportion() {
  return (
    <div className={card}>
      <p className={lab}>Proportion in a typical piece</p>
      <div className="mt-5 flex h-16 overflow-hidden rounded-[10px] border border-line-strong">
        {proportion.map(([n, fill, p]) => (
          <span key={n} title={n} className="block" style={{ width: `${p}%`, background: fill }} />
        ))}
      </div>
      <ul className="m-0 mt-5 grid list-none grid-cols-2 gap-2 p-0 text-[14px]">
        {proportion.map(([n, , p]) => (
          <li key={n} className="flex justify-between border-b border-(--b-line) py-1.5">
            <span>{n}</span>
            <span className="font-mono text-(--b-muted)">{p}%</span>
          </li>
        ))}
      </ul>
      <ul className="m-0 mt-6 list-disc pl-[18px] text-[14.5px] text-(--b-muted) [&_li]:mb-1.5">
        <li>The gradient never fills text, buttons or large flat areas.</li>
        <li>Volt is for actions. If nothing can be clicked, there is no volt.</li>
        <li>Status green, amber and red belong to the product UI, never to marketing.</li>
        <li>The retired orange appears nowhere, not even as a nod.</li>
      </ul>
    </div>
  );
}

export function Color() {
  return (
    <Section
      id="color"
      n="05"
      file="color.css"
      title={
        <>
          Neutral black, <em>light</em> moving through it
        </>
      }
      status="approved"
      intro={
        <p>
          Ink is the ground. The gradient is light, not paint: it lives in the symbol, the art and
          one or two highlights per piece. Signal cyan is the only accent and means “live”.
        </p>
      }
    >
      <Swatches />
      <div className="mt-16 grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <ContrastTable />
        <Proportion />
      </div>
    </Section>
  );
}
