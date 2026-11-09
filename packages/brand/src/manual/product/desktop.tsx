// 07 Desktop app: the desktop as captured from the real product, app icon, dock, menu bar,
// window chrome, empty composer and the wallpaper template.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { cap, card, h3, lab, specs } from "../cls";
import { UiCrop } from "../graphics/ui-crop";
import { Section } from "../parts/section";
import { macos } from "./data";
import { DesktopMenubar } from "./desktop-menubar";
import { DesktopWallpaper, DesktopWindow } from "./desktop-window";

const P = "/brand/png";
const iconSizes: [file: string, px: number][] = [
  ["1024", 128],
  ["512", 96],
  ["256h", 64],
  ["128h", 48],
  ["64h", 32],
];
/** neutral placeholder apps around Compozy in the dock (mixPct null = the Compozy icon) */
const dock: { app: string; mixPct: number | null }[] = [
  { app: "finder", mixPct: 27 },
  { app: "browser", mixPct: 22 },
  { app: "compozy", mixPct: null },
  { app: "mail", mixPct: 31 },
  { app: "editor", mixPct: 25 },
  { app: "terminal", mixPct: 20 },
];

function Dock() {
  return (
    <div className="grid content-start gap-4">
      <div className="relative aspect-[16/10] overflow-hidden rounded-card border border-(--b-line)">
        <Visual name="horizon" x={0.5} y={0.4} className="opacity-60" />
        <div className="absolute inset-x-[12%] bottom-[6%] flex items-end justify-center gap-[2.2%] rounded-[18px] border border-[color-mix(in_oklab,var(--color-fg-strong)_15%,transparent)] bg-[color-mix(in_oklab,var(--color-fg-strong)_10%,transparent)] px-[3%] py-[1.6%] backdrop-blur-md">
          {dock.map(({ app, mixPct }) =>
            mixPct === null ? (
              <span key={app} className="relative w-[13%]">
                <img
                  src={`${P}/compozy-app-icon-256h.png`}
                  alt="Compozy in the dock"
                  className="block w-full rounded-[22%]"
                />
                <span
                  className="absolute -top-[10%] -right-[10%] grid aspect-square w-[34%] place-items-center rounded-full text-[10px] font-semibold text-(--color-fg-strong)"
                  style={{ background: macos.badge }}
                >
                  2
                </span>
                <span className="absolute -bottom-[16%] left-1/2 size-1 -translate-x-1/2 rounded-full bg-(--color-fg-strong)/80" />
              </span>
            ) : (
              <span
                key={app}
                className="block aspect-square w-[13%] rounded-[22%]"
                style={{
                  background: `color-mix(in oklab, var(--color-fg-strong) ${mixPct}%, var(--color-rail))`,
                }}
              />
            )
          )}
        </div>
      </div>
      <p className={cap}>
        In the dock among neutral placeholders: the only gradient in the row. The badge counts Needs
        you.
      </p>
    </div>
  );
}

export function Desktop() {
  return (
    <Section
      id="desktop"
      n="07"
      file="desktop/build"
      title={
        <>
          The desktop app, the <em>control plane</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          The Electron app is where Compozy is most visible: the macOS dock, the menu bar and a
          window someone keeps open all day. The icon carries the gradient; everything inside the
          window follows the product surfaces.
        </p>
      }
    >
      <figure className="m-0">
        <UiCrop name="desktop" ratio="16/9" />
        <figcaption className={cap}>
          The desktop in the brand: menubar, the app rail, projects with their Always-on faces, and
          the session window with its handoff messages. This is the reference the real app is built
          toward.
        </figcaption>
      </figure>
      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <figure className="m-0 min-w-0">
          <UiCrop name="agents" ratio="4/3" scale={1.75} oy={42} />
          <figcaption className={cap}>
            <b className="font-medium text-(--b-fg)">Always-on list.</b> Face, name, harness, state
            mark and the last check-in note. Needs you sorts first.
          </figcaption>
        </figure>
        <figure className="m-0 min-w-0">
          <UiCrop name="approval" ratio="4/3" scale={2} oy={52} />
          <figcaption className={cap}>
            <b className="font-medium text-(--b-fg)">Approval dock.</b> The agent asks, you allow
            once, always allow or deny. The one amber element in its region.
          </figcaption>
        </figure>
      </div>

      <h3 className={cn(h3, "mt-16")}>App icon in the macOS dock</h3>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>App icon · compozy-app-icon-*</p>
          <div className="mt-6 flex flex-wrap items-end gap-6">
            {iconSizes.map(([f, s]) => (
              <div key={f} className="grid justify-items-center gap-2">
                <img
                  src={`${P}/compozy-app-icon-${f}.png`}
                  alt={`App icon ${f}`}
                  width={s}
                  height={s}
                  className="rounded-[22%]"
                />
                <span className="font-mono text-[11px] text-(--b-muted)">{f}</span>
              </div>
            ))}
          </div>
          <dl className={cn(specs, "mt-6")}>
            <div>
              <dt>Shape</dt>
              <dd>macOS squircle, 824 px body on a 1024 canvas, system shadow (not baked)</dd>
            </div>
            <div>
              <dt>Content</dt>
              <dd>Gradient symbol on ink #0a0a0a, symbol 56% of the body width</dd>
            </div>
            <div>
              <dt>Builds</dt>
              <dd>.icns via iconutil (16–1024 @1×/@2x), .ico 16–256, Linux 512 PNG</dd>
            </div>
            <div>
              <dt>Never</dt>
              <dd>Wordmark in the icon, a light variant, seasonal versions</dd>
            </div>
          </dl>
        </div>
        <Dock />
      </div>

      <h3 className={cn(h3, "mt-16")}>Menu-bar icon</h3>
      <DesktopMenubar />

      <h3 className={cn(h3, "mt-16")}>Window chrome</h3>
      <DesktopWindow />

      <h3 className={cn(h3, "mt-16")}>Wallpaper</h3>
      <DesktopWallpaper />
    </Section>
  );
}
