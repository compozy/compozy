import Image from "next/image";
import { Database, FileCode2, Layers, Network, Plug } from "lucide-react";

const cardBase =
  "group relative isolate min-w-0 overflow-hidden rounded-diagram border border-line bg-rail p-7 transition-colors hover:border-accent/40 sm:p-8 xl:p-10";

const labelBase = "eyebrow font-semibold! mb-5 flex items-center gap-3 text-accent";

const imageBase = "h-full w-full select-none opacity-95";

export function BentoSection() {
  return (
    <section
      id="runtime-map"
      aria-label="AGH runtime map"
      className="scroll-mt-24 border-y border-line bg-rail px-4 py-6 sm:px-5 md:py-10 lg:px-5 lg:py-24"
    >
      <div
        data-testid="bento-grid"
        className="mx-auto grid w-full max-w-300 gap-4 md:grid-cols-2 lg:aspect-1536/1320 lg:grid-cols-6 lg:grid-rows-2"
      >
        <RuntimeCard />
        <NetworkCard />
        <BridgesCard />
        <MemoryCard />
        <ExtensibilityCard />
      </div>
    </section>
  );
}

function RuntimeCard() {
  return (
    <article
      data-testid="bento-runtime"
      className={`${cardBase} min-h-135 md:min-h-140 lg:col-span-3 lg:col-start-1 lg:row-start-1 lg:min-h-0`}
    >
      <div className="absolute inset-x-0 bottom-0 top-[0%] pointer-events-none">
        <Image
          src="/images/bento-illustrations/runtime-v2.png"
          alt="AGH runtime device showing durable agent sessions and status indicators."
          fill
          priority
          decoding="async"
          sizes="(min-width: 1024px) 50vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover`}
        />
      </div>
      <div className="site-bento-overlay-runtime pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-84">
        <div className={labelBase}>
          <Database className="size-4" />
          <span>Runtime</span>
        </div>
        <h3
          aria-label="Your agents. Under control."
          className="font-display text-site-bento-lg font-normal leading-tight text-fg sm:text-site-bento-xl xl:text-site-bento-2xl"
        >
          Your agents.
          <br />
          <span className="text-accent">Under control.</span>
        </h3>
        <span className="mt-5 block h-px w-8 bg-accent" aria-hidden="true" />
      </div>
    </article>
  );
}

function NetworkCard() {
  return (
    <article
      data-testid="bento-network"
      className={`${cardBase} min-h-105 md:col-span-2 md:min-h-125 lg:col-span-3 lg:col-start-4 lg:row-start-1 lg:min-h-0`}
    >
      <div className="absolute inset-0 pointer-events-none">
        <Image
          src="/images/bento-illustrations/network-v2.png"
          alt="AGH network diagram showing discovery, delegation, receipt, and peers."
          fill
          priority
          decoding="async"
          sizes="(min-width: 1024px) 50vw, 100vw"
          quality={90}
          className={`${imageBase} object-contain object-[40%_100%]`}
        />
      </div>
      <div className="site-bento-overlay-network pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-120">
        <div className={labelBase}>
          <Network className="size-4" />
          <span>Network</span>
        </div>
        <h3
          aria-label="Built-in network. Delegate. Deliver. Done."
          className="font-display text-site-bento-md font-normal leading-tight text-fg sm:text-4xl xl:text-site-bento-2xl"
        >
          Built-in network.
          <br />
          <span className="text-accent">Delegate. Deliver.</span> Done.
        </h3>
      </div>
    </article>
  );
}

function BridgesCard() {
  return (
    <article
      data-testid="bento-bridges"
      className={`${cardBase} min-h-90 md:min-h-97.5 lg:col-span-2 lg:col-start-1 lg:row-start-2 lg:min-h-0`}
    >
      <div className="absolute inset-0 pointer-events-none">
        <Image
          src="/images/bento-illustrations/bridges-v2.png"
          alt="Bridge events from Slack, Discord, and Telegram entering an AGH device."
          fill
          decoding="async"
          sizes="(min-width: 1024px) 33vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover object-[10%_20%]`}
        />
      </div>
      <div className="site-bento-overlay-bridges pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-72">
        <div className={labelBase}>
          <Plug className="size-4" />
          <span>Bridges</span>
        </div>
        <h3
          aria-label="From anywhere. Into a session."
          className="font-display text-site-bento-xs font-normal leading-tight text-fg sm:text-site-bento-md xl:text-site-bento-lg"
        >
          From anywhere.
          <br />
          <span className="text-accent">Into a session.</span>
        </h3>
      </div>
    </article>
  );
}

function MemoryCard() {
  return (
    <article
      data-testid="bento-memory"
      className={`${cardBase} min-h-97.5 lg:col-span-2 lg:col-start-3 lg:row-start-2 lg:min-h-0`}
    >
      <div className="absolute inset-x-0 bottom-0 top-[18%] pointer-events-none">
        <Image
          src="/images/bento-illustrations/memory-v2.png"
          alt="Skill document carrying deployment intent into AGH memory."
          fill
          decoding="async"
          sizes="(min-width: 1024px) 33vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover object-[50%_80%]`}
        />
      </div>
      <div className="site-bento-overlay-memory pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-68">
        <div className={labelBase}>
          <FileCode2 className="size-4" />
          <span>Memory</span>
        </div>
        <h3
          aria-label="Memory that compounds."
          className="font-display text-site-bento-sm font-normal leading-tight text-fg sm:text-site-bento-lg xl:text-4xl"
        >
          Memory that
          <br />
          <span className="text-accent">compounds.</span>
        </h3>
      </div>
    </article>
  );
}

function ExtensibilityCard() {
  return (
    <article
      data-testid="bento-extensibility"
      className={`${cardBase} min-h-97.5 lg:col-span-2 lg:col-start-5 lg:row-start-2 lg:min-h-0`}
    >
      <div className="absolute inset-0 -bottom-30 pointer-events-none">
        <Image
          src="/images/bento-illustrations/extensibility-v2.png"
          alt="AGH daemon device with five pluggable extension cartridges — hooks, skills, tools, automation, extensions — snapping into the runtime."
          fill
          decoding="async"
          sizes="(min-width: 1024px) 33vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover object-[10%_10%]`}
        />
      </div>
      <div className="site-bento-overlay-extensibility pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-84">
        <div className={labelBase}>
          <Layers className="size-4" />
          <span>Extensibility</span>
        </div>
        <h3
          aria-label="Every layer. Pluggable."
          className="font-display text-site-bento-sm font-normal leading-tight text-fg sm:text-site-bento-lg xl:text-4xl"
        >
          Every layer.
          <br />
          <span className="text-accent">Pluggable.</span>
        </h3>
      </div>
    </article>
  );
}
