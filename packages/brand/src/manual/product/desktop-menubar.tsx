// Desktop app → menu-bar icon: the three tray template images, the dropdown and their spec.
import { cn } from "@compozy/ui/utils";

import { card, lab, rules } from "../cls";
import { macos } from "./data";

const T = "/brand/product/tray";
const TRAY = ["idle", "working", "attention"] as const;
const trayStates: [file: (typeof TRAY)[number], name: string, note: string][] = [
  ["idle", "Idle", "Running, nothing needs you"],
  ["working", "Working", "At least one session working"],
  ["attention", "Needs you", "A session needs you"],
];
const bars = [
  { bg: macos.lightBar, fg: macos.lightBarText, invert: false },
  { bg: "var(--color-black)", fg: "var(--color-fg-strong)", invert: true },
];

export function DesktopMenubar() {
  return (
    <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <div className={card}>
        <div className="grid gap-3">
          {bars.map(b => (
            <div
              key={b.bg}
              className="flex items-center justify-end gap-5 rounded-[8px] px-4 py-1.5 text-[13px]"
              style={{ background: b.bg, color: b.fg }}
            >
              {TRAY.map(t => (
                <img
                  key={t}
                  src={`${T}/compozy-tray-${t}Template@2x.png`}
                  alt={`${t} menu-bar icon`}
                  className={cn("size-4", b.invert && "invert")}
                />
              ))}
              <span className="opacity-80">Fri 14:02</span>
            </div>
          ))}
        </div>
        <div className="mt-4 ml-auto w-[min(100%,300px)] rounded-[10px] border border-line-strong bg-(--color-elevated) p-1.5 text-[13px] text-(--color-fg) shadow-[0_24px_48px_-24px_rgb(0_0_0/.6)]">
          <p className="m-0 px-2 py-1.5 font-mono text-[11px] text-(--color-muted-foreground)">
            Compozy is running
          </p>
          <p className="m-0 flex items-center gap-2 rounded-[6px] bg-(--color-surface-2) px-2 py-1.5">
            <span className="text-(--color-attn)">◐</span>migrate-db needs you
          </p>
          <p className="m-0 flex items-center gap-2 px-2 py-1.5">
            <span className="text-(--color-brand-cyan)">●</span>2 working
            <span className="ml-auto font-mono text-[11px] text-(--color-muted-foreground)">
              review-pr, fix-flaky-auth
            </span>
          </p>
          <hr className="my-1 border-0 border-t border-line" />
          <p className="m-0 flex px-2 py-1.5">
            Open Compozy
            <span className="ml-auto font-mono text-[11px] text-(--color-muted-foreground)">
              ⌘O
            </span>
          </p>
          <p className="m-0 flex px-2 py-1.5">
            Quit Compozy
            <span className="ml-auto font-mono text-[11px] text-(--color-muted-foreground)">
              ⌘Q
            </span>
          </p>
        </div>
        <div className="mt-6 grid grid-cols-3 gap-3">
          {trayStates.map(([f, n, d]) => (
            <div
              key={f}
              className="band-paper grid justify-items-center gap-2 rounded-[12px] bg-(--color-fg-strong) p-4 text-center"
            >
              <img
                src={`${T}/compozy-tray-${f}Template@2x.png`}
                alt=""
                className="size-12 [image-rendering:pixelated]"
              />
              <span className="text-[13px] font-medium">{n}</span>
              <span className="text-[11.5px] leading-[1.3] text-(--b-muted)">{d}</span>
            </div>
          ))}
        </div>
      </div>
      <div className={card}>
        <p className={lab}>Template image spec</p>
        <ul className={cn(rules, "mt-4 text-[15px]")}>
          <li>
            <strong>16 × 16 pt</strong>, shipped as <code>…Template.png</code> (16 px) and{" "}
            <code>…Template@2x.png</code> (32 px). The <code>Template</code> suffix makes macOS tint
            it for light, dark and highlighted bars.
          </li>
          <li>
            <strong>Black on transparent</strong> only; alpha carries the drawing. No gradient, no
            colour: the menu bar is the system’s, not ours.
          </li>
          <li>
            <strong>One mark, one 1.6 px stroke.</strong> Idle is the base mark; Working adds a
            second arc; Needs you adds a 4.6 px dot. The menu bar has no room for the dot-and-label
            language of the app, so shape carries state.
          </li>
          <li>
            <strong>Three states, not more.</strong> Counts live in the dropdown and the dock badge,
            never as digits in the menu bar.
          </li>
        </ul>
        <ul className="m-0 mt-5 grid list-none gap-1.5 p-0 font-mono text-[12px]">
          {TRAY.flatMap(t => [
            `compozy-tray-${t}Template.svg`,
            `compozy-tray-${t}Template@2x.png`,
          ]).map(f => (
            <li key={f}>
              <a
                className="text-(--b-accent) no-underline hover:underline"
                href={`${T}/${f}`}
                download
              >
                brand/product/tray/{f}
              </a>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
