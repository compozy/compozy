// Desktop app → window chrome (the real desktop, captured), the empty composer specimen and the
// desktop-wallpaper kit template.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { cap, card, lab, rules } from "../cls";
import { Frame } from "../kit-frame";
import { UiCrop } from "../graphics/ui-crop";

export function DesktopWindow() {
  return (
    <>
      <figure className="m-0 mt-4">
        <UiCrop name="desktop" ratio="16/9" className="rounded-[14px] border-line-strong" />
        <figcaption className={cap}>
          Title bar 38 px in ink with native traffic lights (macOS draws them; they are the one
          place system colour appears). Rail ink, list ink-2, the one amber element is the approval.
        </figcaption>
      </figure>

      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <figure className="m-0 min-w-0">
          <div className="relative grid min-h-[440px] place-items-center overflow-hidden rounded-[14px] border border-line-strong bg-(--color-canvas) py-8 text-(--color-fg) sm:aspect-[16/10] sm:min-h-0 sm:py-0">
            <Visual name="field" x={0.95} y={1} className="opacity-25" />
            <div className="relative grid w-[min(80%,520px)] justify-items-center gap-4 text-center">
              <img src="/brand/png/compozy-symbol-128.png" alt="" className="size-14" />
              <p className="m-0 font-display text-[clamp(22px,2.4vw,30px)] leading-[1.15] tracking-[-0.02em]">
                Start a session
              </p>
              <p className="m-0 text-[14px] text-(--color-muted-foreground)">
                Describe the work. Compozy runs it with the agent you pick and tells you when it
                needs you.
              </p>
              <div className="w-full rounded-[14px] border border-line-strong bg-(--color-rail) p-3 text-left text-[13.5px] text-(--color-muted-foreground)">
                Fix the flaky auth tests and stop when they pass
                <span className="ml-0.5 inline-block h-4 w-px translate-y-0.5 bg-(--color-brand-cyan)" />
                <span className="mt-2 flex items-center gap-2 font-mono text-[11.5px]">
                  <span className="rounded-full border border-line-strong px-2 py-0.5">
                    claude-code
                  </span>
                  <span className="rounded-full border border-line-strong px-2 py-0.5">Loop</span>
                  <span className="ml-auto rounded-[8px] bg-(--color-brand) px-2.5 py-1 text-(--color-brand-ink)">
                    Start ↵
                  </span>
                </span>
              </div>
            </div>
          </div>
          <figcaption className={cap}>
            Empty desktop composer. The gradient appears only in the symbol; the Atmosphere field
            sits at 25% opacity in the far corner, the one place art enters a product window, and
            never behind the text.
          </figcaption>
        </figure>
        <div className={card}>
          <p className={lab}>Splash and empty states</p>
          <ul className={cn(rules, "mt-4 text-[15px]")}>
            <li>
              <strong>Splash</strong> is ink with the gradient symbol at 96 px, centred, for no
              longer than the daemon takes to answer. No progress bar, no tagline.
            </li>
            <li>
              <strong>Empty states</strong> say what to do in one line and offer one action. Copy
              names the real noun: session, Loop, automation.
            </li>
            <li>
              <strong>Art in windows</strong> only when nothing else is on screen: Atmosphere{" "}
              <code>field</code> or <code>horizon</code> at ≤ 25% opacity, or none, never behind
              content.
            </li>
            <li>
              <strong>Window radius</strong> 12 px (macOS draws it); panels inside use 14 px,
              controls 10 px.
            </li>
          </ul>
        </div>
      </div>
    </>
  );
}

/** Wallpaper: the desktop-wallpaper kit template with its guides. */
export function DesktopWallpaper() {
  return (
    <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
      <Frame id="desktop-wallpaper" guides />
      <div className={card}>
        <p className={lab}>desktop-wallpaper · 2880 × 1800</p>
        <ul className={cn(rules, "mt-4 text-[15px]")}>
          <li>
            Atmosphere <code>horizon</code> at 60% opacity, band of light across the lower middle,
            top 40% left black, so icons on the right stay readable.
          </li>
          <li>
            The red bands are the menu bar (top) and the macOS dock (bottom): no mark enters them.
          </li>
          <li>
            One-colour symbol and one mono line bottom-left. No wordmark: the macOS dock already
            says Compozy.
          </li>
          <li>
            Also the empty desktop behind the Compozy shell, so the OS and the product share a
            ground.
          </li>
        </ul>
      </div>
    </div>
  );
}
