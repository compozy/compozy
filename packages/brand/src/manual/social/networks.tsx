import { cn } from "@compozy/ui/utils";

import { cap, card, h3, lab, rules } from "../cls";
import { Frame } from "../kit-frame";
import { kitSpecById } from "../kit-specs";
import { Section } from "../parts/section";
import { Crop } from "./crop";
import { assets, avatarRules, bannerRules, networks } from "./data";

const th = "px-5 py-3 font-medium";

function AssetTable() {
  return (
    <div className="mt-4 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
      <table className="w-full min-w-[720px] border-collapse text-[14.5px]">
        <thead>
          <tr className="border-b border-(--b-line) text-left mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
            <th className={th}>Network</th>
            <th className={th}>Asset</th>
            <th className={th}>Size</th>
            <th className={th}>Safe area</th>
            <th className={th}>Template</th>
          </tr>
        </thead>
        <tbody>
          {assets.map(([net, a, size, safe, id]) => (
            <tr key={id} className="border-b border-(--b-line) last:border-0">
              <td className="px-5 py-2.5 text-(--b-muted)">{net}</td>
              <td className="px-5 py-2.5">{a}</td>
              <td className="px-5 py-2.5 font-mono text-[13px]">{size}</td>
              <td className="px-5 py-2.5 text-(--b-muted)">{safe}</td>
              <td className="px-5 py-2.5 font-mono text-[12.5px] text-(--b-accent)">{id}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Banners() {
  const banner = kitSpecById("discord-banner");
  const icon = kitSpecById("discord-icon");
  return (
    <>
      <h3 className={cn(h3, "mt-16")}>Banners and covers</h3>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]">
        <div className={card}>
          <ul className={cn(rules, "text-[15px]")}>
            {bannerRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
          <p className={cn(cap, "mb-0")}>
            Cyan dashes: where text may sit. Red hatching: what the network covers or crops.
          </p>
        </div>
        <Frame id="x-header" guides />
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Frame id="linkedin-banner" guides />
        <Frame id="linkedin-cover" guides />
      </div>
      <div className="mt-4 grid items-start gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Frame id="discord-banner" guides />
        <div className={card}>
          <p className={lab}>Discord</p>
          <p className="m-0 mt-3 text-[15px]">
            The banner shows at about 240×135 above the channel list, so the Cone visual carries it
            and the only text is the server name. Server icon = the avatar master.
          </p>
          <p className="m-0 mt-3 text-[15px] text-(--b-muted)">
            Role colours: Team in signal cyan, Contributor in sky, everyone else default. No other
            colours.
          </p>
          <p className="m-0 mt-3 font-mono text-[12px] text-(--b-muted)">
            {banner.w}×{banner.h} · {icon.w}×{icon.h}
          </p>
        </div>
      </div>
    </>
  );
}

export function Networks() {
  return (
    <Section
      id="networks"
      n="01"
      file="networks.md"
      title={
        <>
          One idea, five <em>shapes</em>
        </>
      }
      intro={
        <p>
          Each network has one job. GitHub holds the product, X carries the conversation, Discord
          helps the people already using it, YouTube shows it working and LinkedIn makes the process
          argument to managers. The voice is the same everywhere.
        </p>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {networks.map(n => (
          <div key={n.name} className={cn(card, "flex flex-col p-6!")}>
            <div className="flex items-center justify-between gap-2">
              <p className={lab}>{n.name}</p>
              <span className="mono-label text-[10.5px] tracking-[0.08em] text-(--b-accent)">
                {n.priority}
              </span>
            </div>
            <h3 className={cn(h3, "mt-3 text-[20px]!")}>{n.role}</h3>
            <p className="m-0 flex-1 text-[14.5px] text-(--b-muted)">{n.job}</p>
            <dl className="m-0 mt-4 grid gap-1 border-t border-(--b-line) pt-3 text-[13px]">
              <div>
                <dt className="inline text-(--b-muted)">Handle </dt>
                <dd className="m-0 inline font-mono text-[12px]">{n.handle}</dd>
              </div>
              <div>
                <dt className="inline text-(--b-muted)">Cadence </dt>
                <dd className="m-0 inline">{n.cadence}</dd>
              </div>
            </dl>
          </div>
        ))}
      </div>
      <p className={cap}>
        Hacker News is the highest-priority channel for launches but has no profile assets: a Show
        HN post is plain text and a link to the repo. Handles to confirm before launch.
      </p>

      <h3 className={cn(h3, "mt-16")}>Every profile asset</h3>
      <AssetTable />
      <p className={cap}>
        Sizes checked November 2026. Platforms change them without notice: confirm in the upload
        dialog before replacing a profile asset.
      </p>

      <h3 className={cn(h3, "mt-16")}>Avatars</h3>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <div className={card}>
          <ul className={cn(rules, "text-[15px]")}>
            {avatarRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
          <div className="mt-6 max-w-[280px]">
            <Frame id="x-avatar" guides />
          </div>
          <p className={cap}>
            Dashed box: the inscribed square of the circle crop. The symbol never crosses it.
          </p>
        </div>
        <div className="grid gap-6 sm:grid-cols-2">
          <Crop id="x-avatar" shape="circle" label="X · 400 · circle" />
          <Crop id="youtube-avatar" shape="circle" label="YouTube · 800 · circle" />
          <Crop id="discord-icon" shape="circle" label="Discord · 512 · circle" />
          <Crop id="github-avatar" shape="square" label="GitHub org · 500 · square" />
          <Crop id="linkedin-avatar" shape="rounded" label="LinkedIn · 400 · rounded" />
        </div>
      </div>

      <Banners />
    </Section>
  );
}
